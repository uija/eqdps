package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type DungeonCrawlMoteSource struct {
	Name     string
	Quantity int64
}

type DungeonCrawlMote struct {
	Item     string
	Quantity int64
	Sources  []DungeonCrawlMoteSource
}

type DungeonCrawlReward struct {
	Item     string
	Quantity int64
}

type DungeonCrawlDetails struct {
	Kills            int64
	ExperienceGained float64 // Summed XP percentages, as in the session overview.
	Motes            int64
	Motes5Plus       int64 // Number of Major Potential and higher-tier Motes.
	Money            int64 // Copper, including automatic loot sales.
	MoteDetails      []DungeonCrawlMote
	ChestRewards     []DungeonCrawlReward
}

// GetDungeonCrawlDetails returns the selected session's totals, Motes grouped
// by item and source, and Reward Chest loot. Chest rewards retain their original
// upgrade suffix and include stored, sold, and merged drops. The time range is
// inclusive at entry and exclusive at session end, matching GetSessionDetails.
func GetDungeonCrawlDetails(db *sql.DB, session SessionStatistics) (DungeonCrawlDetails, error) {
	if db == nil {
		return DungeonCrawlDetails{}, errors.New("get dungeon crawl details: database is nil")
	}
	if session.ZoneID < 1 || session.EnteredAt.IsZero() || session.Duration <= 0 {
		return DungeonCrawlDetails{}, errors.New("get dungeon crawl details: invalid session")
	}
	arguments := []any{session.ZoneID, session.EnteredAt, session.EnteredAt.Add(session.Duration)}
	result := DungeonCrawlDetails{
		MoteDetails:  make([]DungeonCrawlMote, 0),
		ChestRewards: make([]DungeonCrawlReward, 0),
	}
	if err := db.QueryRow(`
		WITH bounds AS (SELECT ? AS zone_id, ? AS start_at, ? AS end_at)
		SELECT
			(SELECT COUNT(*) FROM kills, bounds
			 WHERE kills.zone_id = bounds.zone_id
			   AND killed_at >= start_at AND killed_at < end_at
			   AND kill_type <> 'unknown'),
			(SELECT COALESCE(SUM(percent), 0) FROM experience, bounds
			 WHERE experience.zone_id = bounds.zone_id
			   AND received_at >= start_at AND received_at < end_at),
			(SELECT COALESCE(SUM(amount_copper), 0) FROM money, bounds
			 WHERE money.zone_id = bounds.zone_id
			   AND received_at >= start_at AND received_at < end_at)
	`, arguments...).Scan(&result.Kills, &result.ExperienceGained, &result.Money); err != nil {
		return DungeonCrawlDetails{}, fmt.Errorf("get dungeon crawl totals: %w", err)
	}

	rows, err := db.Query(`
		SELECT items.name, mobs.name, SUM(loot.quantity)
		FROM loot
		JOIN items ON items.id = loot.item_id
		JOIN mobs ON mobs.id = loot.mob_id
		WHERE loot.zone_id = ? AND looted_at >= ? AND looted_at < ?
			AND items.name LIKE 'Mote of %'
		GROUP BY items.id, items.name, mobs.id, mobs.name
		ORDER BY items.name COLLATE NOCASE, items.id, mobs.name COLLATE NOCASE
	`, arguments...)
	if err != nil {
		return DungeonCrawlDetails{}, fmt.Errorf("get dungeon crawl Motes: %w", err)
	}
	for rows.Next() {
		var item string
		var source DungeonCrawlMoteSource
		if err := rows.Scan(&item, &source.Name, &source.Quantity); err != nil {
			rows.Close()
			return DungeonCrawlDetails{}, fmt.Errorf("scan dungeon crawl Mote: %w", err)
		}
		if len(result.MoteDetails) == 0 || result.MoteDetails[len(result.MoteDetails)-1].Item != item {
			result.MoteDetails = append(result.MoteDetails, DungeonCrawlMote{Item: item})
		}
		mote := &result.MoteDetails[len(result.MoteDetails)-1]
		mote.Quantity += source.Quantity
		mote.Sources = append(mote.Sources, source)
		result.Motes += source.Quantity
		switch strings.ToLower(item) {
		case "mote of major potential", "mote of greater potential", "mote of superior potential",
			"mote of grand potential", "mote of ascendant potential", "mote of infinite potential":
			result.Motes5Plus += source.Quantity
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return DungeonCrawlDetails{}, fmt.Errorf("read dungeon crawl Motes: %w", err)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT loot.raw_item_name, SUM(loot.quantity)
		FROM loot
		JOIN mobs ON mobs.id = loot.mob_id
		WHERE loot.zone_id = ? AND looted_at >= ? AND looted_at < ?
			AND mobs.name = 'Reward Chest' COLLATE NOCASE
		GROUP BY loot.raw_item_name
		ORDER BY loot.raw_item_name COLLATE NOCASE
	`, arguments...)
	if err != nil {
		return DungeonCrawlDetails{}, fmt.Errorf("get dungeon crawl chest rewards: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var reward DungeonCrawlReward
		if err := rows.Scan(&reward.Item, &reward.Quantity); err != nil {
			return DungeonCrawlDetails{}, fmt.Errorf("scan dungeon crawl chest reward: %w", err)
		}
		result.ChestRewards = append(result.ChestRewards, reward)
	}
	if err := rows.Err(); err != nil {
		return DungeonCrawlDetails{}, fmt.Errorf("read dungeon crawl chest rewards: %w", err)
	}
	return result, nil
}
