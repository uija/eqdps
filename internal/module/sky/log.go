package sky

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/eqldb"
)

func (m *Module) OnLogRow(event *data.LogRowEvent) {
	if !m.readyToRead && !m.replay {
		log.Printf("Message while we are not ready to read")
		return
	}
	if event.Offset <= m.config.Log.Offset {
		return
	}
	changed := true
	switch event.Type {
	case data.LogRowEventTypeInventoryExport:
		m.HandleInventoryUpload(event)
	case data.LogRowEventTypeItemDestroyed:
		_, item_orig := normalizeItemName(event.Data[2])
		count, err := strconv.Atoi(event.Data[1])
		if err != nil {
			return
		}
		item := strings.ToLower(item_orig)
		m.EqlDbItem(item_orig, -count, event.Timestamp)
		if _, ok := m.config.QuestItems[item]; ok {
			if m.config.QuestItems[item] > count {
				m.config.QuestItems[item] -= count
			} else {
				delete(m.config.QuestItems, item)
			}
			m.config.Save()
		}
	case data.LogRowEventTypeExperience:
	case data.LogRowEventTypeKillExperienceReward:
		if m.tradeIn != nil {
			m.tradeIn.Experience = event.Timestamp
		}

	case data.LogRowEventTypeLoot:
		quantity, item := normalizeItemName(event.Data[1])
		if m.HandleLootedItems(quantity, item) {
			m.EqlDbItem(item, quantity, event.Timestamp)
			changed = true
		}
	case data.LogRowEventTypeLootResult:
		quantity, item := normalizeItemName(event.Data[1])
		if strings.Contains(event.Data[3], "and stored") {
			if m.HandleLootedItems(quantity, item) {
				m.EqlDbItem(item, quantity, event.Timestamp)
				changed = true
			}
		}
	case data.LogRowEventTypeTradeOffer:
		amount, err := strconv.Atoi(event.Data[1])
		if err != nil {
			return
		}
		_, item := normalizeItemName("a " + event.Data[2])
		lname := strings.ToLower(item)

		npc := event.Data[3]

		if m.tradeIn != nil {
			if !strings.EqualFold(m.tradeIn.Npc, npc) {
				m.tradeIn = nil
			}
		}
		if m.tradeIn == nil {
			m.tradeIn = &TradeInTracker{
				Npc:        npc,
				Given:      make(map[string]int),
				LastUpdate: time.Time{},
				Denied:     time.Time{},
				Experience: time.Time{},
			}
		}
		m.tradeIn.Given[lname] += amount
		m.tradeIn.LastUpdate = event.Timestamp
	case data.LogRowEventTypeTradeCancel:
		m.tradeIn = nil
	case data.LogRowEventTypeTradeComplete:
		if m.tradeIn == nil {
			//log.Printf("No tradein! %s", event.Data[0])
			return
		}
		if !m.tradeIn.Denied.IsZero() {
			//log.Printf("Trade in was denied")
			return
		}
		// TODO check experience
		diff := event.Timestamp.Sub(m.tradeIn.LastUpdate)
		if diff > 10*time.Second {
			//log.Printf("Experience vs handin diff is too big. %s", event.Data[0])
		}

		// find the quest
		var quest *Quest = nil
		for _, c := range m.db.Classes {
			for _, q := range c.Quests {
				if m.match(q) {
					quest = &q
					break
				}
			}
		}
		if quest == nil {
			return
		}
		for item := range m.tradeIn.Given {
			lname := strings.ToLower(item)
			if m.config.QuestItems[lname] > 1 {
				m.config.QuestItems[lname] -= 1
			} else {
				delete(m.config.QuestItems, lname)
			}
		}
		m.config.Quests[quest.Name]++
		m.EqlDbQuest(quest.Name, event.Timestamp)
		if m.config.RedoQuests[quest.Name].IsZero() || event.Timestamp.After(m.config.RedoQuests[quest.Name]) {
			delete(m.config.RedoQuests, quest.Name)
		}
		m.tradeIn = nil
	}
	if changed {
		m.config.Log.LastTimestamp = event.Timestamp
		m.config.Log.Offset = event.Offset
	}
	if !m.replay {
		if changed {
			m.config.Save()
		}
		m.RecalculateStatus()
	}
}

func (m *Module) HandleLootedItems(quantity int, item string) bool {
	lname := strings.ToLower(item)
	for _, c := range m.db.Classes {
		for _, q := range c.Quests {
			for _, r := range q.Requirements {
				if strings.EqualFold(item, r.Name) {
					amount, ok := m.config.QuestItems[lname]
					if !ok {
						amount = 0
					}
					amount += quantity
					m.config.QuestItems[lname] = amount
					return true
				}
			}
		}
	}
	return false
}
func normalizeItemName(value string) (int, string) {
	value = strings.TrimSpace(value)
	prefix, item, ok := strings.Cut(value, " ")
	if !ok {
		return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
	}

	quantity := 1
	if !strings.EqualFold(prefix, "a") && !strings.EqualFold(prefix, "an") {
		parsed, err := strconv.Atoi(prefix)
		if err != nil || parsed < 1 {
			return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
		}
		quantity = parsed
	}

	item = strings.TrimSpace(item)
	return quantity, itemUpgradeSuffixRE.ReplaceAllString(item, "")
}
func (m *Module) EqlDbQuest(name string, timestamp time.Time) {
	if !m.ctx.Config.EQLDbConfig.UploadSkyData {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eqldb_events = append(m.eqldb_events, eqldb.PlaneOfSkyEvent{
		Type:      eqldb.PlaneOfSkyEventTypeQuestTurnIn,
		Quest:     normalizeEqldbQuestName(name),
		Timestamp: timestamp.Format("Mon Jan 02 15:04:05 2006"),
	})
	if len(m.eqldb_events) > 1900 {
		go m.TickSkyItemUpload()
	}
}
func (m *Module) EqlDbItem(item string, amount int, timestamp time.Time) {
	if !m.ctx.Config.EQLDbConfig.UploadSkyData {
		return
	}
	if !strings.Contains(strings.ToLower(item), "wind rune") {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t := eqldb.PlaneOfSkyEventTypeWindRuneReceive
	if amount < 0 {
		t = eqldb.PlaneOfSkyEventTypeWindRuneDelete
		amount *= -1
	}
	m.eqldb_events = append(m.eqldb_events, eqldb.PlaneOfSkyEvent{
		Type:      t,
		Rune:      item,
		Amount:    amount,
		Timestamp: timestamp.Format("Mon Jan 02 15:04:05 2006"),
	})
	if len(m.eqldb_events) > 1900 {
		go m.TickSkyItemUpload()
	}
}
func (m *Module) match(quest Quest) bool {
	if m.tradeIn == nil {
		return false
	}
	if !strings.EqualFold(m.tradeIn.Npc, quest.QuestGiver) {
		return false
	}
	for _, r := range quest.Requirements {
		lname := strings.ToLower(r.Name)
		if _, ok := m.tradeIn.Given[lname]; !ok {
			return false
		}
	}
	return true
}
func normalizeEqldbQuestName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}
