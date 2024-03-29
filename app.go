package main

import (
	"database/sql"
	"fmt"
	"main/database"
	"main/exchange"
	"main/log"
	"main/model"
	"main/service"
	"runtime"
	"sync"
	"time"
)

type Application struct {
	mtx      sync.Mutex
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB

	pairs         []string
	periods       []model.Periods
	candles       map[string]map[string]*model.Candle
	candlesBuffer map[string][]model.Candle
	trigerTrade   map[string]*trigerTrade
}

type trigerTrade struct {
	signal chan bool
	active bool
}

func NewApp(exch service.Exchange, db *sql.DB, timeframe string, pairs []string, periods []model.Periods) (*Application, error) {

	app := &Application{
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, timeframe),
		database: db,

		pairs:         pairs,
		periods:       periods,
		candles:       make(map[string]map[string]*model.Candle),
		candlesBuffer: make(map[string][]model.Candle),
		trigerTrade:   make(map[string]*trigerTrade),
	}
	return app, nil
}

func (app *Application) Run() error {

	timeStart := time.Now().Truncate(time.Minute)

	fmt.Println("Время старта : ", timeStart)

	// Для правильного расчета
	timeStart = timeStart.Add(time.Minute)

	for _, pair := range app.pairs {

		app.dataFeed.SubscribeTrade(pair, app.onTrade)

		if _, ok := app.candles[pair]; !ok {
			app.candles[pair] = map[string]*model.Candle{}
		}
		for _, period := range app.periods {
			nextTime := findNextMultipleTimeV2(timeStart, period.Duration)
			app.candles[pair][period.Name] = &model.Candle{Pair: pair, Time: nextTime}
		}
	}

	for _, period := range app.periods {
		app.candlesBuffer[period.Name] = []model.Candle{}
		app.trigerTrade[period.Name] = &trigerTrade{signal: make(chan bool), active: false}
	}

	periodByName := func(name string) model.Periods {
		for _, period := range app.periods {
			if period.Name == name {
				return period
			}
		}
		return model.Periods{}
	}

	for name, triger := range app.trigerTrade {
		go func(name string, triger *trigerTrade) {
			for range triger.signal {
				app.SetTrigerTrade(name, true)
				timer := time.NewTimer(5 * time.Second)
				<-timer.C
				fmt.Println("Сработал таймер записи периода:", name)
				app.SetTrigerTrade(name, false)
				app.UpdateCandlesTriger(periodByName(name))
			}
		}(name, triger)
	}

	app.dataFeed.Start(true)

	return nil
}

func (app *Application) onTrade(trade model.Trade) {

	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, period := range app.periods {

		candle := app.candles[trade.Pair][period.Name]

		difTime := trade.Time.Sub(candle.Time)

		if difTime >= time.Duration(period.Duration) {
			// Запускаем таймер для полной записи всех пар
			if !app.trigerTrade[period.Name].active {
				fmt.Println("Ща отправим сигнал на запись ", period.Name)
				app.trigerTrade[period.Name].signal <- true
			}
			app.WriteTrade(candle, period)
			difTime = trade.Time.Sub(candle.Time)

		}

		if difTime >= 0 {

			if !candle.StartT {
				candle.StartT = true
				candle.Open = trade.Price
				candle.Low = trade.Price
				candle.High = 0
			}

			candle.Price = trade.Price
			if trade.Price > candle.High {
				candle.High = trade.Price
			}

			if trade.Price < candle.Low {
				candle.Low = trade.Price
			}
			candle.Close = trade.Price
			candle.Volume += trade.Quantity
			candle.QuoteVolume += trade.Quantity * trade.Price

			candle.AmountTrade += 1
			if !trade.IsBuyerMaker {
				candle.AmountTradeBuy += 1
				candle.ActiveBuyVolume += trade.Quantity
				candle.ActiveBuyQuoteVolume += trade.Price * trade.Quantity
			}
		}

	}
}

func (app *Application) WriteCandleBuffer(candle model.Candle, period model.Periods) {
	candle.Time = candle.Time.Add(-1 * period.Duration)
	app.candlesBuffer[period.Name] = append(app.candlesBuffer[period.Name], candle)
}

func (app *Application) WriteTrade(candle *model.Candle, period model.Periods) {

	candle.Time = candle.Time.Add(period.Duration)
	candle.CompleteTrade = true

	candle.StartT = false

	app.WriteCandleBuffer(*candle, period)

	candle.Price = 0
	candle.Low = 0
	candle.High = 0
	candle.Open = 0
	candle.Close = 0
	candle.Volume = 0
	candle.QuoteVolume = 0
	candle.AmountTrade = 0
	candle.AmountTradeBuy = 0
	candle.ActiveBuyVolume = 0
	candle.ActiveBuyQuoteVolume = 0

}

func (app *Application) WriteTradeDatabase(period model.Periods) {

	go func() {
		start := time.Now()

		err := database.InsertCandlesTableNameV3(app.database, period.Name, app.candlesBuffer[period.Name])
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error app.WriteTradeDatabase: %v", err))
		}

		app.candlesBuffer[period.Name] = []model.Candle{}

		duration := time.Since(start)
		log.MyLogger.InfoLog.Printf("t:%v  period %s ", duration, period.Name)

	}()
}

func (app *Application) UpdateCandlesTriger(period model.Periods) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	for _, pair := range app.pairs {
		candle := app.candles[pair][period.Name]
		if !candle.CompleteTrade {
			app.WriteTrade(candle, period)

		}
		candle.CompleteTrade = false
	}

	// Запись в базу данных
	app.WriteTradeDatabase(period)

}

func (app *Application) SetTrigerTrade(period string, value bool) {
	app.mtx.Lock()
	defer app.mtx.Unlock()

	fmt.Println("SetTrigerTrade:", period, value)
	app.trigerTrade[period].active = value
}

// Поиск близжайшего времени большего времени кратное заданному интервалу
func findNextMultipleTime(t time.Time, interval time.Duration) time.Time {
	for {
		// TODO здесь может сделать увеличение на 1 минуту
		t = t.Add(1 * time.Second) // Увеличиваем на 1 секунду для предотвращения зацикливания на текущем времени
		if t.Unix()%int64(interval.Seconds()) == 0 {
			break
		}
	}
	return t
}

// Проверка на кратность времени
func isTimeMultipleOfInterval(t time.Time, interval time.Duration) bool {
	startTime := time.Unix(0, 0) // Начальное время (начало Unix эпохи)
	return t.Sub(startTime)%interval == 0
}
func findNextMultipleTimeV2(t time.Time, interval time.Duration) time.Time {
	// Находим ближайшее время, которое кратно интервалу, начиная с t
	remainder := t.Unix() % int64(interval.Seconds())
	if remainder != 0 {
		seconds := int64(interval.Seconds())
		// Добавляем оставшееся время до следующего кратного интервала
		t = t.Add(time.Duration(seconds-remainder) * time.Second)
		// Добавляем этот же период времени, так как нужно дождаться чтобы все данные успели сформироваться
	}
	t = t.Add(interval)
	return t
}

func checkForDeadlock() {
	for {
		time.Sleep(5 * time.Second) // Проверка каждые 5 секунд
		goroutineCount := runtime.NumGoroutine()
		fmt.Println("Number of goroutines:", goroutineCount)
	}
}
