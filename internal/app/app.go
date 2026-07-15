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
	// успевает примениться — потеря ограничена одним бакетом.
	seedGroup, seedCtx := errgroup.WithContext(ctx)
	for _, pair := range app.pairs {
		for _, period := range app.periods {
			if period.Duration < seedMinPeriod {
				app.mtx.Lock()
				app.candles[pair][period.Name] = coldStartCandle(pair, period)
				app.mtx.Unlock()
				continue
			}
			pair, period := pair, period
			seedGroup.Go(func() error {
				seeded := app.seedCandle(seedCtx, pair, period)
				app.mtx.Lock()
				app.candles[pair][period.Name] = seeded
				app.mtx.Unlock()
				return nil
			})
		}
	}
	if err := seedGroup.Wait(); err != nil {
		return err
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
			app.WriteTrade(candle, period)
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

func (app *Application) WriteCandleBuffer(candle exModel.Candle, period iModel.Periods) {
	candle.Time = candle.Time.Add(-1 * period.Duration)
	app.candlesBuffer[period.Name] = append(app.candlesBuffer[period.Name], candle)
}

func (app *Application) WriteTrade(candle *exModel.Candle, period iModel.Periods) {

	candle.Time = candle.Time.Add(period.Duration)
	candle.CompleteTrade = true

	candle.StartT = false

	app.WriteCandleBuffer(*candle, period)

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
			app.WriteTrade(candle, period)

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
			appLogger.Infof("t:%v  period %s ", duration, period.Name)
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

func findNextMultipleTime(t time.Time, interval time.Duration) time.Time {
	// Находим ближайшее время, которое кратно интервалу, начиная с t
	remainder := t.Unix() % int64(interval.Seconds())
	if remainder != 0 {
		seconds := int64(interval.Seconds())
		// Добавляем оставшееся время до следующего кратного интервала
		t = t.Add(time.Duration(seconds-remainder) * time.Second)
		// Добавляем этот же период времени, так как нужно дождаться чтобы все данные успели сформироваться
	}
	return t.Add(interval)
}
