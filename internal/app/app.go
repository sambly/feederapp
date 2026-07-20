package app

import (
	"context"
	"sync"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/feederapp/internal/database"
	"github.com/sambly/feederapp/internal/logger"
	iModel "github.com/sambly/feederapp/internal/model"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var appLogger = logger.AddFields(map[string]interface{}{
	"package": "app",
})

type Application struct {
	mtx      sync.Mutex
	dataFeed *exchange.DataFeed
	database *gorm.DB

	pairs         []string
	periods       []iModel.Periods
	candles       map[string]map[string]*exModel.Candle
	candlesBuffer map[string][]exModel.Candle
	trigerTimer   map[string]bool

	// healOnClose[pair][period] — первый после старта бакет пары/периода помечен
	// на перепроверку по закрытию (и для сидированных из истории, и для
	// cold-started — см. Run). Между назначением candle.Time и фактическим стартом
	// live-подписки проходит время (сидинг сотен пар через общий весовой лимитер
	// занимает минуты), трейды этого окна не видит ни seed/cold-start, ни live —
	// свеча закрылась бы с недобором (для cold-start — вообще нулями, см. catch-up
	// цикл в onTrade). Поэтому при первом закрытии такого бакета перезапрашиваем
	// его финальный kline с биржи и апсертим поверх live-агрегата (authoritative-
	// значения; amount_trade_buy защищён GREATEST в candleConflictClause).
	// Флаг разовый: снимается сразу при первом использовании в WriteTrade.
	// Защищено app.mtx.
	healOnClose map[string]map[string]bool
}

func NewApp(dataFeed *exchange.DataFeed, db *gorm.DB, pairs []string, periods []iModel.Periods) (*Application, error) {

	app := &Application{
		mtx:      sync.Mutex{},
		dataFeed: dataFeed,
		database: db,

		pairs:         pairs,
		periods:       periods,
		candles:       make(map[string]map[string]*exModel.Candle),
		candlesBuffer: make(map[string][]exModel.Candle),
		trigerTimer:   make(map[string]bool),
		healOnClose:   make(map[string]map[string]bool),
	}
	return app, nil
}

func (app *Application) Run(ctx context.Context) error {

	for _, pair := range app.pairs {
		if _, ok := app.candles[pair]; !ok {
			app.candles[pair] = map[string]*exModel.Candle{}
		}
	}

	// Восстанавливаем текущий (ещё не закрытый) бакет каждой пары/периода из истории
	// биржи, прежде чем подписываться на live-трейды — иначе рестарт посреди накопления
	// крупной свечи (например, 4h) обнулял бы её в памяти. Идёт параллельно по всем
	// парам/периодам; реальная конкурентность ограничена общим весовым лимитером на
	// стороне exchangeService, поэтому отдельный пул воркеров тут не нужен.
	// Периоды короче seedMinPeriod не сидируются: полный сидинг всех пар через весовой
	// лимитер занимает минуты, и для мелких периодов seed протухает быстрее, чем
	// успевает примениться — потеря ограничена одним бакетом. Их coldStartCandle
	// назначается отдельным циклом ниже, после seedGroup.Wait() (см. комментарий там).
	seedGroup, seedCtx := errgroup.WithContext(ctx)
	for _, pair := range app.pairs {
		for _, period := range app.periods {
			if period.Duration < seedMinPeriod {
				// coldStartCandle назначается ПОСЛЕ этого цикла, см. ниже —
				// иначе её Time отсчитывался бы от начала Run(), а не от
				// момента, когда реально стартует live-подписка.
				continue
			}
			pair, period := pair, period
			seedGroup.Go(func() error {
				seeded := app.seedCandle(seedCtx, pair, period)
				app.mtx.Lock()
				app.candles[pair][period.Name] = seeded
				// StartT=true только у реально сидированной свечи (не cold-start):
				// именно её при закрытии надо перечитать с биржи (см. healOnClose).
				if seeded.StartT {
					app.markHealOnClose(pair, period.Name)
				}
				app.mtx.Unlock()
				return nil
			})
		}
	}
	if err := seedGroup.Wait(); err != nil {
		return err
	}

	// Мелкие периоды (1m/3m) не сидируются историей (см. seedMinPeriod), поэтому их
	// coldStartCandle обязан отсчитываться от времени, БЛИЗКОГО к фактическому старту
	// live-подписки (следующий цикл), а не от начала Run(): сидинг ~460 пар × 4
	// периода через общий весовой лимитер exchangeService занимает ~10-15 минут (см.
	// seedGroup.Wait() выше). Если бы candle.Time ставилось до сидинга, первый же
	// live-трейд обнаруживал бы "устаревший" на ~13 минут бакет, onTrade прогонял бы
	// catch-up цикл (см. ниже) и записал бы в БД пачку ПУСТЫХ (все поля 0) минутных
	// свечей за всё время сидирования — именно так и происходило: наблюдалось как
	// "candles_1m забита нулями" сразу после restart. healOnClose помечается и для
	// этих периодов — страховка на случай остаточной небольшой задержки между
	// подпиской и первым трейдом (несколько секунд на открытие 460 gRPC-стримов).
	for _, pair := range app.pairs {
		for _, period := range app.periods {
			if period.Duration >= seedMinPeriod {
				continue
			}
			app.mtx.Lock()
			app.candles[pair][period.Name] = coldStartCandle(pair, period)
			app.markHealOnClose(pair, period.Name)
			app.mtx.Unlock()
		}
	}

	for _, pair := range app.pairs {
		app.dataFeed.SubscribeTrade(pair)
		app.dataFeed.SubscribeObserverTrade(ctx, "FeederApp", pair, func(trade exModel.Trade) {
			app.onTrade(ctx, trade)
		})
	}

	for _, period := range app.periods {
		app.candlesBuffer[period.Name] = []exModel.Candle{}
		app.trigerTimer[period.Name] = false
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.dataFeed.StartTradeFeeder(gCtx, "FeederApp")
	})

	return g.Wait()

}

