package xp

import (
	"math"
	"time"
)

const PauseThreshold = time.Minute

type Snapshot struct {
	Percent        float64
	LevelPercent   float64
	ProgressKnown  bool
	Gains          int
	ActiveDuration time.Duration
	PercentPerHour float64
}

func (s Snapshot) TimeToLevel() (time.Duration, bool) {
	if s.PercentPerHour <= 0 {
		return 0, false
	}
	remaining := 100 - s.LevelPercent
	if remaining < 0 {
		remaining = 0
	}
	minutes := math.Ceil(remaining / s.PercentPerHour * 60)
	return time.Duration(minutes) * time.Minute, true
}

type Session struct {
	percent       float64
	levelPercent  float64
	progressKnown bool
	levelUpTime   time.Time
	gains         int
	started       time.Time
	lastCombat    time.Time
	activeBetween time.Duration
	latestLog     time.Time
	observedAt    time.Time
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Observe(logTime, observedAt time.Time) {
	if logTime.IsZero() || (!s.latestLog.IsZero() && logTime.Before(s.latestLog)) {
		return
	}
	s.latestLog = logTime
	s.observedAt = observedAt
}

func (s *Session) AddCombat(timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	if s.started.IsZero() {
		s.started = timestamp
		s.lastCombat = timestamp
		return
	}
	if !timestamp.After(s.lastCombat) {
		return
	}
	s.activeBetween += cappedInterval(timestamp.Sub(s.lastCombat))
	s.lastCombat = timestamp
}

func (s *Session) AddGain(timestamp time.Time, percent float64) {
	if timestamp.IsZero() || percent <= 0 {
		return
	}
	if s.started.IsZero() {
		s.started = timestamp
		s.lastCombat = timestamp
	}
	s.percent += percent
	if !s.levelUpTime.IsZero() && timestamp.Equal(s.levelUpTime) {
		s.levelUpTime = time.Time{}
	} else {
		s.levelUpTime = time.Time{}
		s.levelPercent += percent
	}
	s.gains++
}

func (s *Session) AddLevelUp(timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	s.levelPercent = 0
	s.progressKnown = true
	s.levelUpTime = timestamp
}

func (s *Session) SnapshotAtLatestLog() Snapshot {
	return s.snapshot(s.latestLog)
}

func (s *Session) SnapshotLive(now time.Time) Snapshot {
	latest := s.latestLog
	if !latest.IsZero() && !s.observedAt.IsZero() && now.After(s.observedAt) {
		latest = latest.Add(now.Sub(s.observedAt))
	}
	return s.snapshot(latest)
}

func (s *Session) snapshot(end time.Time) Snapshot {
	result := Snapshot{
		Percent:       s.percent,
		LevelPercent:  s.levelPercent,
		ProgressKnown: s.progressKnown,
		Gains:         s.gains,
	}
	if s.gains == 0 || s.started.IsZero() || s.lastCombat.IsZero() {
		return result
	}

	result.ActiveDuration = s.activeBetween
	if end.After(s.lastCombat) {
		result.ActiveDuration += cappedInterval(end.Sub(s.lastCombat))
	}
	if hours := result.ActiveDuration.Hours(); hours > 0 {
		result.PercentPerHour = result.Percent / hours
	}
	return result
}

func cappedInterval(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}
	if duration > PauseThreshold {
		return PauseThreshold
	}
	return duration
}
