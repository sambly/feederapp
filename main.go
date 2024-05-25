package main

import (
	"context"
	"log"
	"time"

	"main/config"
	"main/database"
	"main/exchange"
	mylog "main/logging"
	"main/model"
	"net/http"
	_ "net/http/pprof"
)

func main() {

	ctx := context.Background()

	mylog.InitLogger()

	config, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	periods := []model.Periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 4},
		{Name: "ch12h", Duration: time.Hour * 12},
	}

	db, err := database.DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
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

	go http.ListenAndServe(":8080", nil)

	app.Run()
}
