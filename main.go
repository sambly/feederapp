package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"main/config"
	"main/database"
	"main/exchange"
	"main/gorutune"
	"main/logging"
	"main/model"
	"net/http"
	_ "net/http/pprof"

	"golang.org/x/sync/errgroup"
)

func main() {

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

	app, err := NewApp(binance, db, "1m", pairs, periods)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{Addr: ":8080"}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logging.MyLogger.InfoLog.Println("Запуск HTTP-сервера на порту 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		return app.Run(gCtx)
	})

	// Завершение работы сервиса
	g.Go(func() error {

		<-gCtx.Done()

		logging.MyLogger.InfoLog.Println("Завершение приложения")

		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := srv.Shutdown(ctxShutdown); err != nil {
			logging.MyLogger.ErrorOut(fmt.Errorf("ошибка при завершении работы HTTP-сервера: %v", err))
		}
		logging.MyLogger.InfoLog.Println("HTTP-сервер остановлен")
		return gCtx.Err()
	})

	// Запуск горутины для мониторинга количества горутин
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Printf("Number of goroutines: %d\n", atomic.LoadInt32(&gorutune.GoroutineCounter))

			case <-gCtx.Done():
				return
			}
		}
	}()

	if err := g.Wait(); err != nil && gCtx.Err() != context.Canceled {
		log.Fatalf("Приложение завершено с ошибкой: %v", err)
	} else {
		logging.MyLogger.InfoLog.Println("Приложение завершено")
	}

	fmt.Printf("Finished Number of goroutines: %d\n", atomic.LoadInt32(&gorutune.GoroutineCounter))

	gorutune.GoroutineNames.Range(func(key, value interface{}) bool {
		fmt.Printf("Remaining goroutine ID %d: %s\n", key, value)
		return true
	})

}
