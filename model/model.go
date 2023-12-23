package model

import "time"

type Settings struct {
	Pairs     []string
	Timeframe string
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
	Price                float64 // временно
	VolumeC              float64 // Объем от Candle
	VolumeT              float64 // Объем от trade  Для сравнения
	QuoteVolume          float64
	AmountTradeC         int64
	AmountTrade          int64
	AmountTradeBuy       int64
	ActiveBuyVolume      float64
	ActiveBuyQuoteVolume float64

	Complete bool

	// Aditional collums from CSV inputs
	Metadata map[string]float64
}

type Trade struct {
	Pair         string
	Time         time.Time
	Price        float64
	Quantity     float64
	IsBuyerMaker bool
}

type TableCandle struct {
	Pair                 string
	Time                 time.Time
	Price                float64
	QuoteVolume          float64
	AmountTrade          int64
	AmountTradeBuy       int64
	ActiveBuyQuoteVolume float64
	Complete             bool
}
