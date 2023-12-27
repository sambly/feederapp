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
	mtx         sync.Mutex
	settings    model.Settings
	exchange    service.Exchange
	dataFeed    *exchange.DataFeedSubscription
	database    *sql.DB
	infoLog     *log.Logger
	errorLog    *log.Logger
	candlec     chan model.Candle
	trigerTrade bool
}

func NewApp(exch service.Exchange, settings model.Settings, db *sql.DB) (*Application, error) {

	app := &Application{
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Timeframe, settings.Pairs),
		database: db,
		candlec:  make(chan model.Candle, 1),
	}
	return app, nil
}

func (app *Application) Run() error {

	for _, pair := range app.settings.Pairs {
		app.dataFeed.SubscribeTrade(pair, app.onTrade)
	}
	app.dataFeed.Start(true)

	go func() {
		for {
			if app.trigerTrade {
				timer := time.NewTimer(10 * time.Second)
				<-timer.C
				app.trigerTrade = false
				app.UpdateCandlesTriger()
			}
		}
	}()

	return nil
}

func (app *Application) onTrade(trade model.Trade) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	candles := app.dataFeed.Candles[trade.Pair]
	difTime := trade.Time.Sub(candles.Time)

	if difTime < 0 && app.dataFeed.CandleOn {
		log.Printf("запись уже была произведена для пары %s\n", candles.Pair)
		log.Printf("время этой пары: %s\n", trade.Time.Format("15:04:05.00"))
		log.Printf("целевое время: %s\n", candles.Time.Format("15:04:05.00"))

	}

	if difTime >= time.Duration(time.Minute.Nanoseconds()) {

		fmt.Printf("Произведена запись")
		app.trigerTrade = true
		app.WriteTrade(*candles)
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

func (app *Application) WriteTrade(candle model.Candle) {
	if candle.Volume != 0 {
		app.CompleteTrade(candle.Pair)
		app.WriteTradeDatabase(candle)
		app.ClearTrade(candle.Pair)
	}
}

func (app *Application) ClearTrade(pair string) {
	candles := app.dataFeed.Candles[pair]
	candles.StartT = false
	candles.Price = 0
	candles.Low = 0
	candles.High = 0
	candles.Open = 0
	candles.Close = 0
	candles.Volume = 0
	candles.QuoteVolume = 0
	candles.AmountTrade = 0
	candles.AmountTradeBuy = 0
	candles.ActiveBuyVolume = 0
	candles.ActiveBuyQuoteVolume = 0
}

func (app *Application) CompleteTrade(pair string) {
	candles := app.dataFeed.Candles[pair]
	candles.StartT = false
	candles.CompleteTrade = true
	candles.Time = candles.Time.Add(time.Minute)
}

func (app *Application) WriteTradeDatabase(candle model.Candle) {

	go func() {
		candle.Time = candle.Time.Add(-1 * time.Minute)
		err := database.InsertCandlesTable(app.database, candle)
		if err != nil {
			fmt.Println("Ошибка записи")
			fmt.Println(err)
		}
	}()

}

func (app *Application) UpdateCandlesTriger() {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, pair := range app.dataFeed.Pairs {
		if !app.dataFeed.Candles[pair].CompleteTrade {
			app.CompleteTrade(pair)
			app.WriteTradeDatabase(*app.dataFeed.Candles[pair])
			app.ClearTrade(pair)
		}
		app.dataFeed.Candles[pair].CompleteTrade = false
	}

}
