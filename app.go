package main

import (
	"database/sql"
	"fmt"
	"main/database"
	"main/exchange"
	"main/log"
	"main/model"
	"main/service"
	"sync"
	"time"
)

type Application struct {
	mtx      sync.Mutex
	exchange service.Exchange
	dataFeed *exchange.DataFeedSubscription
	database *sql.DB

	pairs          []string
	periods        []model.Periods
	candles        map[string]map[string]*model.Candle
	nextTimeMinute map[string]time.Time // Для пар
	trigerTrade    map[string]bool      // Для периодов
}

func NewApp(exch service.Exchange, db *sql.DB, timeframe string, pairs []string, periods []model.Periods) (*Application, error) {

	app := &Application{
		exchange: exch,
		dataFeed: exchange.NewDataFeed(exch, timeframe),
		database: db,

		pairs:          pairs,
		periods:        periods,
		candles:        make(map[string]map[string]*model.Candle),
		nextTimeMinute: make(map[string]time.Time),
		trigerTrade:    make(map[string]bool),
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

		//app.nextTimeMinute[pair] = timeStart.Add(time.Minute)

		if _, ok := app.candles[pair]; !ok {
			app.candles[pair] = map[string]*model.Candle{}
		}
		for _, period := range app.periods {
			nextTime := findNextMultipleTimeV2(timeStart, period.Duration)
			app.candles[pair][period.Name] = &model.Candle{Pair: pair, Time: nextTime}
		}
	}

	// После того как получен trigerTrade , через 5 секунд делаем принудительное обновление базы данных
	// для формирования candle за период 1 минута для тех пар, которые не получили обновление
	for _, period := range app.periods {
		go func(p model.Periods) {
			for {
				if app.GetTrigerTrade(p.Name) {
					timer := time.NewTimer(5 * time.Second)
					<-timer.C
					app.SetTrigerTrade(p.Name, false)
					app.UpdateCandlesTriger(p)
				}
			}
		}(period)
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
			app.trigerTrade[period.Name] = true
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

func (app *Application) WriteTrade(candle *model.Candle, period model.Periods) {

	candle.Time = candle.Time.Add(period.Duration)
	candle.CompleteTrade = true

	// TODO что это
	// if candle.Volume != 0 {

	candle.StartT = false

	app.WriteTradeDatabase(*candle, period)

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

	//}
}

func (app *Application) WriteTradeDatabase(candle model.Candle, period model.Periods) {

	go func() {
		candle.Time = candle.Time.Add(-1 * period.Duration)
		err := database.InsertCandlesTableName(app.database, period.Name, candle)
		if err != nil {
			log.MyLogger.ErrorOut(fmt.Errorf("error app.WriteTradeDatabase: %v", err))
		}
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
}

func (app *Application) GetTrigerTrade(period string) bool {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	return app.trigerTrade[period]
}

func (app *Application) SetTrigerTrade(period string, triger bool) {
	app.mtx.Lock()
	defer app.mtx.Unlock()
	app.trigerTrade[period] = triger
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
