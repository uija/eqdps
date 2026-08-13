package dps

import "testing"

func TestGetActiveFightLegacyKeepsDamageWithSelectedMob(t *testing.T) {
	combat := newCombat()

	heartHarpie := combat.getActiveFightLegacy("heart harpie", "you")
	spirocLord := combat.getActiveFightLegacy("you", "the spiroc lord")

	if got := combat.getActiveFightLegacy("heart harpie", "the spiroc lord"); got != spirocLord {
		t.Fatal("damage to an active mob was assigned to the charmed pet's fight")
	}
	if got := combat.getActiveFightLegacy("the spiroc lord", "heart harpie"); got != heartHarpie {
		t.Fatal("damage to the active charmed pet was not assigned to its fight")
	}
}

func TestGetActiveFightLegacyDoesNotReuseParticipantFight(t *testing.T) {
	combat := newCombat()

	heartHarpie := combat.getActiveFightLegacy("heart harpie", "you")
	heartHarpie.participants["a spiroc guardian"] = true

	guardian := combat.getActiveFightLegacy("you", "a spiroc guardian")
	if guardian == heartHarpie {
		t.Fatal("participant lookup reused the charmed pet's fight")
	}
	if guardian.name != "a spiroc guardian" {
		t.Fatalf("fight name = %q, want %q", guardian.name, "a spiroc guardian")
	}
}

func TestGetActiveFightLegacyUsesLegacyPetIdentity(t *testing.T) {
	combat := newCombat()

	owner := combat.getActiveFightLegacy("you", "sobatin")
	if got := combat.getActiveFightLegacy("you", "sobatin`s warder"); got != owner {
		t.Fatal("possessive pet was not assigned to its active owner's fight")
	}
	if got := combat.getActiveFightLegacy("you", "hoptor thaggelum pet"); got.name != "hoptor thaggelum" {
		t.Fatalf("pet-suffix fight name = %q, want %q", got.name, "hoptor thaggelum")
	}
}
