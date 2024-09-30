package databaseold

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver initialization
	exModel "github.com/sambly/exchangeService/pkg/model"
)

func dsn(dbname, hostname, port, username, password string) string {
	loc := `&loc=Local`
	if dbname == "" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/", username, password, hostname, port)
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&%s", username, password, hostname, port, dbname, loc)
}

func DbConnection(dbname, hostname, port, username, password string) (*sql.DB, error) {
	// Подключаемся к серверу MySQL без указания базы данных
	db, err := sql.Open("mysql", dsn("", hostname, port, username, password))
	if err != nil {
		return nil, fmt.Errorf("ошибка %s при открытии соединения с MySQL", err)
	}
	defer db.Close()

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+dbname)
	if err != nil {
		return nil, fmt.Errorf("ошибка %s при создании базы данных", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("ошибка %s при получении количества строк", err)
	}

	// Подключаемся к вновь созданной базе данных
	db, err = sql.Open("mysql", dsn(dbname, hostname, port, username, password))
	if err != nil {
		return nil, fmt.Errorf("ошибка %s при открытии соединения с базой данных", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Minute * 5)

	ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка %s при пинге базы данных", err)
	}

	return db, nil
}

func CreateCandlesTable(db *sql.DB) error {
	query := `CREATE TABLE IF NOT EXISTS candles(
		Id int primary key auto_increment,
		Time datetime,
		Pair VARCHAR(20),
		Open DOUBLE,
		Close DOUBLE,
		Low DOUBLE,
		High DOUBLE,
		Volume DOUBLE,
		QuoteVolume DOUBLE,
		AmountTrade INT,
		AmountTradeBuy INT,
		ActiveBuyVolume DOUBLE,
		ActiveBuyQuoteVolume DOUBLE
		)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating candles table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}
	return nil
}

func CreateTableName(db *sql.DB, tableName string) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
        Id int primary key auto_increment,
        Time datetime,
        Pair VARCHAR(20),
        Open DOUBLE,
        Close DOUBLE,
        Low DOUBLE,
        High DOUBLE,
        Volume DOUBLE,
        QuoteVolume DOUBLE,
        AmountTrade INT,
        AmountTradeBuy INT,
        ActiveBuyVolume DOUBLE,
        ActiveBuyQuoteVolume DOUBLE
    )`, "candles"+tableName)

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating %s table", err, "candles"+tableName)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}

	if rowsAffected > 0 {
		indexQuery := fmt.Sprintf(`CREATE INDEX idx_pair ON %s (Pair)`, "candles"+tableName)
		_, err = db.ExecContext(ctx, indexQuery)
		if err != nil {
			return fmt.Errorf("error %s when creating index on %s table", err, "candles"+tableName)
		}
	}

	return nil
}

func InsertCandlesTable(db *sql.DB, candle exModel.Candle) error {
	query := "INSERT INTO candles (Time,Pair,Open,Close,Low,High,Volume,QuoteVolume,AmountTrade,AmountTradeBuy,ActiveBuyVolume,ActiveBuyQuoteVolume) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmtLicense, err := db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmtLicense.Close()

	res, err := stmtLicense.ExecContext(
		ctx,
		candle.Time,
		candle.Pair,
		candle.Open,
		candle.Close,
		candle.Low,
		candle.High,
		candle.Volume,
		candle.QuoteVolume,
		candle.AmountTrade,
		candle.AmountTradeBuy,
		candle.ActiveBuyVolume,
		candle.ActiveBuyQuoteVolume,
	)
	if err != nil {
		return fmt.Errorf("error %s when inserting row into candles table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when finding rows affected", err)
	}

	return nil
}

func InsertCandlesTableName(db *sql.DB, tableName string, candles []exModel.Candle) error {
	if len(candles) == 0 {
		return nil // Нет свечей для вставки, возвращаем nil
	}

	query := fmt.Sprintf("INSERT INTO %s (Time,Pair,Open,Close,Low,High,Volume,QuoteVolume,AmountTrade,AmountTradeBuy,ActiveBuyVolume,ActiveBuyQuoteVolume) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", "candles"+tableName)
	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()

	// Начинаем транзакцию
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error %s when beginning transaction", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	stmtLicense, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmtLicense.Close()

	for _, candle := range candles {
		_, err := stmtLicense.ExecContext(
			ctx,
			candle.Time,
			candle.Pair,
			candle.Open,
			candle.Close,
			candle.Low,
			candle.High,
			candle.Volume,
			candle.QuoteVolume,
			candle.AmountTrade,
			candle.AmountTradeBuy,
			candle.ActiveBuyVolume,
			candle.ActiveBuyQuoteVolume,
		)
		if err != nil {
			return fmt.Errorf("error %s when inserting row into %s table", err, "candles"+tableName)
		}
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error %s when committing transaction", err)
	}

	return nil
}

func SelectCandles(db *sql.DB, pair string) ([]exModel.Candle, error) {
	candles := []exModel.Candle{}

	query := "SELECT Time, Pair, Close, Volume FROM candlesch1m WHERE Pair = ?;"

	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return candles, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, pair)
	if err != nil {
		return candles, err
	}
	defer rows.Close()

	for rows.Next() {
		var candle exModel.Candle
		if err := rows.Scan(&candle.Time, &candle.Pair, &candle.Close, &candle.Volume); err != nil {
			return candles, err
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return candles, err
	}

	return candles, nil
}

func SelectCandle(db *sql.DB, tableName, pair string, timeRounding time.Time) (exModel.Candle, error) {
	candle := exModel.Candle{}

	query := fmt.Sprintf("select Time,Pair,Open,Close,Low,High,Volume,QuoteVolume,AmountTrade,AmountTradeBuy,ActiveBuyVolume,ActiveBuyQuoteVolume from %s WHERE Pair = ? and Time = ?;", "candles"+tableName)
	ctx, cancelfunc := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelfunc()
	stmt, err := db.PrepareContext(ctx, query)

	if err != nil {
		return candle, fmt.Errorf("error %s when preparing SQL statement", err)
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, pair, timeRounding.Format("2006-01-02 15:04:05")).Scan(
		&candle.Time,
		&candle.Pair,
		&candle.Open,
		&candle.Close,
		&candle.Low,
		&candle.High,
		&candle.Volume,
		&candle.QuoteVolume,
		&candle.AmountTrade,
		&candle.AmountTradeBuy,
		&candle.ActiveBuyVolume,
		&candle.ActiveBuyQuoteVolume,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return candle, nil
		}
		return candle, err
	}

	return candle, nil
}
