package main

import (
	"testing"

	"github.com/uija/eqdps/internal/skyquest"
)

func TestSkyRowsIncludeReadyQuestAndRequirements(t *testing.T) {
	quest := skyquest.Quest{
		Name:       "Bard Test of Voice",
		QuestGiver: "Aira",
		Rewards:    []string{"Songblade"},
		Requirements: []skyquest.Requirement{{
			Name: "Wind Rune", Quantity: 1, Kind: "rune",
		}},
	}
	shell := shell{
		skyProgress:  []skyquest.QuestProgress{{Class: "Bard", Quest: quest, Ready: true}},
		skyInventory: map[string]int{"Wind Rune": 1},
	}
	shell.rebuildSkyRows()
	if shell.skyReadyCount() != 1 {
		t.Fatalf("expected one ready quest, got %d", shell.skyReadyCount())
	}
	foundQuest, foundRequirement := false, false
	for _, row := range shell.skyRows {
		foundQuest = foundQuest || row.kind == "quest" && row.status == "READY" && row.detail == "Aira — Reward: Songblade"
		foundRequirement = foundRequirement || row.kind == "requirement" && row.have == "1" && row.need == "1"
	}
	if !foundQuest || !foundRequirement {
		t.Fatalf("missing ready quest rows: %#v", shell.skyRows)
	}
}

func TestReadyToTurnInSectionCanCollapse(t *testing.T) {
	quest := skyquest.Quest{Name: "Bard Test of Voice", Requirements: []skyquest.Requirement{{Name: "Owned", Quantity: 1}}}
	shell := shell{
		skyReadyCollapsed: true,
		skyProgress:       []skyquest.QuestProgress{{Class: "Bard", Quest: quest, Ready: true}},
		skyInventory:      map[string]int{"Owned": 1},
	}

	shell.rebuildSkyRows()
	if len(shell.skyRows) == 0 || shell.skyRows[0].kind != "ready-section" || shell.skyRows[0].name != "READY TO TURN IN (1)" || shell.skyRows[0].status != "SHOW" {
		t.Fatalf("unexpected ready header: %#v", shell.skyRows)
	}
	if len(shell.skyRows) < 2 || shell.skyRows[1].kind != "spacer" {
		t.Fatalf("collapsed ready summary contains rows below its header: %#v", shell.skyRows)
	}
}

func TestWatchedQuestsAppearBelowReadySection(t *testing.T) {
	quest := skyquest.Quest{Name: "Bard Test of Voice", Requirements: []skyquest.Requirement{{Name: "Missing", Quantity: 1}}}
	shell := shell{
		skyProgress:  []skyquest.QuestProgress{{Class: "Bard", Quest: quest}},
		skyInventory: map[string]int{},
		skyWatched:   map[string]bool{"Bard Test of Voice": true},
	}

	shell.rebuildSkyRows()
	watchedHeader := -1
	for index, row := range shell.skyRows {
		if row.kind == "watched-section" && row.name == "WATCHED (1)" {
			watchedHeader = index
			break
		}
	}
	if watchedHeader < 0 || watchedHeader+1 >= len(shell.skyRows) {
		t.Fatalf("watched section missing: %#v", shell.skyRows)
	}
	row := shell.skyRows[watchedHeader+1]
	if row.kind != "quest" || row.name != "Bard — Test of Voice" || !row.watched || row.watchClick == nil {
		t.Fatalf("unexpected watched quest row: %#v", row)
	}
}

func TestWatchedSectionCanCollapse(t *testing.T) {
	quest := skyquest.Quest{Name: "Bard Test of Voice", Requirements: []skyquest.Requirement{{Name: "Missing", Quantity: 1}}}
	shell := shell{
		skyWatchClosed: true,
		skyProgress:    []skyquest.QuestProgress{{Class: "Bard", Quest: quest}},
		skyWatched:     map[string]bool{"Bard Test of Voice": true},
	}

	shell.rebuildSkyRows()
	for index, row := range shell.skyRows {
		if row.kind == "watched-section" {
			if row.status != "SHOW" || row.toggleClick == nil {
				t.Fatalf("unexpected watched header: %#v", row)
			}
			if index+1 < len(shell.skyRows) && shell.skyRows[index+1].kind == "quest" {
				t.Fatalf("collapsed watched section contains quest rows: %#v", shell.skyRows)
			}
			return
		}
	}
	t.Fatal("watched header missing")
}

func TestClassSectionCanCollapse(t *testing.T) {
	quest := skyquest.Quest{Name: "Bard Test of Voice", Requirements: []skyquest.Requirement{{Name: "Missing", Quantity: 1}}}
	shell := shell{
		skyProgress:    []skyquest.QuestProgress{{Class: "Bard", Quest: quest}},
		skyClassClosed: map[string]bool{"Bard": true},
	}

	shell.rebuildSkyRows()
	for index, row := range shell.skyRows {
		if row.kind == "class" && row.name == "Bard" {
			if row.status != "0/1 done · 0 ready · SHOW" || row.toggleClick == nil {
				t.Fatalf("unexpected class header: %#v", row)
			}
			if index+1 < len(shell.skyRows) && shell.skyRows[index+1].kind == "quest" {
				t.Fatalf("collapsed class contains quest rows: %#v", shell.skyRows)
			}
			return
		}
	}
	t.Fatal("class header missing")
}

