package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqldb"
)

const ItemMetadataRetryInterval = 7 * 24 * time.Hour

const databaseQueryBatchSize = 500

var metadataLineBreakRE = regexp.MustCompile(`(?i)<br\s*/?>`)

var allItemClasses = []string{
	"WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD",
	"ROG", "SHM", "NEC", "WIZ", "MAG", "ENC", "BST", "BER",
}

type ItemData struct {
	ID       string
	Name     string
	Slots    []string
	Classes  []string
	Metadata map[string]any
	Location string
}

func (item *ItemData) UnmarshalJSON(data []byte) error {
	type plainItemData ItemData
	var decoded plainItemData
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.Slots = normalizeItemSlots(decoded.Slots)
	*item = ItemData(decoded)
	return nil
}

type Database struct {
	db *sql.DB
}

func OpenDatabase() (*Database, error) {
	path, err := data.AppDataPath("items.sqlite")
	if err != nil {
		return nil, fmt.Errorf("get item database path: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open item database: %w", err)
	}
	db.SetMaxOpenConns(1)

	database := &Database{db: db}
	if err := database.prepare(); err != nil {
		db.Close()
		return nil, err
	}
	return database, nil
}

func (d *Database) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *Database) prepare() error {
	if d == nil || d.db == nil {
		return fmt.Errorf("item database is not open")
	}
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			item_id TEXT PRIMARY KEY,
			metadata TEXT,
			requested_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare item database: %w", err)
	}
	return nil
}

// GetIDsForRequest returns unknown IDs and IDs whose previous request returned
// no metadata long enough ago to be tried again.
func (d *Database) GetIDsForRequest(itemIDs []string) ([]string, error) {
	ids, err := uniqueItemIDs(itemIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []string{}, nil
	}

	type requestState struct {
		hasMetadata bool
		requestedAt time.Time
	}
	states := make(map[string]requestState, len(ids))
	for start := 0; start < len(ids); start += databaseQueryBatchSize {
		end := min(start+databaseQueryBatchSize, len(ids))
		batch := ids[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		arguments := make([]any, len(batch))
		for index := range batch {
			arguments[index] = batch[index]
		}

		rows, err := d.db.Query(
			`SELECT item_id, metadata IS NOT NULL, requested_at FROM items WHERE item_id IN (`+placeholders+`)`,
			arguments...,
		)
		if err != nil {
			return nil, fmt.Errorf("find item metadata request candidates: %w", err)
		}
		for rows.Next() {
			var id string
			var hasMetadata bool
			var requestedAt int64
			if err := rows.Scan(&id, &hasMetadata, &requestedAt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("read item metadata request candidate: %w", err)
			}
			states[id] = requestState{
				hasMetadata: hasMetadata,
				requestedAt: time.Unix(requestedAt, 0),
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read item metadata request candidates: %w", err)
		}
		rows.Close()
	}

	retryBefore := time.Now().Add(-ItemMetadataRetryInterval)
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		state, found := states[id]
		if !found || (!state.hasMetadata && !state.requestedAt.After(retryBefore)) {
			result = append(result, id)
		}
	}
	return result, nil
}

// StoreRequestedItems stores every entry returned by the metadata API. A nil
// Data value is stored as SQL NULL so it can be requested again later.
func (d *Database) StoreRequestedItems(items []eqldb.ItemMetadata) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin storing item metadata: %w", err)
	}
	defer tx.Rollback()

	statement, err := tx.Prepare(`
		INSERT INTO items (item_id, metadata, requested_at)
		VALUES (?, ?, ?)
		ON CONFLICT (item_id) DO UPDATE SET
			metadata = excluded.metadata,
			requested_at = excluded.requested_at
	`)
	if err != nil {
		return fmt.Errorf("prepare storing item metadata: %w", err)
	}
	defer statement.Close()

	requestedAt := time.Now().Unix()
	for _, item := range items {
		if _, err := uniqueItemIDs([]string{item.ID}); err != nil {
			return err
		}
		var metadata any
		if item.Data != nil {
			encoded, err := json.Marshal(item.Data)
			if err != nil {
				return fmt.Errorf("encode metadata for item %s: %w", item.ID, err)
			}
			metadata = string(encoded)
		}
		if _, err := statement.Exec(item.ID, metadata, requestedAt); err != nil {
			return fmt.Errorf("store metadata for item %s: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit item metadata: %w", err)
	}
	return nil
}

