package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (i *Import) GetOrCreateZone(name string) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("get or create statistics zone: name is empty")
	}
	var id int64
	if err := tx.QueryRow(`
		INSERT INTO zones (name)
		VALUES (?)
		ON CONFLICT (name) DO UPDATE SET name = zones.name
		RETURNING id
	`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("get or create statistics zone %q: %w", name, err)
	}
	return id, nil
}

func (i *Import) InsertZoneVisit(zoneID int64, rawName string, enteredAt time.Time) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	rawName = strings.TrimSpace(rawName)
	if zoneID < 1 {
		return 0, fmt.Errorf("insert statistics zone visit: zone ID %d is invalid", zoneID)
	}
	if rawName == "" {
		return 0, errors.New("insert statistics zone visit: raw zone name is empty")
	}
	if enteredAt.IsZero() {
		return 0, errors.New("insert statistics zone visit: timestamp is zero")
	}
	result, err := tx.Exec(`
		INSERT INTO zone_visits (zone_id, entered_at, raw_zone_name)
		VALUES (?, ?, ?)
	`, zoneID, enteredAt, rawName)
	if err != nil {
		return 0, fmt.Errorf("insert statistics zone visit: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics zone visit ID: %w", err)
	}
	return id, nil
}

func (i *Import) GetOrCreateMob(zoneID int64, name string) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if zoneID < 1 {
		return 0, fmt.Errorf("get or create statistics mob: zone ID %d is invalid", zoneID)
	}
	if name == "" {
		return 0, errors.New("get or create statistics mob: name is empty")
	}
	var id int64
	if err := tx.QueryRow(`
		INSERT INTO mobs (zone_id, name)
		VALUES (?, ?)
		ON CONFLICT (zone_id, name) DO UPDATE SET name = mobs.name
		RETURNING id
	`, zoneID, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("get or create statistics mob %q: %w", name, err)
	}
	return id, nil
}

func (i *Import) InsertKill(zoneID, mobID int64, killedAt time.Time, killType KillType) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if zoneID < 1 {
		return 0, fmt.Errorf("insert statistics kill: zone ID %d is invalid", zoneID)
	}
	if mobID < 1 {
		return 0, fmt.Errorf("insert statistics kill: mob ID %d is invalid", mobID)
	}
	if killedAt.IsZero() {
		return 0, errors.New("insert statistics kill: timestamp is zero")
	}
	if !killType.valid() {
		return 0, fmt.Errorf("insert statistics kill: type %q is invalid", killType)
	}
	result, err := tx.Exec(`
		INSERT INTO kills (zone_id, mob_id, killed_at, kill_type)
		VALUES (?, ?, ?, ?)
	`, zoneID, mobID, killedAt, killType)
	if err != nil {
		return 0, fmt.Errorf("insert statistics kill: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics kill ID: %w", err)
	}
	return id, nil
}

func (i *Import) FindRecentKill(zoneID, mobID int64, at time.Time, maximumAge time.Duration) (Kill, bool, error) {
	tx, err := i.activeTx()
	if err != nil {
		return Kill{}, false, err
	}
	if zoneID < 1 || mobID < 1 {
		return Kill{}, false, errors.New("find recent statistics kill: invalid zone or mob ID")
	}
	if at.IsZero() || maximumAge < 0 {
		return Kill{}, false, errors.New("find recent statistics kill: invalid time range")
	}

	var kill Kill
	var killType string
	err = tx.QueryRow(`
		SELECT id, zone_id, mob_id, killed_at, kill_type
		FROM kills
		WHERE zone_id = ?
			AND mob_id = ?
			AND killed_at >= ?
			AND killed_at <= ?
		ORDER BY killed_at DESC, id DESC
		LIMIT 1
	`, zoneID, mobID, at.Add(-maximumAge), at).Scan(
		&kill.ID,
		&kill.ZoneID,
		&kill.MobID,
		&kill.KilledAt,
		&killType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Kill{}, false, nil
	}
	if err != nil {
		return Kill{}, false, fmt.Errorf("find recent statistics kill: %w", err)
	}
	kill.Type = KillType(killType)
	return kill, true, nil
}

func GetRecentKills(db *sql.DB, since time.Time) ([]Kill, error) {
	if db == nil {
		return nil, errors.New("get recent statistics kills: database is nil")
	}
	rows, err := db.Query(`
		SELECT id, zone_id, mob_id, killed_at, kill_type
		FROM kills
		WHERE killed_at >= ?
		ORDER BY killed_at, id
	`, since)
	if err != nil {
		return nil, fmt.Errorf("get recent statistics kills: %w", err)
	}
	defer rows.Close()

	kills := make([]Kill, 0)
	for rows.Next() {
		var kill Kill
		var killType string
		if err := rows.Scan(&kill.ID, &kill.ZoneID, &kill.MobID, &kill.KilledAt, &killType); err != nil {
			return nil, fmt.Errorf("scan recent statistics kill: %w", err)
		}
		kill.Type = KillType(killType)
		kills = append(kills, kill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recent statistics kills: %w", err)
	}
	return kills, nil
}
