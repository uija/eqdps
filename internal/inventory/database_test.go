package inventory

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestMetadataClasses(t *testing.T) {
	tests := []struct {
		name       string
		statsblock string
		want       []string
	}{
		{
			name:       "listed classes",
			statsblock: "Slot: SHOULDERS<br>Class: WAR PAL RNG<br>",
			want:       []string{"WAR", "PAL", "RNG"},
		},
		{
			name:       "all classes",
			statsblock: "Slot: SHOULDERS<br>Class: ALL<br>",
			want:       []string{"ALL"},
		},
		{
			name:       "all except casters",
			statsblock: "Slot: SHOULDERS<br>Class: ALL except NEC WIZ MAG ENC<br>",
			want:       []string{"WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD", "ROG", "SHM", "BST", "BER"},
		},
		{
			name:       "all except without exclusions",
			statsblock: "Slot: PRIMARY<br>Class: ALL except<br>",
			want:       []string{"ALL"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := map[string]any{"statsblock": test.statsblock}
			if got := metadataClasses(metadata); !slices.Equal(got, test.want) {
				t.Fatalf("metadataClasses() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestItemDataNormalizesFingerSlotFromMetadata(t *testing.T) {
	item := itemDataFromMetadata("1", map[string]any{
		"itemname": "Odd Ring",
		"slots":    []any{"FINGERS", "NECK"},
	})
	if want := []string{"FINGER", "NECK"}; !slices.Equal(item.Slots, want) {
		t.Fatalf("itemDataFromMetadata().Slots = %v, want %v", item.Slots, want)
	}
}

func TestItemDataNormalizesFingerSlotFromJSON(t *testing.T) {
	var item ItemData
	if err := json.Unmarshal([]byte(`{"Slots":["FINGERS"]}`), &item); err != nil {
		t.Fatalf("Unmarshal ItemData: %v", err)
	}
	if want := []string{"FINGER"}; !slices.Equal(item.Slots, want) {
		t.Fatalf("Unmarshal ItemData Slots = %v, want %v", item.Slots, want)
	}
}
