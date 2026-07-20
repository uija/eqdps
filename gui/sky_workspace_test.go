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

func TestSkyRequirementSourceUsesSpecificDrop(t *testing.T) {
	got := skyRequirementSource(skyquest.Requirement{Island: 5, DropsFrom: "Protector of Sky"})
	if got != "Island 5 — Protector of Sky" {
		t.Fatalf("unexpected source: %q", got)
	}
}
