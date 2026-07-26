package main

import "testing"

func TestCancelCombatOperation(t *testing.T) {
	cancel := make(chan struct{})
	shell := shell{loading: true, parserState: "loading", logCancel: cancel}
	shell.cancelCurrentOperation()
	if shell.loading || shell.parserState != "" || shell.logCancel != nil {
		t.Fatalf("combat operation was not cleared: loading=%v state=%q cancel-present=%v", shell.loading, shell.parserState, shell.logCancel != nil)
	}
	select {
	case <-cancel:
	default:
		t.Fatal("combat cancellation channel was not closed")
	}
}

func TestCancelSkyOperation(t *testing.T) {
	cancel := make(chan struct{})
	shell := shell{skyLoading: true, skyCancel: cancel}
	shell.cancelCurrentOperation()
	if shell.skyLoading || shell.skyCancel != nil {
		t.Fatalf("Plane of Sky operation was not cleared: loading=%v cancel-present=%v", shell.skyLoading, shell.skyCancel != nil)
	}
	select {
	case <-cancel:
	default:
		t.Fatal("Plane of Sky cancellation channel was not closed")
	}
}

func TestSetSubtreeExpanded(t *testing.T) {
	shell := shell{
		expanded: map[string]bool{},
		treeChildren: map[string][]string{
			"combatant": {"melee", "magic"},
			"melee":     {"slashes"},
		},
	}

	shell.setSubtreeExpanded("combatant", true)
	for _, key := range []string{"combatant", "melee", "magic", "slashes"} {
		if !shell.expanded[key] {
			t.Errorf("expected %q to be expanded", key)
		}
	}

	shell.setSubtreeExpanded("melee", false)
	if shell.expanded["melee"] || shell.expanded["slashes"] {
		t.Fatal("collapsing a category did not collapse its descendants")
	}
	if !shell.expanded["combatant"] || !shell.expanded["magic"] {
		t.Fatal("collapsing a category changed nodes outside its subtree")
	}
}
