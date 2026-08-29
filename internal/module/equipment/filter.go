package equipment

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/uija/eqdps/internal/inventory"
	"github.com/uija/eqdps/internal/ui/form"
)

func (m *Module) PrepareItems() {
	classes := []string{""}
	slots := []string{""}
	m.items = make([]ClickableItems, 0)

	addClasses := func(list []string) {
		if list != nil {
			for _, cl := range list {
				cl = strings.ToUpper(cl)
				if len(cl) == 3 && cl != "ALL" && !slices.Contains(classes, cl) {
					classes = append(classes, cl)
				}
			}
		}
	}
	addSlots := func(list []string) {
		if list != nil {
			for _, cl := range list {
				cl = strings.ToUpper(cl)
				if len(cl) > 0 && cl != "ALL" && !slices.Contains(slots, cl) {
					slots = append(slots, cl)
				}
			}
		}
	}

	for storage, s := range m.inv.Storage {
		for root, r := range s.Slots {
			cl, sl := m.AppendItem(&r, storage, root)
			addClasses(cl)
			addSlots(sl)
			for item, i := range r.Slots {
				if r.IsBag {
					cl, sl = m.AppendItem(&i, storage, root, item)
				} else {
					cl, sl = m.AppendItem(&i, storage, root, r.Name)
				}
				addClasses(cl)
				addSlots(sl)
				for aug, a := range i.Slots {
					if i.IsBag {
						cl, sl = m.AppendItem(&a, storage, root, item, aug)
					} else {
						cl, sl = m.AppendItem(&a, storage, root, item, i.Name)
					}
					addClasses(cl)
					addSlots(sl)
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
}

func (m *Module) AppendItem(item *inventory.Item, path ...string) ([]string, []string) {
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
	} else {
		id.Name = item.Name
		id.ID = item.Id
	}
	id.Name = item.Name
	if item.Level > 0 {
		id.Name = fmt.Sprintf("%s +%d", item.Name, item.Level)
	}
	id.Location = loc
	if m.filter.Text() != "" && !strings.Contains(strings.ToUpper(id.Name), strings.ToUpper(m.filter.Text())) {
		return id.Classes, id.Slots
	}
	match := func() bool {
		if m.slots.Value() != "" {
			if id.Slots == nil || len(id.Slots) == 0 {
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
		return id.Classes, id.Slots
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
		if id.Classes == nil || len(id.Classes) == 0 {
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
		m.items = append(m.items, ClickableItems{Item: id})
	}
	return id.Classes, id.Slots
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
