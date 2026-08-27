package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Import struct {
	tx       *sql.Tx
	finished bool
}

func (i *Import) activeTx() (*sql.Tx, error) {
	if i == nil || i.tx == nil {
		return nil, errors.New("statistics import is nil")
	}
	if i.finished {
		return nil, errors.New("statistics import is already finished")
	}
	return i.tx, nil
}

func BeginImport(db *sql.DB) (*Import, error) {
	if db == nil {
		return nil, errors.New("begin statistics import: database is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin statistics import: %w", err)
	}
	return &Import{tx: tx}, nil
}

func (i *Import) Commit(offset int64, lastTimestamp time.Time) error {
	if i == nil || i.tx == nil {
		return errors.New("commit statistics import: import is nil")
	}
	if i.finished {
		return errors.New("commit statistics import: import is already finished")
	}
	if offset < 0 {
		return fmt.Errorf("commit statistics import: offset %d is negative", offset)
	}
	var timestamp any
	if !lastTimestamp.IsZero() {
		timestamp = lastTimestamp
	}
	if _, err := i.tx.Exec(`
		INSERT INTO log_state (id, byte_offset, last_timestamp)
		VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			byte_offset = excluded.byte_offset,
			last_timestamp = COALESCE(excluded.last_timestamp, log_state.last_timestamp)
	`, offset, timestamp); err != nil {
		return fmt.Errorf("store statistics log offset: %w", err)
	}
	if err := i.tx.Commit(); err != nil {
		i.finished = true
		return fmt.Errorf("commit statistics import: %w", err)
	}
	i.finished = true
	return nil
}

func (i *Import) Rollback() error {
	if i == nil || i.tx == nil || i.finished {
		return nil
	}
	i.finished = true
	if err := i.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("rollback statistics import: %w", err)
	}
	return nil
}

func GetLogOffset(db *sql.DB) (int64, error) {
	if db == nil {
		return 0, errors.New("get statistics log offset: database is nil")
	}
	var offset int64
	if err := db.QueryRow(`
		SELECT byte_offset
		FROM log_state
		WHERE id = 1
	`).Scan(&offset); err != nil {
		return 0, fmt.Errorf("get statistics log offset: %w", err)
	}
	return offset, nil
}

func GetLastLogTimestamp(db *sql.DB) (time.Time, error) {
	if db == nil {
		return time.Time{}, errors.New("get statistics last log timestamp: database is nil")
	}
	var timestamp sql.NullTime
	if err := db.QueryRow(`
		SELECT last_timestamp
		FROM log_state
		WHERE id = 1
	`).Scan(&timestamp); err != nil {
		return time.Time{}, fmt.Errorf("get statistics last log timestamp: %w", err)
	}
	if !timestamp.Valid {
		return time.Time{}, nil
	}
	return timestamp.Time, nil
}

func GetOpenZoneVisit(db *sql.DB) (int64, int64, time.Time, bool, error) {
	if db == nil {
		return 0, 0, time.Time{}, false, errors.New("get open statistics zone visit: database is nil")
	}
	var visitID, zoneID int64
	var enteredAt time.Time
	err := db.QueryRow(`
		SELECT id, zone_id, entered_at
		FROM zone_visits
		WHERE id = (SELECT MAX(id) FROM zone_visits)
			AND left_at IS NULL
	`).Scan(&visitID, &zoneID, &enteredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, time.Time{}, false, nil
	}
	if err != nil {
		return 0, 0, time.Time{}, false, fmt.Errorf("get open statistics zone visit: %w", err)
	}
	return visitID, zoneID, enteredAt, true, nil
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
