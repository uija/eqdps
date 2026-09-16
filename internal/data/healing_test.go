package data

import "testing"

func TestFightUnnamedHealing(t *testing.T) {
	f := NewFight(true)
	f.AddHealing("You", "Wyrmberg", "", 33, false, false)
	f.AddHealing("You", "Wyrmberg", "", 134, false, true)
	f.AddHealing("You", "Wyrmberg", "Minor Healing II", 50, false, false)
	abilities := f.Combatants["you"].HealingCategories[DIRECT_HEAL].Abilities
	unknown := abilities["Unknown"]
	if unknown == nil || unknown.Amount != 167 || unknown.Count != 2 || unknown.Crit != 1 {
		t.Fatalf("incorrect unnamed healing: %+v", unknown)
	}
	if abilities[""] != nil || abilities["Minor Healing II"] == nil {
		t.Fatal("empty or ranked healing name handled incorrectly")
	}
}
