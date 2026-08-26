package statistics

import (
	"database/sql"
	"errors"
	"fmt"
)

type OverviewStatistics struct {
	ZonesVisited     int64
	MobsKilled       int64
	ItemsLooted      int64
	MoneyCollected   int64
	ExperienceGained float64
	LevelsGained     int64
	MotesCollected   int64
	ChatMessagesSent int64
}

func GetOverviewStatistics(db *sql.DB) (OverviewStatistics, error) {
	if db == nil {
		return OverviewStatistics{}, errors.New("get overview statistics: database is nil")
	}

	var result OverviewStatistics
	err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM zones),
			(SELECT COUNT(*) FROM kills WHERE kill_type <> 'unknown'),
			(SELECT COALESCE(SUM(quantity), 0) FROM loot),
			(SELECT COALESCE(SUM(amount_copper), 0) FROM money),
			(SELECT COALESCE(SUM(percent), 0) FROM experience),
			(SELECT COUNT(*) FROM levels),
			(
				SELECT COALESCE(SUM(loot.quantity), 0)
				FROM loot
				JOIN items ON items.id = loot.item_id
				WHERE items.name LIKE 'Mote of %'
			),
			(SELECT COUNT(*) FROM chat WHERE direction = 'sent')
	`).Scan(
		&result.ZonesVisited,
		&result.MobsKilled,
		&result.ItemsLooted,
		&result.MoneyCollected,
		&result.ExperienceGained,
		&result.LevelsGained,
		&result.MotesCollected,
		&result.ChatMessagesSent,
	)
	if err != nil {
		return OverviewStatistics{}, fmt.Errorf("get overview statistics: %w", err)
	}
	return result, nil
}
