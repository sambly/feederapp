package main

import (
	"database/sql"
	"log"
	"main/database"
	"main/exchange"
	"main/model"
	"main/service"
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
		app.dataFeed.Subscribe(pair, app.onCandle)
	}
	app.dataFeed.Start(true)
	return nil
}

func (app *Application) onCandle(candle model.Candle) {
	if candle.Complete {
		if err := database.InsertCandlesTables(app.database, candle); err != nil {
			app.logError(err)
		}
	}
}
