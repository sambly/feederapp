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
	wg        *sync.WaitGroup
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
		wg:        &sync.WaitGroup{},
		timeframe: timeframe,
		exchange:  exchange,
		DataFeeds: make(map[string]*DataFeed),
	}

	return data
}

func (d *DataFeedSubscription) SubscribeTrade(ctx context.Context, pair string, consumer TradeFeedConsumer) {
	// Подписки на websocket
	d.wg.Add(1)
	if _, ok := d.DataFeeds[pair]; !ok {
		d.DataFeeds[pair] = &DataFeed{}
	}
	ctrade, cerrt := d.exchange.TradesSubscription(ctx, pair, d.wg)
	d.DataFeeds[pair].DataTrade = ctrade
	d.DataFeeds[pair].ErrTrade = cerrt

	d.DataFeeds[pair].SubscriptionsByTrade = append(d.DataFeeds[pair].SubscriptionsByTrade, SubscriptionByTrade{
		consumer: consumer,
	})
}

func (d *DataFeedSubscription) Start(ctx context.Context) error {

	for key, feed := range d.DataFeeds {
		d.wg.Add(1)
		go func(key string, feed *DataFeed) {
			defer d.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case trade, ok := <-feed.DataTrade:
					if !ok {
						logging.MyLogger.InfoLog.Println("stopping data feed:", key)
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

	// Завершение подписок по websocket
	d.wg.Wait()
	logging.MyLogger.InfoLog.Println("Все подписки завершены")
	return nil
}
