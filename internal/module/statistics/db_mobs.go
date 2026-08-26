package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type MobStatistics struct {
	Name           string
	KilledByPlayer int64
	KilledPlayer   int64
	MoneyLooted    int64
	ItemsLooted    int64
	DifferentItems int64
}

type MobZoneStatistics struct {
	Name  string
	Kills int64
}

type MobItemStatistics struct {
	Name       string
	Quantity   int64
	DropChance float64
}

type MobDetails struct {
	Zones []MobZoneStatistics
	Items []MobItemStatistics
}

func GetMobStatistics(db *sql.DB) ([]MobStatistics, error) {
	if db == nil {
		return nil, errors.New("get mob statistics: database is nil")
	}
	rows, err := db.Query(`
		WITH mob_names AS (
			SELECT MIN(name) AS name
			FROM mobs
			GROUP BY name COLLATE NOCASE
		)
		SELECT
			mob_names.name,
			(
				SELECT COUNT(*)
				FROM kills
				JOIN mobs ON mobs.id = kills.mob_id
				WHERE mobs.name = mob_names.name COLLATE NOCASE
					AND kills.kill_type <> 'unknown'
			),
			(
				SELECT COUNT(*)
				FROM player_deaths
				JOIN mobs ON mobs.id = player_deaths.mob_id
				WHERE mobs.name = mob_names.name COLLATE NOCASE
			),
			(
				SELECT COALESCE(SUM(money.amount_copper), 0)
				FROM money
				JOIN kills ON kills.id = money.kill_id
				JOIN mobs ON mobs.id = kills.mob_id
				WHERE mobs.name = mob_names.name COLLATE NOCASE
					AND money.source = 'corpse'
			),
			(
				SELECT COALESCE(SUM(loot.quantity), 0)
				FROM loot
				JOIN mobs ON mobs.id = loot.mob_id
				WHERE mobs.name = mob_names.name COLLATE NOCASE
			),
			(
				SELECT COUNT(DISTINCT loot.item_id)
				FROM loot
				JOIN mobs ON mobs.id = loot.mob_id
				WHERE mobs.name = mob_names.name COLLATE NOCASE
			)
		FROM mob_names
		ORDER BY mob_names.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("get mob statistics: %w", err)
	}
	defer rows.Close()

	result := make([]MobStatistics, 0)
	for rows.Next() {
		var value MobStatistics
		if err := rows.Scan(
			&value.Name,
			&value.KilledByPlayer,
			&value.KilledPlayer,
			&value.MoneyLooted,
			&value.ItemsLooted,
			&value.DifferentItems,
		); err != nil {
			return nil, fmt.Errorf("scan mob statistics: %w", err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read mob statistics: %w", err)
	}
	return result, nil
}

func GetMobDetails(db *sql.DB, mobName string) (MobDetails, error) {
	result := MobDetails{
		Zones: make([]MobZoneStatistics, 0),
		Items: make([]MobItemStatistics, 0),
	}
	if db == nil {
		return result, errors.New("get mob details: database is nil")
	}
	mobName = strings.TrimSpace(mobName)
	if mobName == "" {
		return result, errors.New("get mob details: mob name is empty")
	}

	rows, err := db.Query(`
		SELECT zones.name, COUNT(*)
		FROM kills
		JOIN mobs ON mobs.id = kills.mob_id
		JOIN zones ON zones.id = kills.zone_id
		WHERE mobs.name = ? COLLATE NOCASE
			AND kills.kill_type <> 'unknown'
		GROUP BY zones.id, zones.name
		ORDER BY zones.name COLLATE NOCASE
	`, mobName)
	if err != nil {
		return result, fmt.Errorf("get zones for mob %q: %w", mobName, err)
	}
	for rows.Next() {
		var value MobZoneStatistics
		if err := rows.Scan(&value.Name, &value.Kills); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan zone for mob %q: %w", mobName, err)
		}
		result.Zones = append(result.Zones, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("read zones for mob %q: %w", mobName, err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close zones for mob %q: %w", mobName, err)
	}

	var kills int64
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM kills
		JOIN mobs ON mobs.id = kills.mob_id
		WHERE mobs.name = ? COLLATE NOCASE
			AND kills.kill_type <> 'unknown'
	`, mobName).Scan(&kills); err != nil {
		return result, fmt.Errorf("count kills for mob %q: %w", mobName, err)
	}

	rows, err = db.Query(`
		SELECT
			items.name,
			SUM(loot.quantity),
			COUNT(DISTINCT loot.kill_id)
		FROM loot
		JOIN mobs ON mobs.id = loot.mob_id
		JOIN items ON items.id = loot.item_id
		WHERE mobs.name = ? COLLATE NOCASE
		GROUP BY items.id, items.name
		ORDER BY items.name COLLATE NOCASE
	`, mobName)
	if err != nil {
		return result, fmt.Errorf("get items for mob %q: %w", mobName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var value MobItemStatistics
		var killsWithItem int64
		if err := rows.Scan(&value.Name, &value.Quantity, &killsWithItem); err != nil {
			return result, fmt.Errorf("scan item for mob %q: %w", mobName, err)
		}
		if kills > 0 {
			value.DropChance = float64(killsWithItem) / float64(kills) * 100
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("read items for mob %q: %w", mobName, err)
	}
	return result, nil
}
