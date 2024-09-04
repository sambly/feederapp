package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/sambly/feederapp/internal/model"
)

func TestTrade(_ *testing.T) {

	periods := []model.Periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 100},
	}

	timeStart := time.Date(2024, time.March, 25, 13, 33, 0, 0, time.Local)

	for _, period := range periods {
		nextTime := findNextMultipleTimeV2(timeStart, period.Duration)

		fmt.Println(nextTime)

	}

}

func findNextMultipleTimeV2(t time.Time, interval time.Duration) time.Time {
	// Находим ближайшее время, которое кратно интервалу, начиная с t
	remainder := t.Unix() % int64(interval.Seconds())
	if remainder != 0 {
		seconds := int64(interval.Seconds())
		// Добавляем оставшееся время до следующего кратного интервала
		t = t.Add(time.Duration(seconds-remainder) * time.Second)
		// Добавляем этот же период времени, так как нужно дождаться чтобы все данные успели сформироваться
	}
	t = t.Add(interval)
	return t
}
