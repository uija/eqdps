package sky

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/uija/eqdps/internal/data"
)

func (m *Module) HandleInventoryUpload(event *data.LogRowEvent) {
	if m.replay || !m.ctx.Config.SkyConfig.ParseInventoryData {
		return
	}
	exportPath := filepath.Join(
		filepath.Dir(filepath.Dir(m.configPath)),
		event.Data[1],
	)
	inv, err := ReadInventoryExport(exportPath, m.db)

	if err != nil {
		log.Printf("Unable to read inventory export. %v", err)
		return
	}
	// set all that we dont have to 0
	for _, c := range m.db.Classes {
		for _, q := range c.Quests {
			for _, i := range q.Requirements {
				if !strings.Contains(i.Name, "Wind Rune") {
					if _, ok := inv[i.Name]; !ok {
						inv[i.Name] = 0
					}
				}
			}
		}
	}

	changed := false
	for name, count := range inv {
		lname := strings.ToLower(name)
		if m.config.QuestItems[lname] != count {
			m.config.QuestItems[lname] = count
			changed = true
		}
	}
	if changed {
		m.config.Save()
	}
	m.invalidFunc()
}
