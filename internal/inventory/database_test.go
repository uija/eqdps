package inventory

import (
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