// GetItemData returns known metadata keyed by item ID. Unknown items and items
// whose API response was null are omitted.
func (d *Database) GetItemData(itemIDs []string) (map[string]ItemData, error) {
	ids, err := uniqueItemIDs(itemIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]ItemData{}, nil
	}

	known := make(map[string]ItemData, len(ids))
	for start := 0; start < len(ids); start += databaseQueryBatchSize {
		end := min(start+databaseQueryBatchSize, len(ids))
		batch := ids[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		arguments := make([]any, len(batch))
		for index := range batch {
			arguments[index] = batch[index]
		}

		rows, err := d.db.Query(
			`SELECT item_id, metadata FROM items WHERE metadata IS NOT NULL AND item_id IN (`+placeholders+`)`,
			arguments...,
		)
		if err != nil {
			return nil, fmt.Errorf("get item metadata: %w", err)
		}
		for rows.Next() {
			var id string
			var encoded string
			if err := rows.Scan(&id, &encoded); err != nil {
				rows.Close()
				return nil, fmt.Errorf("read item metadata: %w", err)
			}
			var metadata map[string]any
			if err := json.Unmarshal([]byte(encoded), &metadata); err != nil {
				rows.Close()
				return nil, fmt.Errorf("decode metadata for item %s: %w", id, err)
			}
			known[id] = itemDataFromMetadata(id, metadata)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read item metadata: %w", err)
		}
		rows.Close()
	}

	return known, nil
}

func itemDataFromMetadata(id string, metadata map[string]any) ItemData {
	return ItemData{
		ID:       id,
		Name:     metadataString(metadata, "itemname"),
		Slots:    normalizeItemSlots(metadataValues(metadata, "slots", "slot")),
		Classes:  metadataClasses(metadata),
		Metadata: metadata,
	}
}

func normalizeItemSlots(slots []string) []string {
	for index, slot := range slots {
		if strings.EqualFold(strings.TrimSpace(slot), "FINGERS") {
			slots[index] = "FINGER"
		}
	}
	return slots
}

func metadataClasses(metadata map[string]any) []string {
	classes := metadataValues(metadata, "classes", "class")
	if len(classes) < 2 || !strings.EqualFold(classes[0], "ALL") || !strings.EqualFold(classes[1], "except") {
		return classes
	}
	if len(classes) == 2 {
		return []string{"ALL"}
	}

	excluded := make(map[string]bool, len(classes)-2)
	for _, class := range classes[2:] {
		excluded[strings.ToUpper(class)] = true
	}
	result := make([]string, 0, len(allItemClasses)-len(excluded))
	for _, class := range allItemClasses {
		if !excluded[class] {
			result = append(result, class)
		}
	}
	return result
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataValues(metadata map[string]any, key, statsLabel string) []string {
	if value, found := metadata[key]; found {
		switch value := value.(type) {
		case string:
			return splitMetadataValues(value)
		case []any:
			result := make([]string, 0, len(value))
			for _, entry := range value {
				if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
					result = append(result, strings.TrimSpace(text))
				}
			}
			return result
		}
	}

	statsblock := metadataString(metadata, "statsblock")
	for _, line := range strings.Split(metadataLineBreakRE.ReplaceAllString(statsblock, "\n"), "\n") {
		label, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(label), statsLabel) {
			return splitMetadataValues(value)
		}
	}
	return []string{}
}

func splitMetadataValues(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool {
		return character == ',' || character == '/' || character == ' ' || character == '\t'
	})
}

func uniqueItemIDs(itemIDs []string) ([]string, error) {
	seen := make(map[string]bool, len(itemIDs))
	result := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		id = strings.TrimSpace(id)
		if !validItemID(id) {
			return nil, fmt.Errorf("invalid item ID %q", id)
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result, nil
}

func validItemID(id string) bool {
	if len(id) < 1 || len(id) > 20 {
		return false
	}
	positive := false
	for _, character := range id {
		if character < '0' || character > '9' {
			return false
		}
		positive = positive || character != '0'
	}
	return positive
}
