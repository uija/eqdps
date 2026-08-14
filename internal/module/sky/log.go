package sky

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
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
	case data.LogRowEventTypeExperience:
	case data.LogRowEventTypeKillExperienceReward:
		if m.tradeIn != nil {
			m.tradeIn.Experience = event.Timestamp
		}

	case data.LogRowEventTypeLoot:
		quantity, item := normalizeItemName(event.Data[1])
		if m.HandleLootedItems(quantity, item) {
			changed = true
		}
	case data.LogRowEventTypeLootResult:
		quantity, item := normalizeItemName(event.Data[1])
		if m.HandleLootedItems(quantity, item) {
			changed = true
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
		var class_name = ""
		for _, c := range m.db.Classes {
			for _, q := range c.Quests {
				if m.match(q) {
					quest = &q
					class_name = c.Name
					break
				}
			}
		}
		if quest == nil {
			//log.Printf("Didn't fine a quest for that! %#v", m.tradeIn)
			return
		}
		quest_name := QuestName(quest.Name, class_name)

		for item := range m.tradeIn.Given {
			lname := strings.ToLower(item)
			if m.config.QuestItems[lname] > 1 {
				m.config.QuestItems[lname] -= 1
			} else {
				delete(m.config.QuestItems, lname)
			}
		}
		m.config.Quests[quest_name]++
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
