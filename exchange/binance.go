package exchange

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"main/model"

	"github.com/adshao/go-binance/v2"
	"github.com/jpillora/backoff"
)

var Count int
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

type MetadataFetchers func(pair string, t time.Time) (string, float64)

type Binance struct {
	ctx context.Context

	client     *binance.Client
	assetsInfo map[string]model.AssetInfo

	APIKey    string
	APISecret string

	MetadataFetchers []MetadataFetchers
}

type BinanceOption func(*Binance)

// WithBinanceCredentials will set Binance credentials
func WithBinanceCredentials(key, secret string) BinanceOption {
	return func(b *Binance) {
		b.APIKey = key
		b.APISecret = secret
	}
}

// WithMetadataFetcher will execute a function after receive a new candle and include additional
// information to candle's metadata
func WithMetadataFetcher(fetcher MetadataFetchers) BinanceOption {
	return func(b *Binance) {
		b.MetadataFetchers = append(b.MetadataFetchers, fetcher)
	}
}

// NewBinance create a new Binance exchange instance
func NewBinance(ctx context.Context, options ...BinanceOption) (*Binance, error) {
	binance.WebsocketKeepalive = true
	exchange := &Binance{ctx: ctx}
	for _, option := range options {
		option(exchange)
	}

	exchange.client = binance.NewClient(exchange.APIKey, exchange.APISecret)
	err := exchange.client.NewPingService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance ping fail: %w", err)
	}
	timeOffset, err := exchange.client.NewSetServerTimeService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance timeOffset fail: %w", err)
	}
	exchange.client.TimeOffset = timeOffset

	results, err := exchange.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize with orders precision and assets limits
	exchange.assetsInfo = make(map[string]model.AssetInfo)
	for _, info := range results.Symbols {
		tradeLimits := model.AssetInfo{
			BaseAsset:          info.BaseAsset,
			QuoteAsset:         info.QuoteAsset,
			BaseAssetPrecision: info.BaseAssetPrecision,
			QuotePrecision:     info.QuotePrecision,
		}
		for _, filter := range info.Filters {
			if typ, ok := filter["filterType"]; ok {
				if typ == string(binance.SymbolFilterTypeLotSize) {
					tradeLimits.MinQuantity, _ = strconv.ParseFloat(filter["minQty"].(string), 64)
					tradeLimits.MaxQuantity, _ = strconv.ParseFloat(filter["maxQty"].(string), 64)
					tradeLimits.StepSize, _ = strconv.ParseFloat(filter["stepSize"].(string), 64)
				}

				if typ == string(binance.SymbolFilterTypePriceFilter) {
					tradeLimits.MinPrice, _ = strconv.ParseFloat(filter["minPrice"].(string), 64)
					tradeLimits.MaxPrice, _ = strconv.ParseFloat(filter["maxPrice"].(string), 64)
					tradeLimits.TickSize, _ = strconv.ParseFloat(filter["tickSize"].(string), 64)
				}
			}
		}
		exchange.assetsInfo[info.Symbol] = tradeLimits
	}

	return exchange, nil
}

func (b *Binance) LastQuote(ctx context.Context, pair string) (float64, error) {
	candles, err := b.CandlesByLimit(ctx, pair, "1m", 1)
	if err != nil || len(candles) < 1 {
		return 0, err
	}
	return candles[0].Close, nil
}

func (b *Binance) AssetsInfo(pair string) model.AssetInfo {
	return b.assetsInfo[pair]
}

func (b *Binance) validate(pair string, quantity float64) error {
	info, ok := b.assetsInfo[pair]
	if !ok {
		return ErrInvalidAsset
	}

	if quantity > info.MaxQuantity || quantity < info.MinQuantity {
		return &OrderError{
			Err:      fmt.Errorf("%w: min: %f max: %f", ErrInvalidQuantity, info.MinQuantity, info.MaxQuantity),
			Pair:     pair,
			Quantity: quantity,
		}
	}

	return nil
}

