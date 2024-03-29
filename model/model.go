package model

import (
	"fmt"
	"time"
)

type Settings struct {
	Pairs     []string
	Periods   []Periods
	Timeframe string
}

type Periods struct {
	Name     string
	Duration time.Duration
}

type AssetInfo struct {
	BaseAsset  string
	QuoteAsset string

	MinPrice    float64
	MaxPrice    float64
	MinQuantity float64
	MaxQuantity float64
	StepSize    float64
	TickSize    float64

	QuotePrecision     int
	BaseAssetPrecision int
}

type Candle struct {
	Pair                 string
	Time                 time.Time
	UpdatedAt            time.Time
	Open                 float64
	Close                float64
	Low                  float64
	High                 float64
	StartT               bool
	Price                float64
	Volume               float64
	QuoteVolume          float64
	AmountTrade          int64
	AmountTradeBuy       int64
	ActiveBuyVolume      float64
	ActiveBuyQuoteVolume float64

	Complete      bool
	CompleteTrade bool

	// Aditional collums from CSV inputs
	Metadata map[string]float64
}

func (c Candle) String() string {
	return fmt.Sprintf("Time: %s\n"+
		"Pair: %s\n"+
		"Open: %v\n"+
		"Close: %v\n"+
		"Low: %v\n"+
		"High: %v\n"+
		"Volume: %v\n"+
		"QuoteVolume: %v\n"+
		"AmountTrade: %v\n"+
		"AmountTradeBuy: %v\n"+
		"ActiveBuyVolume: %v\n"+
		"ActiveBuyQuoteVolume: %v\n",
		c.Time, c.Pair, c.Open, c.Close, c.Low, c.High, c.Volume, c.QuoteVolume, c.AmountTrade, c.AmountTradeBuy, c.ActiveBuyVolume, c.ActiveBuyQuoteVolume)
}

type Trade struct {
	Pair         string
	Time         time.Time
	Price        float64
	Quantity     float64
	IsBuyerMaker bool
}
