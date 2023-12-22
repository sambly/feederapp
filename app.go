package main

import (
	"database/sql"
	"fmt"
	"log"
	"main/database"
	"main/exchange"
	"main/model"
	"main/service"
	"time"
)

type Application struct {
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
		dataFeed: exchange.NewDataFeed(exch, settings.Timeframe),
		database: db,
	}
	return app, nil
}

func (app *Application) Run() error {

	for _, pair := range app.settings.Pairs {
		app.dataFeed.Subscribe(pair, app.onTrade)
	}
	app.dataFeed.Start(true)
	return nil
}

func (app *Application) onTrade(trade model.Trade) {

	tradePairs := app.dataFeed.TradePairs

	// Меньше 0 при инцициализации
	if trade.Time.Sub(tradePairs[trade.Pair].Time) > 0 {

		if trade.Time.Sub(tradePairs[trade.Pair].Time) >= time.Minute {
			tradePairs[trade.Pair].Complete = true
		}

		tradePairs[trade.Pair].Time = trade.Time
		tradePairs[trade.Pair].Price = trade.Price
		tradePairs[trade.Pair].QuoteVolume += trade.Price * trade.Quantity
		tradePairs[trade.Pair].AmountTrade += 1
		if trade.IsBuyerMaker {
			tradePairs[trade.Pair].AmountTradeBuy += 1
			tradePairs[trade.Pair].ActiveBuyQuoteVolume += trade.Price * trade.Quantity
		}

		if tradePairs[trade.Pair].Complete {
			err := database.InsertTradesTable(app.database, tradePairs[trade.Pair])
			if err != nil {
				fmt.Println("Ошибка записи")
			}
			tradePairs[trade.Pair].QuoteVolume = 0
			tradePairs[trade.Pair].AmountTrade = 0
			tradePairs[trade.Pair].AmountTradeBuy = 0
			tradePairs[trade.Pair].ActiveBuyQuoteVolume = 0
			tradePairs[trade.Pair].Complete = false

		}

	}

}
