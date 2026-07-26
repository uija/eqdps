package audio

import "testing"

func TestEmbeddedSounds(t *testing.T) {
	sounds, err := EmbeddedSounds()
	if err != nil {
		t.Fatal(err)
	}
	if len(sounds) == 0 {
		t.Fatal("no embedded sounds found")
	}
	for _, sound := range sounds {
		if _, err := ReadEmbedded(sound.ID); err != nil {
			t.Fatalf("read %q: %v", sound.ID, err)
		}
	}
}
