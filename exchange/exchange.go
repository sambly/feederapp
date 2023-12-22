package exchange

import (
	"context"
	"errors"
	"fmt"
	"main/model"
	"main/service"
	"sync"
	"time"
)

var (
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInsufficientFunds = errors.New("insufficient funds or locked")
	ErrInvalidAsset      = errors.New("invalid asset")
)

type OrderError struct {
	Err      error
	Pair     string
	Quantity float64
}

func (o *OrderError) Error() string {
	return fmt.Sprintf("order error: %v", o.Err)
}

type DataFeed struct {
	Data chan model.Trade
	Err  chan error
}

type DataFeedSubscription struct {
	timeframe               string
	exchange                service.Exchange
	Pairs                   []string
	TradePairs              map[string]*model.TableCandle
	DataFeeds               map[string]*DataFeed
	SubscriptionsByDataFeed map[string][]Subscription
}

type Subscription struct {
	consumer DataFeedConsumer
}
type DataFeedConsumer func(model.Trade)

func NewDataFeed(exchange service.Exchange, timeframe string) *DataFeedSubscription {
	return &DataFeedSubscription{
		timeframe:               timeframe,
		exchange:                exchange,
		Pairs:                   make([]string, 0),
		TradePairs:              make(map[string]*model.TableCandle),
		DataFeeds:               make(map[string]*DataFeed),
		SubscriptionsByDataFeed: make(map[string][]Subscription),
	}
}
func (d *DataFeedSubscription) Subscribe(pair string, consumer DataFeedConsumer) {
	d.Pairs = append(d.Pairs, pair)

	t := time.Now()
	if _, ok := d.TradePairs[pair]; !ok {
		d.TradePairs[pair] = &model.TableCandle{
			Pair: pair,
			Time: time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local).Add(time.Minute), // Ицициализация времени со следующей минуты
		}
	}
	d.SubscriptionsByDataFeed[pair] = append(d.SubscriptionsByDataFeed[pair], Subscription{
		consumer: consumer,
	})
}

func (d *DataFeedSubscription) Connect() {
	for _, pair := range d.Pairs {
		ccandle, cerr := d.exchange.TradesSubscription(context.Background(), pair)
		d.DataFeeds[pair] = &DataFeed{
			Data: ccandle,
			Err:  cerr,
		}
	}
}

func (d *DataFeedSubscription) Start(loadSync bool) {
	d.Connect()
	wg := new(sync.WaitGroup)
	for key, feed := range d.DataFeeds {
		wg.Add(1)
		go func(key string, feed *DataFeed) {
			for {
				select {
				case candle, ok := <-feed.Data:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range d.SubscriptionsByDataFeed[key] {
						subscription.consumer(candle)
					}
				case err := <-feed.Err:
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}(key, feed)
	}
	if loadSync {
		wg.Wait()
	}
}
