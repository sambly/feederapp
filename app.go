package main

import (
	"database/sql"
	"fmt"
	"main/database"
	"main/exchange"
	"main/log"
	"main/model"
	"main/service"
	"sync"
	"time"
)

type Application struct {
	mtx         sync.Mutex
	settings    model.Settings
	exchange    service.Exchange
	dataFeed    *exchange.DataFeedSubscription
	database    *sql.DB
	trigerTrade bool
}

func NewApp(exch service.Exchange, settings model.Settings, db *sql.DB) (*Application, error) {

	app := &Application{
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Timeframe, settings.Pairs),
		database: db,
	}
	return app, nil
}

func (app *Application) Run() error {

	for _, pair := range app.settings.Pairs {
		app.dataFeed.SubscribeTrade(pair, app.onTrade)
	}

	go func() {
		for {
			if app.trigerTrade {
				timer := time.NewTimer(5 * time.Second)
				<-timer.C
				app.trigerTrade = false
				app.UpdateCandlesTriger()
			}
		}
	}()

	app.dataFeed.Start(true)

	return nil
}

func (app *Application) onTrade(trade model.Trade) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	candles := app.dataFeed.Candles[trade.Pair]
	difTime := trade.Time.Sub(candles.Time)

	if difTime >= time.Duration(time.Minute.Nanoseconds()) {

		app.trigerTrade = true
		app.WriteTrade(candles)
		difTime = trade.Time.Sub(candles.Time)
	}

	if difTime >= 0 {

		if !candles.StartT {
			candles.StartT = true
			candles.Open = trade.Price
			candles.Low = trade.Price
			candles.High = 0
		}

		candles.Price = trade.Price
		if trade.Price > candles.High {
			candles.High = trade.Price
		}

		if trade.Price < candles.Low {
			candles.Low = trade.Price
		}
		candles.Close = trade.Price
		candles.Volume += trade.Quantity
		candles.QuoteVolume += trade.Quantity * trade.Price

		candles.AmountTrade += 1
		if !trade.IsBuyerMaker {
			candles.AmountTradeBuy += 1
			candles.ActiveBuyVolume += trade.Quantity
			candles.ActiveBuyQuoteVolume += trade.Price * trade.Quantity
		}
	}
}

func (app *Application) WriteTrade(candle *model.Candle) {

	candle.Time = candle.Time.Add(time.Minute)
	candle.CompleteTrade = true

	if candle.Volume != 0 {

		candle.StartT = false

		app.WriteTradeDatabase(*candle)

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
}

func (app *Application) WriteTradeDatabase(candle model.Candle) {

	go func() {
		candle.Time = candle.Time.Add(-1 * time.Minute)
		err := database.InsertCandlesTable(app.database, candle)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error app.WriteTradeDatabase: %v", err))
		}
	}()

}

func (app *Application) UpdateCandlesTriger() {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, pair := range app.dataFeed.Pairs {
		if !app.dataFeed.Candles[pair].CompleteTrade {
			app.WriteTrade(app.dataFeed.Candles[pair])

		}
		app.dataFeed.Candles[pair].CompleteTrade = false
	}

}