func (app *Application) onTimer(ctx context.Context, period iModel.Periods) {

	app.trigerTimer[period.Name] = true

	go func(ctx context.Context, period iModel.Periods) {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			app.SetTrigerTimer(period, false)
			app.UpdateCandlesTriger(ctx, period)
		case <-ctx.Done():
			return
		}
	}(ctx, period)
}

func (app *Application) onTrade(ctx context.Context, trade exModel.Trade) {

	select {
	case <-ctx.Done():
		return
	default:
	}

	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, period := range app.periods {

		candle := app.candles[trade.Pair][period.Name]

		difTime := trade.Time.Sub(candle.Time)

		// Цикл (а не if): свеча могла отстать более чем на один период — например,
		// после реконнекта без трейдов или из-за протухшего seed — и без догона
		// трейд записался бы в бакет с чужой временной меткой.
		for difTime >= time.Duration(period.Duration) {
			// Запускаем таймер для полной записи всех пар
			if !app.trigerTimer[period.Name] {
				app.onTimer(ctx, period)
			}
			app.WriteTrade(ctx, candle, period)
			difTime = trade.Time.Sub(candle.Time)

		}

		if difTime >= 0 {

			if !candle.StartT {
				candle.StartT = true
				candle.Open = trade.Price
				candle.Low = trade.Price
				candle.High = 0 //  todo candle.High = trade.Price
			}

			candle.Price = trade.Price
			if trade.Price > candle.High {
				candle.High = trade.Price
			}

			if trade.Price < candle.Low {
				candle.Low = trade.Price
			}
			candle.Close = trade.Price
			candle.Volume += trade.Quantity
			candle.QuoteVolume += trade.Quantity * trade.Price

			candle.AmountTrade++
			if !trade.IsBuyerMaker {
				candle.AmountTradeBuy++
				candle.ActiveBuyVolume += trade.Quantity
				candle.ActiveBuyQuoteVolume += trade.Price * trade.Quantity
			}
		}

	}
}

// markHealOnClose помечает бакет пары/периода как требующий heal при закрытии
// (см. healOnClose в Application и WriteTrade). Вызывающий обязан держать app.mtx.
func (app *Application) markHealOnClose(pair, periodName string) {
	if app.healOnClose[pair] == nil {
		app.healOnClose[pair] = make(map[string]bool)
	}
	app.healOnClose[pair][periodName] = true
}

