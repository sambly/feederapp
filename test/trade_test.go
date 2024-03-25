package test

import (
	"main/model"
	"testing"
	"time"
)

func TestTrade(t *testing.T) {

	periods := []model.Periods{
		{Name: "ch1m", Duration: time.Second * 60},
		{Name: "ch3m", Duration: time.Minute * 3},
		{Name: "ch15m", Duration: time.Minute * 15},
		{Name: "ch1h", Duration: time.Hour},
		{Name: "ch4h", Duration: time.Hour * 4},
	}

	//timeStart := time.Now().Truncate(time.Minute)
	timeStart := time.Date(2024, time.March, 25, 13, 31, 0, 0, time.Local)

	for _, period := range periods {
		nextTime := findNextMultipleTime(timeStart, period.Duration)

		t.Log(nextTime) // Заменим fmt.Println() на t.Log()

	}

}

func findNextMultipleTime(t time.Time, interval time.Duration) time.Time {
	for {
		// TODO здесь может сделать увеличение на 1 минуту
		t = t.Add(1 * time.Second) // Увеличиваем на 1 секунду для предотвращения зацикливания на текущем времени
		if t.Unix()%int64(interval.Seconds()) == 0 {
			break
		}
	}
	return t
}
