package inventory

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type Item struct {
	Name  string
	Level int
	Id    string
	IsBag bool
	Slots map[string]Item
	Data  *ItemData
	Stats map[string]float64
}
type Storage struct {
	Slots map[string]Item
}

type Inventory struct {
	Storage map[string]Storage
}

func NormalizeItemName(value string) (string, int) {
	name := strings.TrimSpace(value)
	separator := strings.LastIndexByte(name, ' ')
	if separator < 0 || separator+2 >= len(name) || name[separator+1] != '+' {
		return name, 0
	}

	level, err := strconv.Atoi(name[separator+2:])
	if err != nil || level < 1 {
		return name, 0
	}
	return strings.TrimSpace(name[:separator]), level
}

func Parse(path string) (Inventory, error) {
	inventory := Inventory{
		Storage: make(map[string]Storage),
	}

	file, err := os.Open(path)
	if err != nil {
		return inventory, fmt.Errorf("open inventory export: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return inventory, fmt.Errorf("read inventory export header: %w", err)
	}
	_ = header
	tmp := make(map[string][]string)
	keyRing := false
	keyRingCounts := make(map[string]int)
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return inventory, fmt.Errorf("read inventory export: %w", readErr)
		}
		if len(row) < 3 {
			continue
		}
		if row[0] == "KeyRing" && row[2] == "ID" {
			keyRing = true
			continue
		}
		if row[2] == "ID" {
			continue
		}
		if keyRing {
			category := row[0]
			index := keyRingCounts[category]
			keyRingCounts[category]++
			row[0] = fmt.Sprintf("%s %d", category, index)
		}
		tmp[row[0]] = row
	}
	// find all general locations
	storage_names := []string{"Bank", "General", "SharedBank", "Activated", "Equipment", "Augmentation"}
	for key, entry := range tmp {
		//log.Printf("%v", entry)
		if entry[1] == "Empty" {
			continue
		}
		storage, is_in_storage := func() (string, bool) {
			for _, sn := range storage_names {
				if strings.Contains(key, sn) {
					return sn, true
				}
			}
			return "", false
		}()
		if !is_in_storage {
			storage = "Wear"
		}
		if _, ok := inventory.Storage[storage]; !ok {
			inventory.Storage[storage] = Storage{
				Slots: make(map[string]Item, 0),
			}
		}
		itemName, level := NormalizeItemName(entry[1])
		fields := strings.Split(key, "-")
		if len(fields) == 0 {
			continue
		}
		if len(fields) == 1 { // first ebene
			var item Item
			if i, ok := inventory.Storage[storage].Slots[fields[0]]; ok {
				item = i
			} else {
				item.Slots = make(map[string]Item)
			}
			item.Id = entry[2]
			item.Name = itemName
			item.Level = level
			if _, ok := tmp[key+"-Slot1"]; ok {
				item.IsBag = true
			}
			inventory.Storage[storage].Slots[fields[0]] = item
		} else if len(fields) == 2 {
			// check first node
			var root Item
			if i, ok := inventory.Storage[storage].Slots[fields[0]]; ok {
				root = i
			} else {
				root.Slots = make(map[string]Item)
			}
			var item Item
			if i, ok := root.Slots[fields[1]]; ok {
				item = i
			} else {
				item.Slots = make(map[string]Item)
			}
			item.Id = entry[2]
			item.Name = itemName
			item.Level = level
			root.Slots[fields[1]] = item
			inventory.Storage[storage].Slots[fields[0]] = root
		} else if len(fields) == 3 {
			var root Item
			if i, ok := inventory.Storage[storage].Slots[fields[0]]; ok {
				root = i
			} else {
				root.Slots = make(map[string]Item)
			}
			var item Item
			if i, ok := root.Slots[fields[1]]; ok {
				item = i
			} else {
				item.Slots = make(map[string]Item)
			}

			var aug Item
			if i, ok := item.Slots[fields[2]]; ok {
				aug = i
			}
			aug.Id = entry[2]
			aug.Name = itemName
			aug.Level = level
			item.Slots[fields[2]] = aug
			root.Slots[fields[1]] = item
			inventory.Storage[storage].Slots[fields[0]] = root
		} else {
			log.Printf("Unhandled: %v", fields)
		}
	}
	return inventory, nil
}
