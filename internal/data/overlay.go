package data

import (
	"time"
)

type TimerTracker struct {
	Started         time.Time
	CancelableUntil time.Time
	StopsAt         time.Time
	Event           EventConfig
}
