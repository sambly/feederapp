package main

import (
	"context"
	"fmt"
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
	"github.com/sambly/feederapp/internal/logger"
	"github.com/sambly/feederapp/internal/model"
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
		{Name: "1m", Duration: time.Second * 60},
		{Name: "3m", Duration: time.Minute * 3},
		{Name: "15m", Duration: time.Minute * 15},
		{Name: "1h", Duration: time.Hour},
		{Name: "4h", Duration: time.Hour * 4},
		{Name: "1d", Duration: time.Hour * 12},
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

	c, conn, err := exchange.NewClientGrpc(fmt.Sprintf("%s:%s", config.GrpcHost, config.GrpcPort))
	if err != nil {
		mainLogger.Fatalf("did not connect to grpc: %v", err)
	}

	defer conn.Close()

	dataFeed := exchange.NewDataFeed(
		c,
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
