package skyquest

import "testing"

func TestEmbeddedDatabaseHasCompleteClassQuestTables(t *testing.T) {
	database, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(database.Classes), 16; got != want {
		t.Fatalf("classes = %d, want %d", got, want)
	}

	quests := 0
	for _, class := range database.Classes {
		if len(class.Quests) == 0 {
			t.Fatalf("class %q has no quests", class.Name)
		}
		quests += len(class.Quests)
		for _, quest := range class.Quests {
			if class.Name == "Necromancer" && quest.QuestGiver != "Drakis Bloodcaster" {
				t.Errorf("Necromancer quest %q giver = %q, want Drakis Bloodcaster", quest.Name, quest.QuestGiver)
			}
		}
	}
	if got, want := quests, 95; got != want {
		t.Fatalf("quests = %d, want %d", got, want)
	}
}

func TestEmbeddedDatabaseRecognizesRunesObservedInReferenceLog(t *testing.T) {
	database, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"Wind Rune Caza": false,
		"Wind Rune Ena":  false,
		"Wind Rune Fana": false,
		"Wind Rune Geza": false,
	}
	for _, class := range database.Classes {
		for _, quest := range class.Quests {
			for _, requirement := range quest.Requirements {
				if _, ok := want[requirement.Name]; ok {
					want[requirement.Name] = true
				}
			}
		}
	}
	for item, found := range want {
		if !found {
			t.Errorf("reference-log item %q is absent from database", item)
		}
	}
}
