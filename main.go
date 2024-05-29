package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"main/config"
	"main/database"
	"main/exchange"
	mylog "main/logging"
	"main/model"
	"net/http"
	_ "net/http/pprof"

	"golang.org/x/sync/errgroup"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	srv := &http.Server{Addr: ":8080"}

	// Создание группы горутин с контекстом
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		fmt.Println("Запуск HTTP-сервера на порту 8080")
		// Запуск HTTP-сервера
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		// Завершение работы сервиса
		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := srv.Shutdown(ctxShutdown); err != nil {
			log.Printf("Ошибка при завершении работы HTTP-сервера: %v", err)
		} else {
			log.Println("HTTP-сервер завершен корректно")
		}

		return err
	})

	app.Run()
}
