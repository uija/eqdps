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
