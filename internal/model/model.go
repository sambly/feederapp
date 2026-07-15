package model

import (
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

// AppState хранит единственную строку состояния приложения — момент последней
// успешно записанной 1m-свечи. Используется при старте, чтобы отличить штатный
// рестарт от простоя всего приложения и решить, нужен ли бэкофилл пропущенных свечей.
type AppState struct {
	ID              uint      `gorm:"primarykey"`
	LastHeartbeatAt time.Time `gorm:"column:last_heartbeat_at"`
}
