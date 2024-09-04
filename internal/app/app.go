package app

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/sambly/exchangeService/pkg/exchange"
	exModel "github.com/sambly/exchangeService/pkg/model"
	"github.com/sambly/feederapp/internal/database"
	"github.com/sambly/feederapp/internal/logger"
	iModel "github.com/sambly/feederapp/internal/model"
	"golang.org/x/sync/errgroup"
)

var appLogger = logger.AddFields(map[string]interface{}{
	"package": "app",
})

type Application struct {
	mtx      sync.Mutex
	dataFeed exchange.RouterDataFeed
	database *sql.DB

	pairs         []string
	periods       []iModel.Periods
	candles       map[string]map[string]*exModel.Candle
	candlesBuffer map[string][]exModel.Candle
	trigerTimer   map[string]bool
}

func NewApp(dataFeed exchange.RouterDataFeed, db *sql.DB, pairs []string, periods []iModel.Periods) (*Application, error) {

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
	timeStart := time.Now().Truncate(time.Minute)
	timeStart = timeStart.Add(time.Minute)

	for _, pair := range app.pairs {

		if _, ok := app.candles[pair]; !ok {
			app.candles[pair] = map[string]*exModel.Candle{}
		}
		for _, period := range app.periods {
			nextTime := findNextMultipleTime(timeStart, period.Duration)
			app.candles[pair][period.Name] = &exModel.Candle{Pair: pair, Time: nextTime}
		}
		app.dataFeed.SubscribeTrade(ctx, pair, "FeederApp")
		err := app.dataFeed.SubscribeObserverTrade(ctx, "FeederApp", pair, func(trade exModel.Trade) {
			app.onTrade(ctx, trade)
		})

		if err != nil {
			appLogger.Errorf("error SubscribeObserverTrade: %v", err)
		}
	}

	for _, period := range app.periods {
		app.candlesBuffer[period.Name] = []exModel.Candle{}
		app.trigerTimer[period.Name] = false
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.dataFeed.StartTradeFeeder(gCtx)
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

		if difTime >= time.Duration(period.Duration) {
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
			err := database.InsertCandlesTableName(app.database, period.Name, candles)
			if err != nil {
				appLogger.Errorf("error app.WriteTradeDatabase: %v", err)
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
