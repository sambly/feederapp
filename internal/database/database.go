package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver initialization
	exModel "github.com/sambly/exchangeService/pkg/model"
	iModel "github.com/sambly/feederapp/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	loggerGorm "gorm.io/gorm/logger"
)

var (
	candlesTables     string   = "candles_" // + periods
	candlesTablesList []string = []string{"1m", "3m", "15m", "1h", "4h", "12h"}
)

// appStateID — в таблице app_state всегда ровно одна строка с этим ID.
const appStateID = 1

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
	if err := db.AutoMigrate(&iModel.AppState{}); err != nil {
		return db, err
	}
	return db, nil
}

// GetLastHeartbeat возвращает момент последней успешной записи свечей.
// ok=false означает, что приложение запускается впервые (строки состояния ещё нет).
func GetLastHeartbeat(db *gorm.DB) (time.Time, bool, error) {
	var state iModel.AppState
	err := db.First(&state, appStateID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return state.LastHeartbeatAt, true, nil
}

// UpdateHeartbeat обновляет (или создаёт) единственную строку состояния приложения.
// GREATEST не даёт heartbeat уехать назад: бэкофилл заканчивается позже, чем live-путь
// успевает записать более свежие значения, и не должен их затирать.
func UpdateHeartbeat(db *gorm.DB, t time.Time) error {
	state := iModel.AppState{ID: appStateID, LastHeartbeatAt: t}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_heartbeat_at": gorm.Expr("GREATEST(last_heartbeat_at, VALUES(last_heartbeat_at))"),
		}),
	}).Create(&state).Error
}

// candleUpsertColumns — что перезаписывать при конфликте по (pair, time). Нужно, чтобы
// повторная докачка бэкофилла (например, после краша посреди него) не падала на дублях
// и не плодила лишние строки, а просто обновляла уже существующую свечу.
var candleUpsertColumns = []string{
	"open", "close", "low", "high",
	"volume", "quote_volume",
	"amount_trade", "amount_trade_buy",
	"active_buy_volume", "active_buy_quote_volume",
}

func candleConflictClause() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "pair"}, {Name: "time"}},
		DoUpdates: clause.AssignmentColumns(candleUpsertColumns),
	}
}

func InsertCandle(db *gorm.DB, candle exModel.Candle, period string) error {
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Clauses(candleConflictClause()).
		Create(&candle)
	return result.Error
}

// insertBatchSize ограничивает размер одного INSERT: бэкофилл после долгого простоя
// может нести десятки тысяч строк, а одиночный гигантский стейтмент упирается в
// max_allowed_packet MySQL.
const insertBatchSize = 500

func InsertCandles(db *gorm.DB, candles []exModel.Candle, period string) error {
	if len(candles) == 0 {
		return nil
	}
	result := db.Table(fmt.Sprintf("%s%s", candlesTables, period)).
		Clauses(candleConflictClause()).
		CreateInBatches(&candles, insertBatchSize)
	return result.Error
}
