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
	"google.golang.org/grpc"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.InitLogger(cfg.DebugLog, cfg.ProductionLog); err != nil {
		log.Fatalf("failed to InitLogger: %v", err)
	}

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

	binance, err := exchange.NewBinance(ctx,
		exchange.WithBinanceLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
	)
	if err != nil {
		mainLogger.Fatalf("failed to create exchange instance: %v", err)
	}

	pairs, err := binance.GetPairsToUSDT(ctx)
	if err != nil {
		mainLogger.Fatal(err)
	}

	mainLogger.Infof("колличество пар: %v", len(pairs))

	db, err := database.DbInit(cfg.NameDb, cfg.HostDb, cfg.PortDb, cfg.UserDb, cfg.PasswordDb)
	if err != nil {
		mainLogger.Fatal(err)
	}

	var exflow exchange.Exflow
	var conn *grpc.ClientConn
	switch cfg.ExchangeType {
	case "exchange":
		exflow = binance
	case "grpc":
		exflow, conn, err = exchange.NewClientGrpc(
			fmt.Sprintf("%s:%s", cfg.GrpcHost, cfg.GrpcPort),
			exchange.WithClientLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
		)
		if err != nil {
			mainLogger.Fatalf("did not connect to grpc: %v", err)
		}
		defer conn.Close()
	}

	dataFeed := exchange.NewDataFeed(
		exflow,
		exchange.WithDataFeedLogger(logadapter.NewLogrusAdapter(logger.AddFieldsEmpty())),
	)
	if err != nil {
		mainLogger.Fatalf("failed to initialize data feed: %v", err)
	}

	app, err := app.NewApp(dataFeed, db, pairs, periods)
	if err != nil {
		mainLogger.Fatal(err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.Run(gCtx)
	})

	fmt.Println("Приложение feeder-app запушено")

	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		mainLogger.Fatalf("приложение feeder-app завершено с ошибкой: %v", err)
	} else {
		mainLogger.Info("приложение feeder-app завершено")
	}

	fmt.Println("Приложение feeder-app завершено")
}
