package main

import (
	"context"
	"log"
	"os"

	"main/database"
	"main/exchange"
	"main/model"
)

func main() {

	ctx := context.Background()

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// pairs, err := binance.GetPairsToUSDT()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	pairs := []string{"BTCUSDT"}

	settings := model.Settings{
		Pairs:     pairs,
		Timeframe: "1m",
	}

	infoLogFile, err := os.OpenFile("log/info.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer infoLogFile.Close()

	errorLogFile, err := os.OpenFile("log/error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer errorLogFile.Close()

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
	app.infoLog = log.New(infoLogFile, "INFO\t", log.Ldate|log.Ltime)
	app.errorLog = log.New(errorLogFile, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	app.Run()
}