func (app *Application) WriteCandleBuffer(candle exModel.Candle, period iModel.Periods) {
	candle.Time = candle.Time.Add(-1 * period.Duration)
	app.candlesBuffer[period.Name] = append(app.candlesBuffer[period.Name], candle)
}

func (app *Application) WriteTrade(ctx context.Context, candle *exModel.Candle, period iModel.Periods) {

	closedStart := candle.Time // начало закрывающегося бакета — для heal-on-close

	candle.Time = candle.Time.Add(period.Duration)
	candle.CompleteTrade = true

	candle.StartT = false

	app.WriteCandleBuffer(*candle, period)

	// Сидированный бакет закрылся: live-агрегат неполон (окно между seed и началом
	// подписки), перечитываем финальный kline с биржи в фоне. Флаг снимается сразу —
	// heal нужен ровно один раз, для первого закрытия после старта.
	if periods := app.healOnClose[candle.Pair]; periods != nil && periods[period.Name] {
		delete(periods, period.Name)
		go app.healClosedBucket(ctx, candle.Pair, period, closedStart)
	}

	// Reset candle fields
	//*candle = model.Candle{Pair: candle.Pair, Time: candle.Time} // todo

	candle.Price = 0
	candle.Low = 0
	candle.High = 0
	candle.Open = 0
	candle.Close = 0
	candle.Volume = 0
	candle.QuoteVolume = 0
	candle.AmountTrade = 0
	candle.AmountTradeBuy = 0
	candle.ActiveBuyVolume = 0
	candle.ActiveBuyQuoteVolume = 0

}

func (app *Application) UpdateCandlesTriger(ctx context.Context, period iModel.Periods) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, pair := range app.pairs {
		candle := app.candles[pair][period.Name]
		if !candle.CompleteTrade {
			app.WriteTrade(ctx, candle, period)

		}
		candle.CompleteTrade = false
	}

	// Запись в базу данных
	app.WriteTradeDatabase(ctx, period)

}

func (app *Application) WriteTradeDatabase(ctx context.Context, period iModel.Periods) {

	candles := app.candlesBuffer[period.Name]
	go func(ctx context.Context) {
		start := time.Now()
		select {
		case <-ctx.Done():
			return
		default:
			err := database.InsertCandles(app.database, candles, period.Name)
			if err != nil {
				appLogger.Errorf("error app.WriteTradeDatabase: %v", err)
			} else if period.Name == "1m" {
				if err := database.UpdateHeartbeat(app.database, time.Now()); err != nil {
					appLogger.Errorf("error app.WriteTradeDatabase: update heartbeat: %v", err)
				}
			}
			duration := time.Since(start)
			appLogger.Debugf("t:%v  period %s ", duration, period.Name)
		}
	}(ctx)
	app.candlesBuffer[period.Name] = []exModel.Candle{}
}

func (app *Application) SetTrigerTimer(period iModel.Periods, value bool) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.trigerTimer[period.Name] = value
}

// seedMinPeriod — периоды короче этого не сидируются из истории: полный сидинг
// всех пар через весовой лимитер занимает минуты, и для мелких периодов seed
// протухает быстрее, чем успевает примениться. Потеря ограничена одним бакетом.
const seedMinPeriod = 15 * time.Minute

// coldStartCandle — прежнее поведение холодного старта: пустая свеча с началом на
// следующей полной границе периода (накопление начнётся только с этой границы).
func coldStartCandle(pair string, period iModel.Periods) *exModel.Candle {
	timeStart := time.Now().Truncate(time.Minute).Add(time.Minute)
	nextTime := findNextMultipleTime(timeStart, period.Duration)
	return &exModel.Candle{Pair: pair, Time: nextTime}
}

