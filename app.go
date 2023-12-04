package main

import (
	"fmt"
	"main/exchange"
	"main/model"
	"main/service"
)

type App struct {
	settings model.Settings
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
}

func NewApp(exch service.Exchange, settings model.Settings) (*App, error) {

	app := &App{
		settings: settings,
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, settings.Timeframe),
	}
	return app, nil
}

func (n *App) Run() error {

	for _, pair := range n.settings.Pairs {
		n.dataFeed.Subscribe(pair, n.onCandle)
	}
	n.dataFeed.Start(true)
	return nil
}

func (n *App) onCandle(candle model.Candle) {
	fmt.Println(candle)
}
