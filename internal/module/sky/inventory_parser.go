package sky

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ReadInventoryExport returns quantities for known Plane of Sky quest items.
// Wind Runes are deliberately excluded because they are stored on a currency
// tab that is not represented reliably by inventory exports.
func ReadInventoryExport(path string, database Database) (map[string]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inventory export: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read inventory export header: %w", err)
	}
	nameColumn, countColumn := columnIndex(header, "Name"), columnIndex(header, "Count")
	if nameColumn < 0 || countColumn < 0 {
		return nil, fmt.Errorf("inventory export is missing Name or Count column")
	}

	result := make(map[string]int)
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read inventory export: %w", readErr)
		}
		if nameColumn >= len(row) || countColumn >= len(row) {
			continue
		}
		inventoryName := strings.TrimSpace(row[nameColumn])
		// Exaltation rows are augment slots belonging to the parent item, not
		// additional inventory copies.
		if strings.HasSuffix(inventoryName, " (Exaltation)") {
			continue
		}
		item, known := knownItem(&database, inventoryName)
		if !known || strings.HasPrefix(item, "Wind Rune ") {
			continue
		}
		quantity, parseErr := strconv.Atoi(strings.TrimSpace(row[countColumn]))
		if parseErr != nil || quantity < 1 {
			continue
		}
		result[item] += quantity
	}
	return result, nil
}

func knownItem(db *Database, name string) (string, bool) {
	_, item := normalizeItemName(name)
	for _, c := range db.Classes {
		for _, q := range c.Quests {
			for _, i := range q.Requirements {
				if strings.EqualFold(item, i.Name) {
					return i.Name, true
				}
			}
		}
	}
	return item, false
}

func columnIndex(header []string, name string) int {
	for index, value := range header {
		if strings.EqualFold(strings.TrimSpace(value), name) {
			return index
		}
	}
	return -1
}
