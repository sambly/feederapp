package exchange

import (
	"context"
	"fmt"
	"main/logging"
	"main/model"
	"main/service"
	"sync"
)

type DataFeed struct {
	DataTrade            chan model.Trade
	ErrTrade             chan error
	SubscriptionsByTrade []SubscriptionByTrade
}

type DataFeedSubscription struct {
	timeframe string
	exchange  service.Exchange
	DataFeeds map[string]*DataFeed
}

type SubscriptionByTrade struct {
	consumer TradeFeedConsumer
}

type TradeFeedConsumer func(model.Trade)

func NewDataFeed(exchange service.Exchange, timeframe string) *DataFeedSubscription {

	data := &DataFeedSubscription{
		timeframe: timeframe,
		exchange:  exchange,
		DataFeeds: make(map[string]*DataFeed),
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
						logging.MyLogger.ErrorOut(fmt.Errorf("error ws trade: %v", err))
					}
				}
			}
		}(key, feed)
	}
	if loadSync {
		wg.Wait()
	}
}
