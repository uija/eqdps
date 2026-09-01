package equipment

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/inventory"
)

func (m *Module) ImportInventory(path string, datapath string) (*inventory.Inventory, error) {
	if m.ctx.Config.EQLDbConfig.AccessToken == "" {
		return nil, fmt.Errorf("Not logged in to eqldb")
	}
	database, err := inventory.OpenDatabase()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	inv, err := inventory.Parse(path)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, storage := range inv.Storage {
		for _, root := range storage.Slots {
			if root.Id != "" && !slices.Contains(ids, root.Id) {
				ids = append(ids, root.Id)
			}
			for _, item := range root.Slots {
				if item.Id != "" && !slices.Contains(ids, item.Id) {
					ids = append(ids, item.Id)
				}
				for _, aug := range item.Slots {
					if aug.Id != "" && !slices.Contains(ids, aug.Id) {
						ids = append(ids, aug.Id)
					}
				}
			}
		}
	}
	requestIds, err := database.GetIDsForRequest(ids)
	if err != nil {
		return nil, err
	}
	for len(requestIds) > 0 {
		batchSize := min(len(requestIds), 100)
		batch := requestIds[:batchSize]
		requestIds = requestIds[batchSize:]

		data, err := eqldb.GetItemMetadata(m.ctx.Config.EQLDbConfig.AccessToken, batch...)
		if err != nil {
			return nil, err
		}
		if err := database.StoreRequestedItems(data); err != nil {
			return nil, err
		}
	}
	data, err := database.GetItemData(ids)
	if err != nil {
		return nil, err
	}
	for sidx, storage := range inv.Storage {
		for ridx, root := range storage.Slots {
			if d, ok := data[root.Id]; ok {
				root.Data = &d
				root.Stats = d.GetStats(root.Level)
			}
			for iidx, item := range root.Slots {
				if d, ok := data[item.Id]; ok {
					item.Data = &d
					item.Stats = d.GetStats(item.Level)
				}
				for aidx, aug := range item.Slots {
					if d, ok := data[aug.Id]; ok {
						aug.Data = &d
						aug.Stats = d.GetStats(aug.Level)
					}
					item.Slots[aidx] = aug
				}
				root.Slots[iidx] = item
			}
			inv.Storage[sidx].Slots[ridx] = root
		}
	}

	bytes, err := json.Marshal(inv)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(datapath, bytes, 0o644)
	return &inv, err
}
