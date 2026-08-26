package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (i *Import) InsertChat(
	sentAt time.Time,
	channel ChatChannel,
	direction Direction,
	otherCharacter string,
) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if sentAt.IsZero() {
		return 0, errors.New("insert statistics chat: timestamp is zero")
	}
	if strings.TrimSpace(string(channel)) == "" {
		return 0, errors.New("insert statistics chat: channel is empty")
	}
	if !direction.valid() {
		return 0, fmt.Errorf("insert statistics chat: direction %q is invalid", direction)
	}
	otherCharacter = strings.TrimSpace(otherCharacter)
	var otherValue any
	if otherCharacter != "" {
		otherValue = otherCharacter
	}
	result, err := tx.Exec(`
		INSERT INTO chat (sent_at, channel, direction, other_character)
		VALUES (?, ?, ?, ?)
	`, sentAt, channel, direction, otherValue)
	if err != nil {
		return 0, fmt.Errorf("insert statistics chat: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics chat ID: %w", err)
	}
	return id, nil
}

func (i *Import) InsertParcel(
	direction Direction,
	otherCharacter string,
	itemID *int64,
	quantity int,
	moneyCopper *int64,
	observedAt time.Time,
) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	otherCharacter = strings.TrimSpace(otherCharacter)
	if !direction.valid() {
		return 0, fmt.Errorf("insert statistics parcel: direction %q is invalid", direction)
	}
	if otherCharacter == "" {
		return 0, errors.New("insert statistics parcel: other character is empty")
	}
	if itemID != nil && *itemID < 1 {
		return 0, fmt.Errorf("insert statistics parcel: item ID %d is invalid", *itemID)
	}
	if moneyCopper != nil && *moneyCopper < 0 {
		return 0, fmt.Errorf("insert statistics parcel: money amount %d is negative", *moneyCopper)
	}
	if (itemID == nil) == (moneyCopper == nil) {
		return 0, errors.New("insert statistics parcel: exactly one of item or money is required")
	}
	if quantity < 1 {
		return 0, fmt.Errorf("insert statistics parcel: quantity %d is invalid", quantity)
	}
	if observedAt.IsZero() {
		return 0, errors.New("insert statistics parcel: timestamp is zero")
	}
	result, err := tx.Exec(`
		INSERT INTO parcels (
			direction,
			other_character,
			item_id,
			quantity,
			money_copper,
			sent_or_received_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		direction,
		otherCharacter,
		nullableInt64(itemID),
		quantity,
		nullableInt64(moneyCopper),
		observedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert statistics parcel: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics parcel ID: %w", err)
	}
	return id, nil
}

func (i *Import) FindPendingParcel(
	sender string,
	itemID *int64,
	quantity int,
	moneyCopper *int64,
) (int64, bool, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, false, err
	}
	sender = strings.TrimSpace(sender)
	if sender == "" {
		return 0, false, errors.New("find pending statistics parcel: sender is empty")
	}
	if quantity < 1 {
		return 0, false, fmt.Errorf("find pending statistics parcel: quantity %d is invalid", quantity)
	}
	if (itemID == nil) == (moneyCopper == nil) {
		return 0, false, errors.New("find pending statistics parcel: exactly one of item or money is required")
	}

	var id int64
	var queryErr error
	if itemID != nil {
		if *itemID < 1 {
			return 0, false, fmt.Errorf("find pending statistics parcel: item ID %d is invalid", *itemID)
		}
		queryErr = tx.QueryRow(`
			SELECT id
			FROM parcels
			WHERE direction = 'received'
				AND collected_at IS NULL
				AND other_character = ?
				AND item_id = ?
				AND quantity = ?
			ORDER BY sent_or_received_at, id
			LIMIT 1
		`, sender, *itemID, quantity).Scan(&id)
	} else {
		if *moneyCopper < 0 {
			return 0, false, fmt.Errorf("find pending statistics parcel: money amount %d is negative", *moneyCopper)
		}
		queryErr = tx.QueryRow(`
			SELECT id
			FROM parcels
			WHERE direction = 'received'
				AND collected_at IS NULL
				AND other_character = ?
				AND money_copper = ?
			ORDER BY sent_or_received_at, id
			LIMIT 1
		`, sender, *moneyCopper).Scan(&id)
	}
	if errors.Is(queryErr, sql.ErrNoRows) {
		return 0, false, nil
	}
	if queryErr != nil {
		return 0, false, fmt.Errorf("find pending statistics parcel: %w", queryErr)
	}
	return id, true, nil
}

func (i *Import) MarkParcelCollected(parcelID int64, collectedAt time.Time) error {
	tx, err := i.activeTx()
	if err != nil {
		return err
	}
	if parcelID < 1 {
		return fmt.Errorf("mark statistics parcel collected: parcel ID %d is invalid", parcelID)
	}
	if collectedAt.IsZero() {
		return errors.New("mark statistics parcel collected: timestamp is zero")
	}
	result, err := tx.Exec(`
		UPDATE parcels
		SET collected_at = ?
		WHERE id = ?
			AND direction = 'received'
			AND collected_at IS NULL
	`, collectedAt, parcelID)
	if err != nil {
		return fmt.Errorf("mark statistics parcel collected: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check collected statistics parcel: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("mark statistics parcel collected: pending parcel %d does not exist", parcelID)
	}
	return nil
}

func GetPendingParcels(db *sql.DB) ([]Parcel, error) {
	if db == nil {
		return nil, errors.New("get pending statistics parcels: database is nil")
	}
	rows, err := db.Query(`
		SELECT
			id,
			direction,
			other_character,
			item_id,
			quantity,
			money_copper,
			sent_or_received_at,
			collected_at
		FROM parcels
		WHERE direction = 'received'
			AND collected_at IS NULL
		ORDER BY sent_or_received_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("get pending statistics parcels: %w", err)
	}
	defer rows.Close()

	parcels := make([]Parcel, 0)
	for rows.Next() {
		var parcel Parcel
		var direction string
		var itemID sql.NullInt64
		var moneyCopper sql.NullInt64
		var collectedAt sql.NullTime
		if err := rows.Scan(
			&parcel.ID,
			&direction,
			&parcel.OtherCharacter,
			&itemID,
			&parcel.Quantity,
			&moneyCopper,
			&parcel.SentOrReceivedAt,
			&collectedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending statistics parcel: %w", err)
		}
		parcel.Direction = Direction(direction)
		parcel.ItemID = int64Pointer(itemID)
		parcel.MoneyCopper = int64Pointer(moneyCopper)
		if collectedAt.Valid {
			value := collectedAt.Time
			parcel.CollectedAt = &value
		}
		parcels = append(parcels, parcel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending statistics parcels: %w", err)
	}
	return parcels, nil
}
