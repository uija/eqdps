package statistics

import (
	"database/sql"
	"fmt"
)

type DamageRangeStatistics struct {
	Count  int64
	Damage int64
	Min    sql.NullInt64
	Max    sql.NullInt64
}

func (s DamageRangeStatistics) Average() float64 {
	if s.Count == 0 {
		return 0
	}
	return float64(s.Damage) / float64(s.Count)
}

type DamageStatistics struct {
	Name                                       string
	Category                                   string
	Normal                                     DamageRangeStatistics
	Critical                                   DamageRangeStatistics
	Miss, Dodge, Parry, Block, Riposte, Absorb int64
}

func (s DamageStatistics) Hits() int64 { return s.Normal.Count + s.Critical.Count }
func (s DamageStatistics) CritPercent() float64 {
	if s.Hits() == 0 {
		return 0
	}
	return 100 * float64(s.Critical.Count) / float64(s.Hits())
}

func GetDamageStatistics(db *sql.DB) ([]DamageStatistics, error) {
	if db == nil {
		return nil, fmt.Errorf("get damage statistics: database is nil")
	}
	rows, err := db.Query(`SELECT name, category,
		normal_count, normal_damage, normal_min, normal_max,
		crit_count, crit_damage, crit_min, crit_max,
		miss_count, dodge_count, parry_count, block_count, riposte_count, absorb_count
		FROM attack_statistics ORDER BY name COLLATE NOCASE, category`)
	if err != nil {
		return nil, fmt.Errorf("get damage statistics: %w", err)
	}
	defer rows.Close()
	result := make([]DamageStatistics, 0)
	for rows.Next() {
		var s DamageStatistics
		if err := rows.Scan(&s.Name, &s.Category,
			&s.Normal.Count, &s.Normal.Damage, &s.Normal.Min, &s.Normal.Max,
			&s.Critical.Count, &s.Critical.Damage, &s.Critical.Min, &s.Critical.Max,
			&s.Miss, &s.Dodge, &s.Parry, &s.Block, &s.Riposte, &s.Absorb); err != nil {
			return nil, fmt.Errorf("scan damage statistics: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read damage statistics: %w", err)
	}
	return result, nil
}
