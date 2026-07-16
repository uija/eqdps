package skyquest

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

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
			for _, requirement := range quest.Requirements {
				if requirement.Kind != "rune" && requirement.DropsFrom == "" {
					t.Errorf("requirement %q in %q has no drop source", requirement.Name, quest.Name)
				}
			}
		}
	}
	if got, want := quests, 95; got != want {
		t.Fatalf("quests = %d, want %d", got, want)
	}
}

func TestQuestGiverAndRequirementsUniquelyIdentifyEveryQuest(t *testing.T) {
	database, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]string)
	for _, class := range database.Classes {
		for _, quest := range class.Quests {
			items := make([]string, 0, len(quest.Requirements))
			for _, requirement := range quest.Requirements {
				items = append(items, fmt.Sprintf("%d:%s", requirement.Quantity, requirement.Name))
			}
			sort.Strings(items)
			key := quest.QuestGiver + "\x00" + strings.Join(items, "\x00")
			if previous, exists := seen[key]; exists {
				t.Errorf("quests %q and %q share giver and requirements", previous, quest.Name)
			}
			seen[key] = quest.Name
		}
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
