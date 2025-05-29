package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"net/http"
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

	var serviceReady bool
	httpReady := make(chan struct{})
	appReady := make(chan struct{})

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
		// TODO
		close(appReady)
		return app.Run(gCtx)
	})

	// HTTP сервер
	g.Go(func() error {
		if cfg.HTTPPort == "" {
			close(httpReady)
			mainLogger.Info("HTTP healthcheck server disabled (no port specified)")
			return nil
		}

		httpServer := &http.Server{
			Addr: ":" + cfg.HTTPPort,
		}
		// Готовность компонента
		close(httpReady)

		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if serviceReady {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		})
		// shutdown
		go func() {
			<-gCtx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(ctx); err != nil {
				mainLogger.Warn("HTTP shutdown error", err)
			}
		}()
		return httpServer.ListenAndServe()
	})

	g.Go(func() error {
		select {
		case <-httpReady:
			mainLogger.Debug("HTTP сервер готов")
		case <-gCtx.Done():
			return gCtx.Err()
		}

		select {
		case <-appReady:
			mainLogger.Debug("app ready")
		case <-gCtx.Done():
			return gCtx.Err()
		}

		serviceReady = true
		mainLogger.Info("Приложение feeder-app запушено")
		fmt.Println("Приложение feeder-app запушено")
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		mainLogger.Errorf("feeder-app error: %v", err)
	}
	mainLogger.Info("Приложение feeder-app завершено")
	fmt.Println("Приложение feeder-app завершено")
}
