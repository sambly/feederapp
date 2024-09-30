package database

import (
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql" // MySQL driver initialization
	exModel "github.com/sambly/exchangeService/pkg/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	loggerGorm "gorm.io/gorm/logger"
)

var (
	candlesTables     string   = "candles_" // + periods
	candlesTablesList []string = []string{"1m", "3m", "15m", "1h", "4h", "1d"}
)

func dsn(dbname, hostname, port, username, password string) string {
	loc := `&loc=Local`
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&%s", username, password, hostname, port, dbname, loc)
}

func DbInit(dbname, hostname, port, username, password string) (*gorm.DB, error) {
	ds := dsn(dbname, hostname, port, username, password)

	// Настройка логирования
	logConfig := loggerGorm.Config{
		LogLevel: loggerGorm.Silent,
	}

	db, err := gorm.Open(mysql.Open(ds), &gorm.Config{
		Logger: loggerGorm.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logConfig,
		),
	})
	if err != nil {
		return db, err
	}
	for _, tableName := range candlesTablesList {
		if err := db.Table(fmt.Sprintf("%s%s", candlesTables, tableName)).AutoMigrate(&exModel.Candle{}); err != nil {
			return db, err
		}
	}
	return db, nil
}

func InsertCandle(db *gorm.DB, candle exModel.Candle, period string) error {
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candle)
	return result.Error
}

func InsertCandles(db *gorm.DB, candles []exModel.Candle, period string) error {
	if len(candles) == 0 {
		return nil
	}
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).Create(&candles)
	return result.Error
}
