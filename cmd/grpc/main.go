package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/feederApp/internal/app"
	"github.com/sambly/feederApp/internal/config"
	"github.com/sambly/feederApp/internal/database"
	"github.com/sambly/feederApp/internal/logging"
	"github.com/sambly/feederApp/internal/model"

	"golang.org/x/sync/errgroup"
)

func main() {

	var a string

	_ = a

	logging.InitLogger()
	logging.MyLogger.InfoLog.Println("Запуск приложения")

	config, err := config.NewConfig()
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		log.Fatal(err)
	}

	logging.MyLogger.InfoLog.Println("Колличество пар : ", len(pairs))

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

	app, err := app.NewApp(binance, db, "1m", pairs, periods)
	if err != nil {
		log.Fatal(err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.Run(gCtx)
	})

	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		log.Fatalf("Приложение завершено с ошибкой: %v", err)
	} else {
		logging.MyLogger.InfoLog.Println("Приложение завершено")
	}

}