func (b *Binance) CandlesSubscription(ctx context.Context, pair, period string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)

	go func() {
		defer close(ccandle)
		defer close(cerr)
		ba := &backoff.Backoff{
			Min: 100 * time.Millisecond,
			Max: 1 * time.Second,
		}

		for {
			done, _, err := binance.WsKlineServe(pair, period, func(event *binance.WsKlineEvent) {
				ba.Reset()

				candle := CandleFromWsKline(pair, event.Kline)

				select {
				case <-ctx.Done():
					return
				case ccandle <- candle:
				}

			}, func(err error) {

				select {
				case <-ctx.Done():
					return
				case cerr <- err:
				}
			})
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case cerr <- err:
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-done:
				time.Sleep(ba.Duration())
			}
		}
	}()

	return ccandle, cerr
}

func (b *Binance) TradesSubscription(ctx context.Context, pair string, wg *sync.WaitGroup) (chan model.Trade, chan error) {
	ctrade := make(chan model.Trade)
	cerr := make(chan error)
	go func() {
		defer close(ctrade)
		defer close(cerr)
		defer wg.Done()
		ba := &backoff.Backoff{
			Min: 100 * time.Millisecond,
			Max: 1 * time.Second,
		}

		for {
			done, _, err := binance.WsTradeServe(pair, func(event *binance.WsTradeEvent) {
				ba.Reset()

				t := time.Unix(0, event.TradeTime*int64(time.Millisecond))
				trade := model.Trade{Pair: event.Symbol, Time: t}
				trade.Price, _ = strconv.ParseFloat(event.Price, 64)
				trade.Quantity, _ = strconv.ParseFloat(event.Quantity, 64)
				trade.IsBuyerMaker = event.IsBuyerMaker

				select {
				case <-ctx.Done():
					return
				default:
					select {
					case ctrade <- trade:
					case <-ctx.Done():
						return
					}
				}

			}, func(err error) {

				select {
				case <-ctx.Done():
					return
				default:
					select {
					case cerr <- err:
					case <-ctx.Done():
						return
					}
				}
			})
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					select {
					case cerr <- err:
					case <-ctx.Done():
						return
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
				select {
				case <-done:
					time.Sleep(ba.Duration())
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ctrade, cerr
}

func (b *Binance) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()

	data, err := klineService.Symbol(pair).
		Interval(period).
		Limit(limit + 1).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candle := CandleFromKline(pair, *d)

		candles = append(candles, candle)
	}

	// discard last candle, because it is incomplete
	return candles[:len(candles)-1], nil
}

func (b *Binance) CandlesByPeriod(ctx context.Context, pair, period string,
	start, end time.Time) ([]model.Candle, error) {

	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()

	data, err := klineService.Symbol(pair).
		Interval(period).
		StartTime(start.UnixNano() / int64(time.Millisecond)).
		EndTime(end.UnixNano() / int64(time.Millisecond)).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candle := CandleFromKline(pair, *d)

		candles = append(candles, candle)
	}

	return candles, nil
}

func CandleFromKline(pair string, k binance.Kline) model.Candle {
	t := time.Unix(0, k.OpenTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = true
	candle.AmountTrade = k.TradeNum

	candle.Metadata = make(map[string]float64)
	return candle
}

func CandleFromWsKline(pair string, k binance.WsKline) model.Candle {
	t := time.Unix(0, k.StartTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.QuoteVolume, _ = strconv.ParseFloat(k.QuoteVolume, 64)
	candle.AmountTrade = k.TradeNum
	candle.ActiveBuyVolume, _ = strconv.ParseFloat(k.ActiveBuyVolume, 64)
	candle.ActiveBuyQuoteVolume, _ = strconv.ParseFloat(k.ActiveBuyQuoteVolume, 64)
	candle.Complete = k.IsFinal
	candle.AmountTrade = k.TradeNum
	candle.Metadata = make(map[string]float64)
	return candle
}

func (b *Binance) GetPairsToUSDT() ([]string, error) {
	infoPairs, err := b.client.NewExchangeInfoService().Do(b.ctx)
	if err != nil {
		return nil, err
	}
	allPairs := make([]string, 0)
	for _, value := range infoPairs.Symbols {
		if value.QuoteAsset == "USDT" && value.Status == "TRADING" { // Только пары с USDT
			allPairs = append(allPairs, value.BaseAsset+value.QuoteAsset)
		}
	}
	return allPairs, nil
}
