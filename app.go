package main

import (
	"database/sql"
	"fmt"
	"log"
	"main/database"
	"main/exchange"
	"main/model"
	"main/service"
	"sync"
	"time"
)

type Application struct {
	mtx      sync.Mutex
	settings model.Settings
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB
	infoLog  *log.Logger
	errorLog *log.Logger
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
		app.dataFeed.SubscribeCandle(pair, app.onCandle)
		app.dataFeed.SubscribeTrade(pair, app.onTrade)
	}
	app.dataFeed.Start(true)
	return nil
}

func (app *Application) onTrade(trade model.Trade) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	difTime := trade.Time.Sub(app.dataFeed.TimeStartTrade)

	if app.dataFeed.TradeOn && difTime >= 0 && difTime < time.Duration(time.Minute.Nanoseconds()) {

		//if app.dataFeed.TradeOn {

		c := app.dataFeed.Candles[trade.Pair]

		c.Price = trade.Price
		c.VolumeT += trade.Quantity
		c.AmountTrade += 1
		if !trade.IsBuyerMaker {
			c.AmountTradeBuy += 1
			c.ActiveBuyQuoteVolume += trade.Price * trade.Quantity
		}

	}

}

func (app *Application) onCandle(candle model.Candle) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	c := app.dataFeed.Candles[candle.Pair]
	//difTime := candle.Time.Sub(app.dataFeed.TimeStartCandle)

	// if app.dataFeed.CandleOn && difTime >= 0 && difTime < time.Duration(time.Minute.Nanoseconds()) {
	if app.dataFeed.CandleOn {

		if candle.Complete {
			c.Time = candle.Time
			c.Open = candle.Open
			c.Close = candle.Close
			c.Low = candle.Low
			c.High = candle.High
			c.VolumeC = candle.VolumeC
			c.QuoteVolume = candle.QuoteVolume
			c.AmountTradeC = candle.AmountTradeC
			c.Complete = candle.Complete

		}

		if c.Complete {
			err := database.InsertCandlesTable(app.database, c)
			if err != nil {
				fmt.Println("Ошибка записи")
			}
			app.dataFeed.TimeStartTrade = app.dataFeed.TimeStartTrade.Add(time.Minute)
		}
	}

	if candle.Complete {

		c.VolumeT = 0
		c.AmountTrade = 0
		c.AmountTradeBuy = 0
		c.ActiveBuyQuoteVolume = 0
		c.Complete = false
	}

}

func (app *Application) CompleteCandle(candle chan model.Candle) {

}
