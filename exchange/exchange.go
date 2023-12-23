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
	DataCandle            chan model.Candle
	DataTrade             chan model.Trade
	ErrCandle             chan error
	ErrTrade              chan error
	SubscriptionsByCandle []SubscriptionByCandle
	SubscriptionsByTrade  []SubscriptionByTrade
}

type DataFeedSubscription struct {
	timeframe       string
	exchange        service.Exchange
	Pairs           []string
	Candles         map[string]*model.Candle
	DataFeeds       map[string]*DataFeed
	CandleComplete  map[string]chan model.Candle
	TimeStartTrade  time.Time
	TimeStartCandle time.Time
	TradeOn         bool
	CandleOn        bool
}

type SubscriptionByCandle struct {
	consumer CandleFeedConsumer
}

type SubscriptionByTrade struct {
	consumer TradeFeedConsumer
}

type CandleFeedConsumer func(model.Candle)
type TradeFeedConsumer func(model.Trade)

func NewDataFeed(exchange service.Exchange, timeframe string, pairs []string) *DataFeedSubscription {

	data := &DataFeedSubscription{
		timeframe:      timeframe,
		exchange:       exchange,
		Pairs:          pairs,
		Candles:        make(map[string]*model.Candle),
		DataFeeds:      make(map[string]*DataFeed),
		CandleComplete: make(map[string]chan model.Candle),
		TradeOn:        false,
		CandleOn:       false,
	}
	for _, pair := range pairs {
		if _, ok := data.Candles[pair]; !ok {
			data.Candles[pair] = &model.Candle{Pair: pair}
		}
	}
	return data
}
func (d *DataFeedSubscription) SubscribeCandle(pair string, consumer CandleFeedConsumer) {

	if _, ok := d.DataFeeds[pair]; !ok {
		d.DataFeeds[pair] = &DataFeed{}
	}
	ccandle, cerr := d.exchange.CandlesSubscription(context.Background(), pair, d.timeframe)
	d.DataFeeds[pair].DataCandle = ccandle
	d.DataFeeds[pair].ErrCandle = cerr

	d.DataFeeds[pair].SubscriptionsByCandle = append(d.DataFeeds[pair].SubscriptionsByCandle, SubscriptionByCandle{
		consumer: consumer,
	})
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
		timeNextMinuteForTrade := time.Date(timeStart.Year(), timeStart.Month(), timeStart.Day(), timeStart.Hour(), timeStart.Minute(), 0, 0, time.Local).Add(time.Minute)
		timeNextMinuteForCandle := timeNextMinuteForTrade.Add(time.Minute)
		d.TimeStartTrade = timeNextMinuteForTrade
		d.TimeStartCandle = timeNextMinuteForCandle

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
				case candle, ok := <-feed.DataCandle:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range feed.SubscriptionsByCandle {
						subscription.consumer(candle)
					}
				case trade, ok := <-feed.DataTrade:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range feed.SubscriptionsByTrade {
						subscription.consumer(trade)
					}
				case err := <-feed.ErrCandle:
					if err != nil {
						fmt.Printf("Ошибка ws candle %s", err.Error())
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
