package database

import (
	"context"
	"database/sql"
	"fmt"
	"main/model"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	username = "root"
	password = "q1w2e3"
	hostname = "127.0.0.1:3306"
	dbname   = "datafeeder"
)

func dsn(dbName string) string {
	loc := `loc=Europe%2FMoscow`
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&%s", username, password, hostname, dbName, loc)
}

func DbConnection() (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn(""))
	if err != nil {
		return nil, fmt.Errorf("error %s when opening DB", err)
	}
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+dbname)
	if err != nil {
		return nil, fmt.Errorf("error %s when creating DB", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error %s when fetching rows", err)
	}
	db.Close()

	db, err = sql.Open("mysql", dsn(dbname))
	if err != nil {
		return nil, fmt.Errorf("error %s when opening DB", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Minute * 5)

	ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("errors %s pinging DB", err)
	}

	return db, nil
}

func CreateFeederTables(db *sql.DB) error {

	query := `CREATE TABLE IF NOT EXISTS candles(
		Id int primary key auto_increment,
		Time datetime,
		Pair text,
		Open text,
		Close text,
		High text,
		Low text,
		Volume text
		)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error %s when creating license table", err)
	}
	_, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error %s when getting rows affected", err)
	}
	return nil
}

func InsertCandlesTables(db *sql.DB, candle model.Candle) error {

	query := "INSERT INTO candles (Time,Pair,Open,Close,High,Low,Volume) VALUES (?,?,?,?,?,?,?)"
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
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
		candle.High,
		candle.Low,
		candle.Volume,
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
