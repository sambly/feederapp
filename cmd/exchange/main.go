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
	"github.com/sambly/exchangeService/pkg/logadapter"
	"github.com/sambly/feederapp/internal/app"
	"github.com/sambly/feederapp/internal/config"
	"github.com/sambly/feederapp/internal/database"
	"github.com/sambly/feederapp/internal/model"

	"github.com/sambly/feederapp/internal/logger"

	"golang.org/x/sync/errgroup"
)

func main() {

	config, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger.InitLogger(config.Debug, config.Production)

	mainLogger := logger.AddFields(map[string]interface{}{
		"package": "main",
	})

	mainLogger.Info("запуск приложения feeder-app")

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
		mainLogger.Fatal(err)
	}

	pairs, err := binance.GetPairsToUSDT()
	if err != nil {
		mainLogger.Fatal(err)
	}

	mainLogger.Infof("колличество пар: %v", len(pairs))

	db, err := database.DbInit(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		mainLogger.Fatal(err)
	}

	dataFeed := exchange.NewDataFeedWithExchange(
		binance,
		logadapter.NewLogrusAdapter(logger.AddFieldsEmpty()),
	)

	app, err := app.NewApp(dataFeed, db, pairs, periods)
	if err != nil {
		mainLogger.Fatal(err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.Run(gCtx)
	})

	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		mainLogger.Fatalf("приложение feeder-app завершено с ошибкой: %v", err)
	} else {
		mainLogger.Info("приложение feeder-app завершено")
	}

}
