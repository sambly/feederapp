package main

import (
	"context"
	"log"
	"time"

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
	// pairs, err := binance.GetPairsToUSDT()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	pairs := []string{"ILVUSDT"}

	periods := []model.Periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 4},
	}

	db, err := database.DbConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, period := range periods {
		err = database.CreateTableName(db, period.Name)
		if err != nil {
			log.Fatal(err)
		}
	}

	timeframe := "1m"

	app, err := NewApp(binance, db, timeframe, pairs, periods)
	if err != nil {
		log.Fatal(err)
	}

	app.Run()
}