// seedCandle восстанавливает текущий, ещё не закрытый бакет свечи для пары/периода из
// истории биржи (через exchangeService/dataFeed), чтобы рестарт не обнулял то, что уже
// было накоплено с начала бакета — критично для крупных периодов (1h/4h/12h), где иначе
// терялись бы часы объёма и трейдов. Время берётся в момент вызова (не общий now на все
// пары): сидинг сотен пар через весовой лимитер занимает минуты, общий now протухал бы.
// Если данных нет (например, самый первый бакет пары с начала торгов, или сеть
// недоступна) — откатывается к прежнему поведению холодного старта.
func (app *Application) seedCandle(ctx context.Context, pair string, period iModel.Periods) *exModel.Candle {
	now := time.Now()
	bucketStart := now.Truncate(period.Duration)

	candlesChan, errChan := app.dataFeed.HistoricalCandles(ctx, pair, period.Name, bucketStart, now)

	var seed *exModel.Candle
loop:
	for {
		select {
		case candle, ok := <-candlesChan:
			if !ok {
				break loop
			}
			c := candle
			seed = &c
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if err != nil {
				appLogger.Errorf("seed candle: pair %s period %s: %v", pair, period.Name, err)
			}
		case <-ctx.Done():
			break loop
		}
	}

	// Дочитываем ошибку, которую select мог не успеть прочитать до закрытия candlesChan.
	if errChan != nil {
		if err, ok := <-errChan; ok && err != nil {
			appLogger.Errorf("seed candle: pair %s period %s: %v", pair, period.Name, err)
		}
	}

	if seed == nil {
		return coldStartCandle(pair, period)
	}

	seed.Pair = pair
	seed.Time = bucketStart
	seed.StartT = true
	return seed
}

// findNextMultipleTime возвращает ближайшую границу интервала >= t. Бакет,
// начинающийся на этой границе, будет накоплен с самого начала (t — уже будущая
// граница минуты относительно старта), поэтому дополнительный интервал ожидания
// не нужен: раньше он безусловно добавлялся и терял один полный закрытый бакет
// каждого мелкого периода на каждом рестарте (для 1m — до 2-3 минутных свечей).
// healClosedBucket перечитывает с биржи финальный kline закрывшегося сидированного
// бакета и апсертит его поверх live-агрегата. Live-агрегат для такого бакета неполон:
// сидинг сотен пар через весовой лимитер занимает минуты, и трейды между seed-снапшотом
// пары и фактическим стартом подписки не учтены нигде. Kline биржи — authoritative
// для всех полей, кроме amount_trade_buy (в klines его нет, у heal-строки он 0) —
// live-значение этого поля защищено GREATEST в candleConflictClause.
func (app *Application) healClosedBucket(ctx context.Context, pair string, period iModel.Periods, bucketStart time.Time) {
	// Даём бирже финализировать kline только что закрывшегося бакета.
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return
	}

	end := bucketStart.Add(period.Duration)
	candlesChan, errChan := app.dataFeed.HistoricalCandles(ctx, pair, period.Name, bucketStart, end)

	// Binance включает в выборку и kline с openTime == end (следующий, открытый) —
	// берём строго свечу нашего бакета.
	var final *exModel.Candle
loop:
	for {
		select {
		case candle, ok := <-candlesChan:
			if !ok {
				break loop
			}
			if candle.Time.Equal(bucketStart) {
				c := candle
				final = &c
			}
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if err != nil {
				appLogger.Errorf("heal-on-close: pair %s period %s: %v", pair, period.Name, err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
	if errChan != nil {
		if err, ok := <-errChan; ok && err != nil {
			appLogger.Errorf("heal-on-close: pair %s period %s: %v", pair, period.Name, err)
			return
		}
	}

	if final == nil {
		appLogger.Warnf("heal-on-close: pair %s period %s: no kline for bucket %s", pair, period.Name, bucketStart)
		return
	}

	final.Pair = pair
	final.Time = bucketStart
	if err := database.InsertCandle(app.database, *final, period.Name); err != nil {
		appLogger.Errorf("heal-on-close: pair %s period %s insert: %v", pair, period.Name, err)
		return
	}
	appLogger.Debugf("heal-on-close: pair %s period %s bucket %s healed (trades=%d)", pair, period.Name, bucketStart, final.AmountTrade)
}

func findNextMultipleTime(t time.Time, interval time.Duration) time.Time {
	remainder := t.Unix() % int64(interval.Seconds())
	if remainder != 0 {
		seconds := int64(interval.Seconds())
		// Поднимаем к следующей границе, кратной интервалу
		t = t.Add(time.Duration(seconds-remainder) * time.Second)
	}
	return t
}
