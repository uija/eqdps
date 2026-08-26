package statistics

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (i *Import) GetOrCreateItem(name string) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("get or create statistics item: name is empty")
	}
	var id int64
	if err := tx.QueryRow(`
		INSERT INTO items (name)
		VALUES (?)
		ON CONFLICT (name) DO UPDATE SET name = items.name
		RETURNING id
	`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("get or create statistics item %q: %w", name, err)
	}
	return id, nil
}

func (i *Import) InsertLoot(
	zoneID, mobID int64,
	killID *int64,
	itemID int64,
	rawItemName string,
	quantity int,
	lootedAt time.Time,
	destination LootDestination,
	saleValueCopper *int64,
) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if zoneID < 1 {
		return 0, fmt.Errorf("insert statistics loot: zone ID %d is invalid", zoneID)
	}
	if mobID < 1 {
		return 0, fmt.Errorf("insert statistics loot: mob ID %d is invalid", mobID)
	}
	if killID != nil && *killID < 1 {
		return 0, fmt.Errorf("insert statistics loot: kill ID %d is invalid", *killID)
	}
	if itemID < 1 {
		return 0, fmt.Errorf("insert statistics loot: item ID %d is invalid", itemID)
	}
	rawItemName = strings.TrimSpace(rawItemName)
	if rawItemName == "" {
		return 0, errors.New("insert statistics loot: raw item name is empty")
	}
	if quantity < 1 {
		return 0, fmt.Errorf("insert statistics loot: quantity %d is invalid", quantity)
	}
	if lootedAt.IsZero() {
		return 0, errors.New("insert statistics loot: timestamp is zero")
	}
	if !destination.valid() {
		return 0, fmt.Errorf("insert statistics loot: destination %q is invalid", destination)
	}
	if saleValueCopper != nil && *saleValueCopper < 0 {
		return 0, fmt.Errorf("insert statistics loot: sale value %d is negative", *saleValueCopper)
	}
	result, err := tx.Exec(`
		INSERT INTO loot (
			zone_id,
			mob_id,
			kill_id,
			item_id,
			raw_item_name,
			quantity,
			looted_at,
			destination,
			sale_value_copper
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		zoneID,
		mobID,
		nullableInt64(killID),
		itemID,
		rawItemName,
		quantity,
		lootedAt,
		destination,
		nullableInt64(saleValueCopper),
	)
	if err != nil {
		return 0, fmt.Errorf("insert statistics loot: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics loot ID: %w", err)
	}
	return id, nil
}

func (i *Import) InsertMoney(
	zoneID *int64,
	receivedAt time.Time,
	amountCopper int64,
	source MoneySource,
) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if zoneID != nil && *zoneID < 1 {
		return 0, fmt.Errorf("insert statistics money: zone ID %d is invalid", *zoneID)
	}
	if receivedAt.IsZero() {
		return 0, errors.New("insert statistics money: timestamp is zero")
	}
	if amountCopper < 0 {
		return 0, fmt.Errorf("insert statistics money: amount %d is negative", amountCopper)
	}
	if !source.valid() {
		return 0, fmt.Errorf("insert statistics money: source %q is invalid", source)
	}
	result, err := tx.Exec(`
		INSERT INTO money (zone_id, received_at, amount_copper, source)
		VALUES (?, ?, ?, ?)
	`, nullableInt64(zoneID), receivedAt, amountCopper, source)
	if err != nil {
		return 0, fmt.Errorf("insert statistics money: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics money ID: %w", err)
	}
	return id, nil
}

func (i *Import) AssociateMoneyWithKill(moneyID, killID int64) error {
	tx, err := i.activeTx()
	if err != nil {
		return err
	}
	if moneyID < 1 || killID < 1 {
		return errors.New("associate statistics money: invalid money or kill ID")
	}
	result, err := tx.Exec(`
		UPDATE money
		SET kill_id = ?
		WHERE id = ? AND kill_id IS NULL
	`, killID, moneyID)
	if err != nil {
		return fmt.Errorf("associate statistics money with kill: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check statistics money association: %w", err)
	}
	if rows != 1 {
		return errors.New("associate statistics money with kill: money record was not found")
	}
	return nil
}

func (i *Import) InsertExperience(zoneID *int64, receivedAt time.Time, percent *float64) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if zoneID != nil && *zoneID < 1 {
		return 0, fmt.Errorf("insert statistics experience: zone ID %d is invalid", *zoneID)
	}
	if receivedAt.IsZero() {
		return 0, errors.New("insert statistics experience: timestamp is zero")
	}
	if percent != nil && (math.IsNaN(*percent) || math.IsInf(*percent, 0) || *percent < 0) {
		return 0, fmt.Errorf("insert statistics experience: percentage %v is invalid", *percent)
	}
	var percentValue any
	if percent != nil {
		percentValue = *percent
	}
	result, err := tx.Exec(`
		INSERT INTO experience (zone_id, received_at, percent)
		VALUES (?, ?, ?)
	`, nullableInt64(zoneID), receivedAt, percentValue)
	if err != nil {
		return 0, fmt.Errorf("insert statistics experience: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics experience ID: %w", err)
	}
	return id, nil
}

func (i *Import) InsertLevel(zoneID *int64, reachedAt time.Time, level int) (int64, error) {
	tx, err := i.activeTx()
	if err != nil {
		return 0, err
	}
	if zoneID != nil && *zoneID < 1 {
		return 0, fmt.Errorf("insert statistics level: zone ID %d is invalid", *zoneID)
	}
	if reachedAt.IsZero() {
		return 0, errors.New("insert statistics level: timestamp is zero")
	}
	if level < 1 {
		return 0, fmt.Errorf("insert statistics level: level %d is invalid", level)
	}
	result, err := tx.Exec(`
		INSERT INTO levels (zone_id, reached_at, level)
		VALUES (?, ?, ?)
	`, nullableInt64(zoneID), reachedAt, level)
	if err != nil {
		return 0, fmt.Errorf("insert statistics level: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read statistics level ID: %w", err)
	}
	return id, nil
}
