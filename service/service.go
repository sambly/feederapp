package service

import (
	"context"
	"main/model"
	"sync"
	"time"
)

type Exchange interface {
	Feeder
}

type Feeder interface {
	AssetsInfo(pair string) model.AssetInfo
	LastQuote(ctx context.Context, pair string) (float64, error)
	CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error)
	CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error)
	CandlesSubscription(ctx context.Context, pair, timeframe string) (chan model.Candle, chan error)
	TradesSubscription(ctx context.Context, pair string, wg *sync.WaitGroup) (chan model.Trade, chan error)
}
