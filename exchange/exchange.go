package exchange

import (
	"context"
	"fmt"
	"main/model"
	"main/service"
	"sync"
	"time"
)

type DataFeed struct {
	DataTrade            chan model.Trade
	ErrTrade             chan error
	SubscriptionsByTrade []SubscriptionByTrade
}

type DataFeedSubscription struct {
	timeframe          string
	exchange           service.Exchange
	Pairs              []string
	Candles            map[string]*model.Candle
	CandlesBufferTrade map[string]*model.Candle
	DataFeeds          map[string]*DataFeed
	TimeStartTrade     time.Time
	TimeStartCandle    time.Time
	TradeOn            bool
	CandleOn           bool
	TrigerFromTrade    chan bool
}

type SubscriptionByTrade struct {
	consumer TradeFeedConsumer
}

type TradeFeedConsumer func(model.Trade)

func NewDataFeed(exchange service.Exchange, timeframe string, pairs []string) *DataFeedSubscription {

	data := &DataFeedSubscription{
		timeframe:          timeframe,
		exchange:           exchange,
		Pairs:              pairs,
		Candles:            make(map[string]*model.Candle),
		DataFeeds:          make(map[string]*DataFeed),
		CandlesBufferTrade: make(map[string]*model.Candle),
		TradeOn:            false,
		CandleOn:           false,
	}

	return data
}

func (d *DataFeedSubscription) SubscribeTrade(pair string, consumer TradeFeedConsumer) {

	if _, ok := d.DataFeeds[pair]; !ok {
		d.DataFeeds[pair] = &DataFeed{}
	}
	ctrade, cerrt := d.exchange.TradesSubscription(context.Background(), pair)
	d.DataFeeds[pair].DataTrade = ctrade
	d.DataFeeds[pair].ErrTrade = cerrt

	d.DataFeeds[pair].SubscriptionsByTrade = append(d.DataFeeds[pair].SubscriptionsByTrade, SubscriptionByTrade{
		consumer: consumer,
	})
}

func (d *DataFeedSubscription) Start(loadSync bool) {

	wg := new(sync.WaitGroup)

	// Ждем следующую минуту, чтобы ws trade начал заполняться с начала минуты
	go func() {
		timeStart := time.Now()
		fmt.Printf("Время старта: %s\n", timeStart.Format("15:04:05.00"))

		timeNextMinuteForTrade := time.Date(timeStart.Year(), timeStart.Month(), timeStart.Day(), timeStart.Hour(), timeStart.Minute(), 0, 0, time.Local).Add(time.Minute)
		timeNextMinuteForCandle := timeNextMinuteForTrade.Add(time.Minute)
		d.TimeStartTrade = timeNextMinuteForTrade
		d.TimeStartCandle = timeNextMinuteForCandle

		for _, pair := range d.Pairs {
			d.Candles[pair] = &model.Candle{Pair: pair, Time: timeNextMinuteForTrade}
		}

		for !d.TradeOn || !d.CandleOn {
			now := time.Now()
			currentTime := now.Add(-time.Duration(now.Nanosecond()))
			if currentTime.Equal(timeNextMinuteForTrade) {
				d.TradeOn = true
			}
			if currentTime.Equal(timeNextMinuteForCandle) {
				d.CandleOn = true
			}

		}
	}()

	for key, feed := range d.DataFeeds {
		wg.Add(1)
		go func(key string, feed *DataFeed) {
			for {
				select {

				case trade, ok := <-feed.DataTrade:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range feed.SubscriptionsByTrade {
						subscription.consumer(trade)
					}
				case err := <-feed.ErrTrade:
					if err != nil {
						fmt.Printf("Ошибка ws trade %s", err.Error())
					}
				}
			}
		}(key, feed)
	}
	if loadSync {
		wg.Wait()
	}
}
