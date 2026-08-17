package statistics

import (
	"database/sql"
	"fmt"
)

func PrepareDb(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin database setup: %w", err)
	}
	defer tx.Rollback()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS zones (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			visits INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS mobs (
			id INTEGER PRIMARY KEY,
			zone_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			kills INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (zone_id) REFERENCES zones(id),
			UNIQUE (zone_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY,
			mob_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			looted INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (mob_id) REFERENCES mobs(id),
			UNIQUE (mob_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS log_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			byte_offset INTEGER NOT NULL DEFAULT 0 CHECK (byte_offset >= 0)
		)`,
		`INSERT INTO log_state (id, byte_offset)
		VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("create statistics tables: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database setup: %w", err)
	}
	return nil
}

func GetLogOffset(db *sql.DB) (int64, error) {
	var offset int64
	if err := db.QueryRow(`SELECT byte_offset FROM log_state WHERE id = 1`).Scan(&offset); err != nil {
		return 0, fmt.Errorf("get log offset: %w", err)
	}
	return offset, nil
}

func SetLogOffset(db *sql.DB, offset int64) error {
	if offset < 0 {
		return fmt.Errorf("set log offset: offset must not be negative")
	}
	_, err := db.Exec(`
		INSERT INTO log_state (id, byte_offset)
		VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET byte_offset = excluded.byte_offset
	`, offset)
	if err != nil {
		return fmt.Errorf("set log offset to %d: %w", offset, err)
	}
	return nil
}

func GetOrCreateZone(db *sql.DB, name string) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO zones (name)
		VALUES (?)
		ON CONFLICT (name) DO UPDATE SET name = excluded.name
		RETURNING id
	`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get or create zone %q: %w", name, err)
	}
	return id, nil
}

func GetOrCreateMob(db *sql.DB, zoneID int64, name string) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO mobs (zone_id, name)
		VALUES (?, ?)
		ON CONFLICT (zone_id, name) DO UPDATE SET name = excluded.name
		RETURNING id
	`, zoneID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get or create mob %q in zone %d: %w", name, zoneID, err)
	}
	return id, nil
}

func GetOrCreateItem(db *sql.DB, mobID int64, name string) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO items (mob_id, name)
		VALUES (?, ?)
		ON CONFLICT (mob_id, name) DO UPDATE SET name = excluded.name
		RETURNING id
	`, mobID, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get or create item %q for mob %d: %w", name, mobID, err)
	}
	return id, nil
}

func IncrementZoneVisits(db *sql.DB, zoneID int64) error {
	return incrementCounter(db, "zones", "visits", zoneID, 1)
}

func IncrementMobKills(db *sql.DB, mobID int64) error {
	return incrementCounter(db, "mobs", "kills", mobID, 1)
}

func IncrementItemLooted(db *sql.DB, itemID int64, quantity int) error {
	if quantity < 1 {
		return fmt.Errorf("increment item %d: quantity must be positive", itemID)
	}
	return incrementCounter(db, "items", "looted", itemID, quantity)
}

func incrementCounter(db *sql.DB, table, column string, id int64, amount int) error {
	query := fmt.Sprintf("UPDATE %s SET %s = %s + ? WHERE id = ?", table, column, column)
	result, err := db.Exec(query, amount, id)
	if err != nil {
		return fmt.Errorf("increment %s.%s for id %d: %w", table, column, id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check increment of %s.%s for id %d: %w", table, column, id, err)
	}
	if rows == 0 {
		return fmt.Errorf("increment %s.%s: id %d does not exist", table, column, id)
	}
	return nil
}
