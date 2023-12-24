package main

import (
	"database/sql"
	"fmt"
	"log"
	"main/database"
	"main/exchange"
	"main/model"
	"main/service"
	"math"
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

	candlesBuffer := app.dataFeed.CandlesBufferTrade[trade.Pair]
	difTime := trade.Time.Sub(candlesBuffer.Time)

	if difTime >= time.Duration(time.Minute.Nanoseconds()) {

		candlesBuffer.StartT = false

		candles := app.dataFeed.Candles[trade.Pair]

		candles.Price = candlesBuffer.Price
		candles.VolumeT = candlesBuffer.VolumeT
		candles.AmountTrade = candlesBuffer.AmountTrade
		candles.AmountTradeBuy = candlesBuffer.AmountTradeBuy
		candles.ActiveBuyQuoteVolume = candlesBuffer.ActiveBuyQuoteVolume
		candles.CompleteTrade = true

		candles.OpenT = candlesBuffer.OpenT
		candles.CloseT = candlesBuffer.Price
		candles.LowT = candlesBuffer.LowT
		candles.HighT = candlesBuffer.HighT

		app.CompleteCandle(candles)

		candlesBuffer.Time = candlesBuffer.Time.Add(time.Minute)
		difTime = trade.Time.Sub(candlesBuffer.Time)

		candlesBuffer.VolumeT = 0
		candlesBuffer.AmountTrade = 0
		candlesBuffer.AmountTradeBuy = 0
		candlesBuffer.ActiveBuyQuoteVolume = 0

		candlesBuffer.Price = 0
		candlesBuffer.LowT = 0
		candlesBuffer.HighT = 0
		candlesBuffer.OpenT = 0
		candlesBuffer.CloseT = 0

	}

	if difTime >= 0 {
		if !candlesBuffer.StartT {
			candlesBuffer.StartT = true
			candlesBuffer.OpenT = trade.Price
		}
		if candlesBuffer.Price == 0 {
			candlesBuffer.LowT = trade.Price
			candlesBuffer.HighT = 0
		}
		candlesBuffer.Price = trade.Price

		if candlesBuffer.Price > candlesBuffer.HighT {
			candlesBuffer.HighT = candlesBuffer.Price
		}

		if candlesBuffer.Price < candlesBuffer.LowT {
			candlesBuffer.LowT = candlesBuffer.Price
		}

		candlesBuffer.VolumeT += math.Round(trade.Quantity*100000) / 100000
		candlesBuffer.AmountTrade += 1
		if !trade.IsBuyerMaker {
			candlesBuffer.AmountTradeBuy += 1
			candlesBuffer.ActiveBuyQuoteVolume += trade.Price * trade.Quantity
		}
	}
}

func (app *Application) onCandle(candle model.Candle) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	c := app.dataFeed.Candles[candle.Pair]

	if app.dataFeed.CandleOn {

		if candle.Complete {
			c.Open = candle.Open
			c.Close = candle.Close
			c.Low = candle.Low
			c.High = candle.High
			c.VolumeC = candle.VolumeC
			c.QuoteVolume = candle.QuoteVolume
			c.AmountTradeC = candle.AmountTradeC
			c.Complete = candle.Complete

			app.CompleteCandle(c)
		}

	}

}

func (app *Application) CompleteCandle(candle *model.Candle) {

	if candle.Complete && candle.CompleteTrade {

		err := database.InsertCandlesTable(app.database, candle)
		if err != nil {
			fmt.Println("Ошибка записи")
			fmt.Println(err)
		}

		candle.Time = candle.Time.Add(time.Minute)
		candle.VolumeT = 0
		candle.AmountTrade = 0
		candle.AmountTradeBuy = 0
		candle.ActiveBuyQuoteVolume = 0
		candle.Complete = false
		candle.CompleteTrade = false

	}

}
