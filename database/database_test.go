package database

import (
	"context"
	"fmt"
	"log"
	"main/exchange"
	"testing"
	"time"
)

// Проверка разности времени между двумя candle. Есть ли пропуске во времени
func TestSelectCandlesTable(t *testing.T) {

	db, err := DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	candles, err := SelectCandles(db, "ILVUSDT")
	if err != nil {
		t.Error(err)
	}

	for i := 1; i < len(candles); i++ {
		if candles[i].Time.Sub(candles[i-1].Time) > time.Minute {
			log.Println(candles[i].Time)
			log.Println(candles[i-1].Time)

		}
	}

}

// Проверка содерижт ли candle достоверные данные за период

func TestDataCandle(t *testing.T) {

	ctx := context.Background()
	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	// НЕОБХОДИМЫЕ ДАННЫЕ
	tableName := "ch1m"
	timeCandleSTR := "2024-03-29 11:40:00"
	pair := "FLOKIUSDT"

	timeCandle, err := time.Parse("2006-01-02 15:04:05", timeCandleSTR)
	if err != nil {
		t.Error(err)
	}
	candle, err := SelectCandle(db, tableName, pair, timeCandle)
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("%v", candle)
	fmt.Println()

	candles, err := binance.CandlesByLimit(ctx, pair, "1m", 10)
	if err != nil {
		t.Error(err)
	}

	for _, candle := range candles {
		fmt.Printf("%v", candle)
		fmt.Println()
	}
}
