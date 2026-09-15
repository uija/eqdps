package data

type HealingData struct {
	Name   string
	Amount int64
	Count  int64
	Crit   int64
	Min    int64
	Max    int64
}
type HealingCategory struct {
	Overall   *HealingData
	Abilities map[string]*HealingData
}

func NewHealingData(name string) *HealingData {
	return &HealingData{
		Name: name,
	}
}
func NewHealingCategory(name string) HealingCategory {
	return HealingCategory{
		Overall:   NewHealingData(name),
		Abilities: make(map[string]*HealingData),
	}
}

func (d *HealingData) AddHealing(amount int64, crit bool) {
	d.Amount += amount
	d.Count++
	if crit {
		d.Crit++
	}
	if d.Min == 0 || amount < d.Min {
		d.Min = amount
	}
	if amount > d.Max {
		d.Max = amount
	}
}
func (c HealingCategory) AddHealing(ability string, amount int64, crit bool) {
	c.Overall.AddHealing(amount, crit)
	if c.Abilities[ability] == nil {
		c.Abilities[ability] = NewHealingData(ability)
	}
	c.Abilities[ability].AddHealing(amount, crit)
}
