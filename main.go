package main

import (
	"context"
	"log"

	"main/database"
	"main/exchange"
	mylog "main/log"
	"main/model"
)

func main() {

	ctx := context.Background()

	mylog.InitLogger()

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	//pairs := []string{"BTCUSDT", "ILVUSDT", "ETCUSDT", "ETHUSDT", "BNBUSDT", "SANDUSDT"}

	settings := model.Settings{
		Pairs:     pairs,
		Timeframe: "1m",
	}

	db, err := database.DbConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = database.CreateCandlesTable(db)
	if err != nil {
		log.Fatal(err)
	}

	app, err := NewApp(binance, settings, db)
	if err != nil {
		log.Fatal(err)
	}

	app.Run()
}
