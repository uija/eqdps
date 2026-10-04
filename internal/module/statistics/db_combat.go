package statistics

import (
	"database/sql"
	"fmt"
	"time"
)

type SessionCombatStatistics struct {
	TotalDamage int64
	CombatTime  time.Duration
	SessionDPS  float64
	CombatDPS   float64
}

func sessionCombatStatistics(damage, seconds int64, duration time.Duration) SessionCombatStatistics {
	seconds = max(0, min(seconds, int64(duration/time.Second)))
	s := SessionCombatStatistics{TotalDamage: damage, CombatTime: time.Duration(seconds) * time.Second}
	if duration > 0 {
		s.SessionDPS = float64(damage) / duration.Seconds()
	}
	if seconds > 0 {
		s.CombatDPS = float64(damage) / float64(seconds)
	}
	return s
}

func GetSessionCombatStatistics(db *sql.DB, session SessionStatistics) (SessionCombatStatistics, error) {
	if db == nil || session.ZoneID < 1 || session.EnteredAt.IsZero() || session.Duration <= 0 {
		return SessionCombatStatistics{}, fmt.Errorf("get session combat: invalid database or session")
	}
	var damage, seconds int64
	err := db.QueryRow(`SELECT COALESCE(SUM(total_damage),0),COALESCE(SUM(combat_seconds),0)
		FROM zone_visits WHERE zone_id=? AND entered_at>=? AND entered_at<?`, session.ZoneID, session.EnteredAt, session.EnteredAt.Add(session.Duration)).Scan(&damage, &seconds)
	if err != nil {
		return SessionCombatStatistics{}, fmt.Errorf("get session combat: %w", err)
	}
	return sessionCombatStatistics(damage, seconds, session.Duration), nil
}
