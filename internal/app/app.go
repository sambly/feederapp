package app

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/sambly/feederApp/external/exchangeService/pkg/exchange"
	"github.com/sambly/feederApp/internal/database"
	"github.com/sambly/feederApp/internal/logging"
	"github.com/sambly/feederApp/internal/model"
	"github.com/sambly/feederApp/internal/service"

	"golang.org/x/sync/errgroup"
)

type Application struct {
	mtx      sync.Mutex
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB

	pairs         []string
	periods       []model.Periods
	candles       map[string]map[string]*model.Candle
	candlesBuffer map[string][]model.Candle
	trigerTimer   map[string]bool
}

func NewApp(exch service.Exchange, db *sql.DB, timeframe string, pairs []string, periods []model.Periods) (*Application, error) {

	app := &Application{
		mtx:      sync.Mutex{},
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, timeframe),
		database: db,

		pairs:         pairs,
		periods:       periods,
		candles:       make(map[string]map[string]*model.Candle),
		candlesBuffer: make(map[string][]model.Candle),
		trigerTimer:   make(map[string]bool),
	}
	return app, nil
}

func (app *Application) Run(ctx context.Context) error {
	timeStart := time.Now().Truncate(time.Minute)
	timeStart = timeStart.Add(time.Minute)

	for _, pair := range app.pairs {
		app.dataFeed.SubscribeTrade(ctx, pair, func(trade model.Trade) {
			app.onTrade(ctx, trade)
		})

		if _, ok := app.candles[pair]; !ok {
			app.candles[pair] = map[string]*model.Candle{}
		}
		for _, period := range app.periods {
			nextTime := findNextMultipleTime(timeStart, period.Duration)
			app.candles[pair][period.Name] = &model.Candle{Pair: pair, Time: nextTime}
		}
	}

	for _, period := range app.periods {
		app.candlesBuffer[period.Name] = []model.Candle{}
		app.trigerTimer[period.Name] = false
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.dataFeed.Start(ctx)
	})

	return g.Wait()

}

func (app *Application) onTimer(ctx context.Context, period model.Periods) {

	app.trigerTimer[period.Name] = true

	go func(ctx context.Context, period model.Periods) {
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

func (app *Application) onTrade(ctx context.Context, trade model.Trade) {

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

func (app *Application) WriteCandleBuffer(candle model.Candle, period model.Periods) {
	candle.Time = candle.Time.Add(-1 * period.Duration)
	app.candlesBuffer[period.Name] = append(app.candlesBuffer[period.Name], candle)
}

func (app *Application) WriteTrade(candle *model.Candle, period model.Periods) {

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

func (app *Application) UpdateCandlesTriger(ctx context.Context, period model.Periods) {
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

func (app *Application) WriteTradeDatabase(ctx context.Context, period model.Periods) {

	candles := app.candlesBuffer[period.Name]
	go func(ctx context.Context) {
		start := time.Now()
		select {
		case <-ctx.Done():
			return
		default:
			err := database.InsertCandlesTableNameV3(app.database, period.Name, candles)
			if err != nil {
				logging.MyLogger.ErrorOut(fmt.Errorf("error app.WriteTradeDatabase: %v", err))
			}
			duration := time.Since(start)
			logging.MyLogger.InfoLog.Printf("t:%v  period %s ", duration, period.Name)
		}
	}(ctx)
	app.candlesBuffer[period.Name] = []model.Candle{}
}

func (app *Application) SetTrigerTimer(period model.Periods, value bool) {
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
