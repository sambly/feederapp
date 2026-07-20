package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/feederapp/internal/database"
	iModel "github.com/sambly/feederapp/internal/model"
	"golang.org/x/sync/errgroup"
)

const candlePeriod1m = "1m"

// backfillRetryDelays — общие задержки между попытками: и для докачки 1m с биржи
// (ретрай пары целиком в RunBackfill), и для вставки уже посчитанных агрегатов
// (локальный ретрай в insertAggregatedWithRetry) — оба идемпотентны благодаря
// upsert по (pair, time).
var backfillRetryDelays = []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}

// RunBackfill докачивает пропущенные (уже полностью закрытые) свечи по всем парам за
// время, прошедшее с последнего heartbeat, и агрегирует из них остальные таймфреймы.
// Запускается на каждом старте безусловно — сама докачка дешёвая (диапазон обычно мал),
// а threshold используется только для того, чтобы отличить в логах штатный рестарт от
// настоящего простоя. Свечу, которая на момент старта ещё не закрылась (текущий открытый
// бакет любого периода), сюда не пишем — её достраивает seedCandle в app.go, а закрывает
// обычный live-путь. Не блокирует запуск live-потока трейдов: предполагается, что
// вызывается параллельно с Run в отдельной горутине.
func (app *Application) RunBackfill(ctx context.Context, threshold time.Duration, workers int) error {
	last, ok, err := database.GetLastHeartbeat(app.database)
	if err != nil {
		appLogger.Errorf("backfill: failed to read last heartbeat: %v", err)
		return nil
	}
	if !ok {
		appLogger.Infof("backfill: no previous heartbeat found, skipping (first run)")
		return nil
	}

	runStart := time.Now()
	now := runStart
	gap := now.Sub(last)
	if gap <= threshold {
		appLogger.Infof("backfill: gap since last heartbeat is %s (routine restart), backfilling any closed candles for %d pairs", gap, len(app.pairs))
	} else {
		appLogger.Infof("backfill: detected downtime of %s (last heartbeat at %s), backfilling closed candles for %d pairs", gap, last, len(app.pairs))
	}

	// Начало диапазона выравниваем вниз на границу самого крупного периода: иначе
	// (а) минутный бакет, содержащий heartbeat, выпадает из выборки биржи (Binance
	// отдаёт openTime >= startTime), и (б) первый крупный бакет, закрывшийся во время
	// простоя, агрегируется из неполного набора 1m-свечей. Пересечение с уже
	// записанными строками безвредно благодаря upsert.
	maxDur := time.Duration(0)
	for _, p := range app.periods {
		if p.Duration > maxDur {
			maxDur = p.Duration
		}
	}
	fetchStart := last.Truncate(maxDur)

	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)

	var (
		failedPairs     atomic.Int64
		aggregateFailed atomic.Int64
		totalInserted   atomic.Int64
	)

	g, gCtx := errgroup.WithContext(ctx)
	for _, pair := range app.pairs {
		pair := pair
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
			case <-gCtx.Done():
				return gCtx.Err()
			}
			defer func() { <-sem }()

			// Повторные попытки идемпотентны благодаря upsert по (pair, time).
			var (
				inserted int
				err      error
			)
			for attempt := 0; attempt <= len(backfillRetryDelays); attempt++ {
				inserted, err = app.backfillPair(gCtx, pair, fetchStart, now, &aggregateFailed)
				if err == nil || gCtx.Err() != nil {
					break
				}
				appLogger.Warnf("backfill: pair %s attempt %d failed: %v", pair, attempt+1, err)
				if attempt < len(backfillRetryDelays) {
					select {
					case <-time.After(backfillRetryDelays[attempt]):
					case <-gCtx.Done():
					}
				}
			}
			if err != nil {
				// Ошибка по одной паре не должна останавливать бэкофилл остальных.
				appLogger.Errorf("backfill: pair %s failed after %d attempts: %v", pair, len(backfillRetryDelays)+1, err)
				failedPairs.Add(1)
				return nil
			}
			totalInserted.Add(int64(inserted))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if n := failedPairs.Load(); n > 0 {
		appLogger.Errorf("backfill: %d/%d pairs failed; their ranges will NOT be retried automatically", n, len(app.pairs))
	}
	if n := aggregateFailed.Load(); n > 0 {
		appLogger.Errorf("backfill: %d pair/period aggregate inserts failed after retries (1m data is intact, aggregated candle for that bucket is missing)", n)
	}

	// Сдвигаем heartbeat вперёд независимо от того, все ли пары докачались успешно:
	// иначе одна упорно падающая пара будет запускать полный бэкофилл на каждом рестарте.
	if err := database.UpdateHeartbeat(app.database, now); err != nil {
		appLogger.Errorf("backfill: failed to update heartbeat after backfill: %v", err)
	}

	appLogger.Infof("backfill: done, %d 1m candles inserted across %d/%d pairs in %s",
		totalInserted.Load(), len(app.pairs)-int(failedPairs.Load()), len(app.pairs), time.Since(runStart))
	return nil
}

