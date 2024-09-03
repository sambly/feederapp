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
