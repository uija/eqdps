package statistics

import (
	"database/sql"
	"fmt"
)

func PrepareDb(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin database setup: %w", err)
	}
	defer tx.Rollback()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS zones (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL COLLATE NOCASE UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS zone_visits (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER NOT NULL,
			entered_at DATETIME NOT NULL,
			raw_zone_name TEXT NOT NULL,
			FOREIGN KEY (zone_id) REFERENCES zones(id)
		)`,
		`CREATE TABLE IF NOT EXISTS mobs (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER NOT NULL,
			name TEXT NOT NULL COLLATE NOCASE,
			FOREIGN KEY (zone_id) REFERENCES zones(id),
			UNIQUE (zone_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS kills (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER NOT NULL,
			mob_id INTEGER NOT NULL,
			killed_at DATETIME NOT NULL,
			kill_type TEXT NOT NULL CHECK (kill_type IN ('player', 'other', 'unknown')),
			FOREIGN KEY (zone_id) REFERENCES zones(id),
			FOREIGN KEY (mob_id) REFERENCES mobs(id)
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL COLLATE NOCASE UNIQUE,
			normalized_name TEXT NOT NULL COLLATE NOCASE
		)`,
		`CREATE TABLE IF NOT EXISTS loot (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER NOT NULL,
			mob_id INTEGER NOT NULL,
			kill_id INTEGER,
			item_id INTEGER NOT NULL,
			quantity INTEGER NOT NULL CHECK (quantity > 0),
			looted_at DATETIME NOT NULL,
			destination TEXT NOT NULL CHECK (destination IN (
				'inventory',
				'bank',
				'tradeskill_depot',
				'currency',
				'sold',
				'unknown'
			)),
			sale_value_copper INTEGER CHECK (sale_value_copper IS NULL OR sale_value_copper >= 0),
			FOREIGN KEY (zone_id) REFERENCES zones(id),
			FOREIGN KEY (mob_id) REFERENCES mobs(id),
			FOREIGN KEY (kill_id) REFERENCES kills(id),
			FOREIGN KEY (item_id) REFERENCES items(id)
		)`,
		`CREATE TABLE IF NOT EXISTS money (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER,
			received_at DATETIME NOT NULL,
			amount_copper INTEGER NOT NULL CHECK (amount_copper >= 0),
			source TEXT NOT NULL CHECK (source IN (
				'corpse',
				'loot_sale',
				'parcel',
				'trade',
				'merchant'
			)),
			FOREIGN KEY (zone_id) REFERENCES zones(id)
		)`,
		`CREATE TABLE IF NOT EXISTS experience (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER,
			received_at DATETIME NOT NULL,
			percent REAL CHECK (percent IS NULL OR percent >= 0),
			FOREIGN KEY (zone_id) REFERENCES zones(id)
		)`,
		`CREATE TABLE IF NOT EXISTS levels (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER,
			reached_at DATETIME NOT NULL,
			level INTEGER NOT NULL CHECK (level > 0),
			FOREIGN KEY (zone_id) REFERENCES zones(id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat (
			id INTEGER PRIMARY KEY,
			sent_at DATETIME NOT NULL,
			channel TEXT NOT NULL,
			direction TEXT NOT NULL CHECK (direction IN ('sent', 'received')),
			other_character TEXT COLLATE NOCASE
		)`,
		`CREATE TABLE IF NOT EXISTS parcels (
			id INTEGER PRIMARY KEY,
			direction TEXT NOT NULL CHECK (direction IN ('sent', 'received')),
			other_character TEXT NOT NULL COLLATE NOCASE,
			item_id INTEGER,
			quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
			money_copper INTEGER CHECK (money_copper IS NULL OR money_copper >= 0),
			sent_or_received_at DATETIME NOT NULL,
			collected_at DATETIME,
			CHECK (item_id IS NOT NULL OR money_copper IS NOT NULL),
			FOREIGN KEY (item_id) REFERENCES items(id)
		)`,
		`CREATE TABLE IF NOT EXISTS log_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			byte_offset INTEGER NOT NULL DEFAULT 0 CHECK (byte_offset >= 0)
		)`,
		`INSERT INTO log_state (id, byte_offset)
		VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING`,

		`CREATE INDEX IF NOT EXISTS zone_visits_zone_time_idx
		ON zone_visits (zone_id, entered_at)`,
		`CREATE INDEX IF NOT EXISTS kills_zone_time_idx
		ON kills (zone_id, killed_at)`,
		`CREATE INDEX IF NOT EXISTS kills_mob_time_idx
		ON kills (mob_id, killed_at)`,
		`CREATE INDEX IF NOT EXISTS items_normalized_name_idx
		ON items (normalized_name)`,
		`CREATE INDEX IF NOT EXISTS loot_zone_time_idx
		ON loot (zone_id, looted_at)`,
		`CREATE INDEX IF NOT EXISTS loot_mob_item_idx
		ON loot (mob_id, item_id)`,
		`CREATE INDEX IF NOT EXISTS loot_item_time_idx
		ON loot (item_id, looted_at)`,
		`CREATE INDEX IF NOT EXISTS loot_kill_idx
		ON loot (kill_id)`,
		`CREATE INDEX IF NOT EXISTS money_zone_time_idx
		ON money (zone_id, received_at)`,
		`CREATE INDEX IF NOT EXISTS money_source_idx
		ON money (source)`,
		`CREATE INDEX IF NOT EXISTS experience_zone_time_idx
		ON experience (zone_id, received_at)`,
		`CREATE INDEX IF NOT EXISTS levels_time_idx
		ON levels (reached_at)`,
		`CREATE INDEX IF NOT EXISTS chat_channel_direction_idx
		ON chat (channel, direction)`,
		`CREATE INDEX IF NOT EXISTS chat_character_idx
		ON chat (other_character)`,
		`CREATE INDEX IF NOT EXISTS parcels_direction_time_idx
		ON parcels (direction, sent_or_received_at)`,
		`CREATE INDEX IF NOT EXISTS parcels_character_idx
		ON parcels (other_character)`,
		`CREATE INDEX IF NOT EXISTS parcels_pending_idx
		ON parcels (direction, collected_at)
		WHERE direction = 'received' AND collected_at IS NULL`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("create statistics schema: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database setup: %w", err)
	}
	return nil
}