func TestCompletedClassStartsCollapsed(t *testing.T) {
	quest := func(name string) skyquest.Quest {
		return skyquest.Quest{Name: name, Requirements: []skyquest.Requirement{{Name: "Spent", Quantity: 1}}}
	}
	shell := shell{skyProgress: []skyquest.QuestProgress{
		{Class: "Bard", Quest: quest("Bard Test One"), Completed: true},
		{Class: "Bard", Quest: quest("Bard Test Two"), Completed: true},
	}}

	shell.rebuildSkyRows()
	for index, row := range shell.skyRows {
		if row.kind == "class" && row.name == "Bard" {
			if row.status != "2/2 done · 0 ready · SHOW" || !shell.skyClassClosed["Bard"] {
				t.Fatalf("completed class did not start collapsed: row=%#v state=%#v", row, shell.skyClassClosed)
			}
			if index+1 < len(shell.skyRows) && shell.skyRows[index+1].kind == "quest" {
				t.Fatalf("completed class contains quest rows: %#v", shell.skyRows)
			}
			return
		}
	}
	t.Fatal("class header missing")
}

func TestSkyHideEmptyKeepsCompletedAndStartedQuests(t *testing.T) {
	quest := func(name, item string) skyquest.Quest {
		return skyquest.Quest{Name: name, Requirements: []skyquest.Requirement{{Name: item, Quantity: 1}}}
	}
	shell := shell{
		skyHideEmpty: true,
		skyProgress: []skyquest.QuestProgress{
			{Class: "Bard", Quest: quest("Bard Started", "Owned")},
			{Class: "Bard", Quest: quest("Bard Empty", "Missing")},
			{Class: "Bard", Quest: quest("Bard Done", "Spent"), Completed: true},
		},
		skyInventory: map[string]int{"Owned": 1},
	}
	shell.rebuildSkyRows()
	quests := 0
	for _, row := range shell.skyRows {
		if row.kind == "quest" {
			quests++
		}
	}
	if quests != 2 {
		t.Fatalf("expected started and completed quests only, got %#v", shell.skyRows)
	}
}

func TestSkyHideEmptyDoesNotTreatWindRunesAsStartedQuestItems(t *testing.T) {
	quest := func(name, item string) skyquest.Quest {
		return skyquest.Quest{Name: name, Requirements: []skyquest.Requirement{
			{Name: "Wind Rune Caza", Kind: "rune", Quantity: 1},
			{Name: item, Kind: "item", Quantity: 1},
		}}
	}
	shell := shell{
		skyHideEmpty: true,
		skyProgress: []skyquest.QuestProgress{
			{Class: "Bard", Quest: quest("Bard Rune Only", "Missing")},
			{Class: "Bard", Quest: quest("Bard With Item", "Owned")},
		},
		skyInventory: map[string]int{"Wind Rune Caza": 4, "Owned": 1},
	}

	shell.rebuildSkyRows()
	var quests []string
	for _, row := range shell.skyRows {
		if row.kind == "quest" {
			quests = append(quests, row.name)
		}
	}
	if len(quests) != 1 || quests[0] != "With Item" {
		t.Fatalf("visible quests = %#v, want only quest with non-rune item", quests)
	}
}

func TestSkyHideFinishedRemovesCompletedQuestsFromAllSections(t *testing.T) {
	quest := func(name string) skyquest.Quest {
		return skyquest.Quest{Name: name, Requirements: []skyquest.Requirement{{Name: "Item", Quantity: 1}}}
	}
	shell := shell{
		skyHideFinished: true,
		skyWatched:      map[string]bool{"Bard Done": true, "Bard Open": true},
		skyProgress: []skyquest.QuestProgress{
			{Class: "Bard", Quest: quest("Bard Done"), Completed: true},
			{Class: "Bard", Quest: quest("Bard Open")},
		},
	}

	shell.rebuildSkyRows()
	doneRows, openRows := 0, 0
	for _, row := range shell.skyRows {
		if row.kind != "quest" {
			continue
		}
		switch row.questName {
		case "Bard Done":
			doneRows++
		case "Bard Open":
			openRows++
		}
	}
	if doneRows != 0 || openRows != 2 {
		t.Fatalf("completed rows = %d, open rows = %d; rows: %#v", doneRows, openRows, shell.skyRows)
	}
}

func TestSkyRequirementSourceUsesSpecificDrop(t *testing.T) {
	got := skyRequirementSource(skyquest.Requirement{Island: 5, DropsFrom: "Protector of Sky"})
	if got != "Island 5 — Protector of Sky" {
		t.Fatalf("unexpected source: %q", got)
	}
}

func TestSkyRewardURL(t *testing.T) {
	if got := skyRewardURL("Mask of Song"); got != "https://eqlwiki.com/Mask_of_Song" {
		t.Fatalf("unexpected reward URL: %q", got)
	}
}
