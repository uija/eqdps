package equipment

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/ui/form"
)

func (m *Module) PrepareItems() {
	if m.inv == nil {
		log.Printf("Inventory is nil")
		return
	}
	classes := []string{""}
	slots := []string{""}
	stats := []string{""}
	m.items = make([]ClickableItems, 0)

	addClasses := func(list []string) {
		for _, cl := range list {
			cl = strings.ToUpper(cl)
			if len(cl) == 3 && cl != "ALL" && !slices.Contains(classes, cl) {
				classes = append(classes, cl)
			}
		}
	}
	addSlots := func(list []string) {
		for _, cl := range list {
			cl = strings.ToUpper(cl)
			if len(cl) > 0 && cl != "ALL" && !slices.Contains(slots, cl) {
				slots = append(slots, cl)
			}
		}
	}
	addStats := func(list []string) {
		for _, stat := range list {
			if stat != "" && !slices.Contains(stats, stat) {
				stats = append(stats, stat)
			}
		}
	}
	for storage, s := range m.inv.Storage {
		for root, r := range s.Slots {
			cl, sl, st := m.AppendItem(&r, storage, root)
			addClasses(cl)
			addSlots(sl)
			addStats(st)
			for item, i := range r.Slots {
				if r.IsBag {
					cl, sl, st = m.AppendItem(&i, storage, root, item)
				} else {
					cl, sl, st = m.AppendItem(&i, storage, root, r.Name)
				}
				addClasses(cl)
				addSlots(sl)
				addStats(st)
				for aug, a := range i.Slots {
					if i.IsBag {
						cl, sl, st = m.AppendItem(&a, storage, root, item, aug)
					} else {
						cl, sl, st = m.AppendItem(&a, storage, root, item, i.Name)
					}
					addClasses(cl)
					addSlots(sl)
					addStats(st)
				}
			}
		}
	}
	sort.Slice(m.items, func(i, j int) bool {
		return m.items[i].Item.Name < m.items[j].Item.Name
	})
	sort.Slice(classes, func(i, j int) bool {
		return classes[i] < classes[j]
	})
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] < slots[j]
	})
	sort.Slice(stats, func(i, j int) bool {
		return stats[i] < stats[j]
	})
	cl1 := m.class1.Value()
	cl2 := m.class2.Value()
	cl3 := m.class3.Value()
	m.class1.SetOptions(classes)
	m.class2.SetOptions(classes)
	m.class3.SetOptions(classes)
	m.class1.Select(cl1)
	m.class2.Select(cl2)
	m.class3.Select(cl3)

	sl := m.slots.Value()
	m.slots.SetOptions(slots)
	m.slots.Select(sl)

	selectedStat := m.stats.Value()
	m.stats.SetOptions(stats)
	m.stats.Select(selectedStat)
}

func (m *Module) AppendItem(item *inventory.Item, path ...string) ([]string, []string, []string) {
	loc := ""
	for _, name := range path {
		if loc != "" {
			loc = loc + " - "
		}
		loc += name
	}
	var id inventory.ItemData
	if item.Data != nil {
		id = *item.Data
		if item.Stats == nil {
			item.Stats = item.Data.GetStats(item.Level)
		}
	} else {
		id.Name = item.Name
		id.ID = item.Id
	}
	id.Name = item.Name
	if item.Level > 0 {
		id.Name = fmt.Sprintf("%s +%d", item.Name, item.Level)
	}
	id.Location = loc
	statNames := make([]string, 0, len(item.Stats))
	for stat := range item.Stats {
		statNames = append(statNames, stat)
	}
	if m.filter.Text() != "" && !strings.Contains(strings.ToUpper(id.Name), strings.ToUpper(m.filter.Text())) {
		return id.Classes, id.Slots, statNames
	}
	match := func() bool {
		if m.slots.Value() != "" {
			if len(id.Slots) == 0 {
				return false
			}
			for _, sl := range id.Slots {
				if sl == m.slots.Value() {
					return true
				}
			}
			return false
		}
		return true
	}()
	if !match {
		return id.Classes, id.Slots, statNames
	}
	if selectedStat := m.stats.Value(); selectedStat != "" {
		if _, found := item.Stats[selectedStat]; !found {
			return id.Classes, id.Slots, statNames
		}
	}

	classes := m.SelectedClasses()
	match = func() bool {
		if m.hide_exaltations {
			if strings.Contains(id.Name, "Exalt") || strings.Contains(id.Name, "Ornamentation") {
				return false
			}
		}
		if len(classes) == 0 {
			return true
		}
		if len(id.Classes) == 0 {
			return false
		}
		for _, cl := range id.Classes {
			if cl == "ALL" {
				return true
			}
			if slices.Contains(classes, cl) {
				return true
			}
		}
		return false
	}()
	if match {
		m.items = append(m.items, ClickableItems{Item: id, Stats: item.Stats})
	}
	return id.Classes, id.Slots, statNames
}
func (m *Module) SelectedClasses() []string {
	cl := []string{}
	selects := []*form.SelectBox{m.class1, m.class2, m.class3}
	for _, s := range selects {
		if s.Value() != "" && !slices.Contains(cl, s.Value()) {
			cl = append(cl, s.Value())
		}
	}
	return cl
}
