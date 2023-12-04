package main

import (
	"context"
	"fmt"
	"log"

	"main/exchange"
	"main/model"
)

func main() {

	ctx := context.Background()

	settings := model.Settings{
		Pairs: []string{
			"BTCUSDT",
			"ETHUSDT",
		},
		Timeframe: "15m",
	}

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}

	app, err := NewApp(binance, settings)
	if err != nil {
		log.Fatal(err)
	}
	app.Run()
	fmt.Println("Out")

}