// backfillPair докачивает и вставляет закрытые 1m-свечи пары, затем агрегирует их в
// остальные периоды. Возвращает число вставленных 1m-свечей (0 при ошибке или пустом
// диапазоне) — используется RunBackfill только для итоговой сводки в логе.
func (app *Application) backfillPair(ctx context.Context, pair string, start, end time.Time, aggregateFailed *atomic.Int64) (int, error) {
	candlesChan, errChan := app.dataFeed.HistoricalCandles(ctx, pair, candlePeriod1m, start, end)

	candles1m := make([]exModel.Candle, 0)
loop:
	for {
		select {
		case candle, ok := <-candlesChan:
			if !ok {
				break loop
			}
			candles1m = append(candles1m, candle)
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if err != nil {
				return 0, fmt.Errorf("fetch historical candles: %w", err)
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	// candlesChan мог закрыться раньше, чем select прочитал ошибку из errChan
	// (продюсер кладёт ошибку и закрывает оба канала) — дочитываем, иначе частичные
	// данные будут молча засчитаны как полный диапазон. Блокирующее чтение безопасно:
	// продюсер всегда закрывает errChan.
	if errChan != nil {
		if err, ok := <-errChan; ok && err != nil {
			return 0, fmt.Errorf("fetch historical candles: %w", err)
		}
	}

	// Текущую, ещё не закрытую 1m-свечу сюда не пишем — её досчитает и запишет
	// обычный live-путь (или seedCandle её уже подхватил в память при старте).
	closed := closedCandles(candles1m, time.Minute, end)

	if len(closed) == 0 {
		appLogger.Debugf("backfill: pair %s has no closed historical candles in range", pair)
		return 0, nil
	}

	if err := database.InsertCandles(app.database, closed, candlePeriod1m); err != nil {
		return 0, fmt.Errorf("insert 1m candles: %w", err)
	}

	for _, period := range app.periods {
		if period.Name == candlePeriod1m {
			continue
		}
		aggregated := closedCandles(aggregateCandles(closed, period), period.Duration, end)
		if len(aggregated) == 0 {
			continue
		}
		if err := app.insertAggregatedWithRetry(ctx, aggregated, period.Name); err != nil {
			appLogger.Errorf("backfill: pair %s aggregate %s failed after retries: %v", pair, period.Name, err)
			aggregateFailed.Add(1)
		}
	}

	appLogger.Debugf("backfill: pair %s done, %d 1m candles inserted", pair, len(closed))
	return len(closed), nil
}

// insertAggregatedWithRetry ретраит вставку уже посчитанных агрегированных свечей.
// Данные уже в памяти (пересчёт с биржи не нужен — в отличие от ретрая всей пары
// в RunBackfill), поэтому ретраится только сам insert: этого достаточно против
// временных проблем вроде дедлоков MySQL при параллельной вставке разных пар.
func (app *Application) insertAggregatedWithRetry(ctx context.Context, candles []exModel.Candle, periodName string) error {
	var err error
	for attempt := 0; attempt <= len(backfillRetryDelays); attempt++ {
		err = database.InsertCandles(app.database, candles, periodName)
		if err == nil {
			return nil
		}
		if attempt < len(backfillRetryDelays) {
			select {
			case <-time.After(backfillRetryDelays[attempt]):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return err
}

// closedCandles отбрасывает свечи, чей бакет [Time, Time+duration) ещё не закрылся
// относительно now — чтобы бэкофилл никогда не писал текущий открытый бакет (это зона
// ответственности seedCandle + live-пути) и не создавал по нему конфликт при upsert.
func closedCandles(candles []exModel.Candle, duration time.Duration, now time.Time) []exModel.Candle {
	result := make([]exModel.Candle, 0, len(candles))
	for _, c := range candles {
		if !c.Time.Add(duration).After(now) {
			result = append(result, c)
		}
	}
	return result
}

// aggregateCandles группирует уже отсортированные по времени 1m-свечи в свечи периода
// period.Duration. Границы бакетов совпадают с тем, как биржа сама формирует крупные
// интервалы (усечение unix-времени до кратного period.Duration).
func aggregateCandles(candles1m []exModel.Candle, period iModel.Periods) []exModel.Candle {
	if len(candles1m) == 0 {
		return nil
	}

	order := make([]time.Time, 0)
	buckets := make(map[time.Time]*exModel.Candle)

	for _, c := range candles1m {
		bucketTime := c.Time.Truncate(period.Duration)

		agg, ok := buckets[bucketTime]
		if !ok {
			first := c
			first.ID = 0
			first.Time = bucketTime
			buckets[bucketTime] = &first
			order = append(order, bucketTime)
			continue
		}

		if c.High > agg.High {
			agg.High = c.High
		}
		if c.Low < agg.Low {
			agg.Low = c.Low
		}
		agg.Close = c.Close
		agg.Volume += c.Volume
		agg.QuoteVolume += c.QuoteVolume
		agg.AmountTrade += c.AmountTrade
		agg.AmountTradeBuy += c.AmountTradeBuy
		agg.ActiveBuyVolume += c.ActiveBuyVolume
		agg.ActiveBuyQuoteVolume += c.ActiveBuyQuoteVolume
	}

	result := make([]exModel.Candle, 0, len(order))
	for _, t := range order {
		result = append(result, *buckets[t])
	}
	return result
}
