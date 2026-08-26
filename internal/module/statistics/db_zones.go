package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ZoneStatistics struct {
	Name           string
	Visits         int64
	TimeSpent      time.Duration
	MobsKilled     int64
	MotesDropped   int64
	MotesPerHour   float64
	MoteDropChance float64
}

func GetZoneStatistics(db *sql.DB) ([]ZoneStatistics, error) {
	if db == nil {
		return nil, errors.New("get zone statistics: database is nil")
	}

	var lastTimestamp sql.NullTime
	if err := db.QueryRow(`
		SELECT last_timestamp
		FROM log_state
		WHERE id = 1
	`).Scan(&lastTimestamp); err != nil {
		return nil, fmt.Errorf("get statistics last timestamp: %w", err)
	}

	rows, err := db.Query(`
		SELECT zones.name, zone_visits.entered_at
		FROM zone_visits
		JOIN zones ON zones.id = zone_visits.zone_id
		ORDER BY zone_visits.entered_at, zone_visits.id
	`)
	if err != nil {
		return nil, fmt.Errorf("get zone visits: %w", err)
	}
	defer rows.Close()

	type visit struct {
		zoneName string
		entered  time.Time
	}
	visits := make([]visit, 0)
	for rows.Next() {
		var value visit
		if err := rows.Scan(&value.zoneName, &value.entered); err != nil {
			return nil, fmt.Errorf("scan zone visit: %w", err)
		}
		visits = append(visits, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read zone visits: %w", err)
	}

	result := make([]ZoneStatistics, 0)
	indices := make(map[string]int)
	for index, value := range visits {
		key := strings.ToLower(value.zoneName)
		resultIndex, found := indices[key]
		if !found {
			resultIndex = len(result)
			indices[key] = resultIndex
			result = append(result, ZoneStatistics{Name: value.zoneName})
		}
		result[resultIndex].Visits++

		var leftAt time.Time
		if index+1 < len(visits) {
			leftAt = visits[index+1].entered
		} else if lastTimestamp.Valid {
			leftAt = lastTimestamp.Time
		}
		if leftAt.After(value.entered) {
			result[resultIndex].TimeSpent += leftAt.Sub(value.entered)
		}
	}

	rows, err = db.Query(`
		SELECT
			zones.name,
			(
				SELECT COUNT(*)
				FROM kills
				WHERE kills.zone_id = zones.id
					AND kills.kill_type <> 'unknown'
			),
			(
				SELECT COALESCE(SUM(loot.quantity), 0)
				FROM loot
				JOIN items ON items.id = loot.item_id
				WHERE loot.zone_id = zones.id
					AND items.name LIKE 'Mote of %'
			),
			(
				SELECT COUNT(DISTINCT loot.kill_id)
				FROM loot
				JOIN items ON items.id = loot.item_id
				WHERE loot.zone_id = zones.id
					AND loot.kill_id IS NOT NULL
					AND items.name LIKE 'Mote of %'
			)
		FROM zones
	`)
	if err != nil {
		return nil, fmt.Errorf("get zone kill and Mote statistics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var zoneName string
		var mobsKilled, motesDropped, killsWithMotes int64
		if err := rows.Scan(&zoneName, &mobsKilled, &motesDropped, &killsWithMotes); err != nil {
			return nil, fmt.Errorf("scan zone kill and Mote statistics: %w", err)
		}
		resultIndex, found := indices[strings.ToLower(zoneName)]
		if !found {
			continue
		}
		zone := &result[resultIndex]
		zone.MobsKilled = mobsKilled
		zone.MotesDropped = motesDropped
		if hours := zone.TimeSpent.Hours(); hours > 0 {
			zone.MotesPerHour = float64(motesDropped) / hours
		}
		if mobsKilled > 0 {
			zone.MoteDropChance = float64(killsWithMotes) / float64(mobsKilled) * 100
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read zone kill and Mote statistics: %w", err)
	}
	return result, nil
}
