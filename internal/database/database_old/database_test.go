package databaseold

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	exModel "github.com/sambly/exchangeService/pkg/model"

	"github.com/sambly/exchangeService/pkg/exchange"
	"github.com/sambly/feederapp/internal/config"
)

// Проверка разности времени между двумя candle. Есть ли пропуске во времени
// TODO сделать также проверку не записан ли candle два раза
func TestSelectCandlesTable(t *testing.T) {

	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
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

	// НЕОБХОДИМЫЕ ДАННЫЕ
	tableName := "ch1m"

	pair := "FLOKIUSDT"
	period := "1m"
	limit := 500

	ctx := context.Background()
	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		t.Error(err)
	}

	config, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}

	db, err := DbConnection(config.NameDb, config.HostDb, config.PortDb, config.UserDb, config.PasswordDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	candles, err := binance.CandlesByLimit(ctx, pair, period, limit)
	if err != nil {
		t.Error(err)
	}

	notVS := func(cEx, cDb exModel.Candle) {
		fmt.Printf("----Excange------\n%v\n", cEx)
		fmt.Printf("----DB------\n%v\n", cDb)
	}

	allCandles := 0
	checkCandles := 0
	for _, candleExch := range candles {

		candleDb, err := SelectCandle(db, tableName, candleExch.Pair, candleExch.Time)
		if err != nil {
			t.Error(err)
		}
		allCandles++

		// VS
		if candleDb.Time.Equal(candleExch.Time) {

			checkCandles++

			if candleDb.Open != candleExch.Open {
				notVS(candleExch, candleDb)
				continue
			}
			if candleDb.Close != candleExch.Close {
				notVS(candleExch, candleDb)
				continue
			}
			if candleDb.Low != candleExch.Low {
				notVS(candleExch, candleDb)
				continue
			}
			if candleDb.High != candleExch.High {
				notVS(candleExch, candleDb)
				continue
			}
			if candleDb.Volume != candleExch.Volume {
				notVS(candleExch, candleDb)
				continue
			}
			if candleDb.AmountTrade != candleExch.AmountTrade {
				notVS(candleExch, candleDb)
				continue
			}
		}

	}
	fmt.Println("Запрошенные Candles: ", allCandles)
	fmt.Println("Проверенные Candles:", checkCandles)

}
