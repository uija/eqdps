package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ItemStatistics struct {
	Name  string
	Drops int64
}

type ItemDropStatistics struct {
	Mob        string
	Zone       string
	Kills      int64
	Drops      int64
	DropChance float64
}

type ItemDetails struct {
	Drops []ItemDropStatistics
}

func GetItemStatistics(db *sql.DB) ([]ItemStatistics, error) {
	if db == nil {
		return nil, errors.New("get item statistics: database is nil")
	}
	rows, err := db.Query(`
		SELECT items.name, SUM(loot.quantity)
		FROM loot
		JOIN items ON items.id = loot.item_id
		GROUP BY items.id, items.name
		ORDER BY items.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("get item statistics: %w", err)
	}
	defer rows.Close()

	result := make([]ItemStatistics, 0)
	for rows.Next() {
		var value ItemStatistics
		if err := rows.Scan(&value.Name, &value.Drops); err != nil {
			return nil, fmt.Errorf("scan item statistics: %w", err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read item statistics: %w", err)
	}
	return result, nil
}

func GetItemDetails(db *sql.DB, itemName string) (ItemDetails, error) {
	result := ItemDetails{Drops: make([]ItemDropStatistics, 0)}
	if db == nil {
		return result, errors.New("get item details: database is nil")
	}
	itemName = strings.TrimSpace(itemName)
	if itemName == "" {
		return result, errors.New("get item details: item name is empty")
	}

	rows, err := db.Query(`
		SELECT
			mobs.name,
			zones.name,
			(
				SELECT COUNT(*)
				FROM kills
				WHERE kills.mob_id = mobs.id
					AND kills.zone_id = zones.id
					AND kills.kill_type <> 'unknown'
			),
			SUM(loot.quantity),
			COUNT(DISTINCT loot.kill_id)
		FROM loot
		JOIN items ON items.id = loot.item_id
		JOIN mobs ON mobs.id = loot.mob_id
		JOIN zones ON zones.id = loot.zone_id
		WHERE items.name = ? COLLATE NOCASE
		GROUP BY mobs.id, mobs.name, zones.id, zones.name
		ORDER BY mobs.name COLLATE NOCASE, zones.name COLLATE NOCASE
	`, itemName)
	if err != nil {
		return result, fmt.Errorf("get item details for %q: %w", itemName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var value ItemDropStatistics
		var killsWithItem int64
		if err := rows.Scan(
			&value.Mob,
			&value.Zone,
			&value.Kills,
			&value.Drops,
			&killsWithItem,
		); err != nil {
			return result, fmt.Errorf("scan item details for %q: %w", itemName, err)
		}
		if value.Kills > 0 {
			value.DropChance = float64(killsWithItem) / float64(value.Kills) * 100
		}
		result.Drops = append(result.Drops, value)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("read item details for %q: %w", itemName, err)
	}
	sort.Slice(result.Drops, func(i, j int) bool {
		if result.Drops[i].Zone != result.Drops[j].Zone {
			return result.Drops[i].Zone < result.Drops[j].Zone
		}
		if result.Drops[i].DropChance != result.Drops[j].DropChance {
			return result.Drops[i].DropChance > result.Drops[j].DropChance
		}
		return result.Drops[i].Mob < result.Drops[j].Mob
	})
	return result, nil
}
