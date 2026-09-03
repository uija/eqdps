package achievments

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Achievements.txt")
	contents := strings.Join([]string{
		"General: Level",
		"C\tLevel 5",
		"C\t\tReach Level 5",
		"Slayer: Conquest",
		"I\tDoesn't Play Well With Others",
		"I\t\tThe playable races.\t2254/10000",
	}, "\r\n")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categories) != 2 {
		t.Fatalf("got %d categories, want 2", len(result.Categories))
	}
	level := result.Categories[0].Subcategories[0].Achievements[0]
	if level.Name != "Level 5" || !level.Complete || len(level.Objectives) != 1 || !level.Objectives[0].Complete {
		t.Fatalf("unexpected completed achievement: %#v", level)
	}
	conquest := result.Categories[1].Subcategories[0].Achievements[0]
	if conquest.Complete || conquest.Objectives[0].Progress == nil {
		t.Fatalf("unexpected incomplete achievement: %#v", conquest)
	}
	if got, want := conquest.Objectives[0].Progress.Current, 2254; got != want {
		t.Fatalf("current progress = %d, want %d", got, want)
	}
	if got, want := conquest.Objectives[0].Progress.Required, 10000; got != want {
		t.Fatalf("required progress = %d, want %d", got, want)
	}

	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("marshal parsed export: %v", err)
	}
}

func TestParseRejectsObjectiveWithoutAchievement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Achievements.txt")
	if err := os.WriteFile(path, []byte("General: Level\nC\t\tReach Level 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Parse(path)
	if err == nil || !strings.Contains(err.Error(), "objective has no achievement") {
		t.Fatalf("Parse error = %v", err)
	}
}
