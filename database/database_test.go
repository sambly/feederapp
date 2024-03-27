package database

import (
	"log"
	"testing"
	"time"
)

func TestSelectCandlesTable(t *testing.T) {

	db, err := DbConnection()
	if err != nil {
		t.Error(err)
	}
	defer db.Close()
	log.Println("HI2")
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
