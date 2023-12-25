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
	mtx          sync.Mutex
	settings     model.Settings
	exchange     service.Exchange
	dataFeed     *exchange.DataFeedSubscription
	database     *sql.DB
	infoLog      *log.Logger
	errorLog     *log.Logger
	candleTriger bool
	candlec      chan model.Candle
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

		//app.CompleteTrade(candles.Pair)
		fmt.Println("shag1")
		app.WriteTrade(*candles)
		fmt.Println("shag3")
		// app.ClearTrade(candles.Pair)
		difTime = trade.Time.Sub(candles.Time)
	}

	if difTime >= 0 {

		if !candles.StartT {
			candles.StartT = true
			candles.CompleteTrade = false
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

	app.candlec <- candle

	// Один запуск
	if !app.candleTriger {
		app.candleTriger = true
		//candlec := make(chan model.Candle, 1)

		var numChan int
		var numChanNotVolume int
		var numNotChanNotVolume int
		var numNotChanVolume int
		go func() {
			timer := time.NewTimer((10 * time.Second))
			//candlec <- candle
			timerWork := true
			for {
				select {
				case c := <-app.candlec:

					if !app.dataFeed.Candles[c.Pair].CompleteTrade {
						if !timerWork {
							timer.Reset(10 * time.Second)
						}
						if app.dataFeed.Candles[c.Pair].Volume != 0 {
							app.CompleteTrade(c.Pair)
							app.WriteTradeDatabase(c)
							app.ClearTrade(c.Pair)
							numChan = numChan + 1
						} else {
							numChanNotVolume = numChanNotVolume + 1
						}
					}

				case <-timer.C:
					// Запись остальных пар
					for _, pair := range app.dataFeed.Pairs {
						if !app.dataFeed.Candles[pair].CompleteTrade {

							if app.dataFeed.Candles[pair].Volume != 0 {
								app.CompleteTrade(pair)
								app.WriteTradeDatabase(*app.dataFeed.Candles[pair])
								app.ClearTrade(pair)
								numNotChanVolume = numNotChanVolume + 1
							} else {
								numNotChanNotVolume = numNotChanNotVolume + 1
							}
							app.dataFeed.Candles[pair].CompleteTrade = false
						}

					}
					fmt.Printf("Колличество пар  %v\n", len(app.dataFeed.Pairs))
					fmt.Printf("numChan  %v\n", numChan)
					fmt.Printf("numChanNotVolume  %v\n", numChanNotVolume)
					fmt.Printf("numNotChanNotVolume  %v\n", numNotChanNotVolume)
					fmt.Printf("numNotChanVolume  %v\n", numNotChanVolume)
					if (numChan + numChanNotVolume + numNotChanNotVolume + numNotChanVolume) == len(app.dataFeed.Pairs) {
						fmt.Println("Количество пар сходится")
					}
					fmt.Println()
					timer.Stop()
					timerWork = false
					//return
				}
			}
		}()

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
	//candles.Time = candles.Time.Add(time.Minute)
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
