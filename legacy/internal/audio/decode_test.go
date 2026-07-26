package audio

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeEmbeddedMP3(t *testing.T) {
	data, err := ReadEmbedded(EmbeddedPrefix + "Chord.mp3")
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := decodeMP3(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("decoded PCM has invalid length %d", len(pcm))
	}
}

func TestDecodeFloatMonoWAV(t *testing.T) {
	samples := []float32{-1, -0.5, 0, 0.5, 1}
	dataSize := len(samples) * 4
	wav := make([]byte, 44+dataSize)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 3)
	binary.LittleEndian.PutUint16(wav[22:24], 1)
	binary.LittleEndian.PutUint32(wav[24:28], 44100)
	binary.LittleEndian.PutUint32(wav[28:32], 44100*4)
	binary.LittleEndian.PutUint16(wav[32:34], 4)
	binary.LittleEndian.PutUint16(wav[34:36], 32)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))
	for i, sample := range samples {
		binary.LittleEndian.PutUint32(wav[44+i*4:], math.Float32bits(sample))
	}

	pcm, err := decodeWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(pcm), len(samples)*4; got != want {
		t.Fatalf("decoded PCM length = %d, want %d", got, want)
	}
	for frame := range samples {
		left := int16(binary.LittleEndian.Uint16(pcm[frame*4:]))
		right := int16(binary.LittleEndian.Uint16(pcm[frame*4+2:]))
		if left != right {
			t.Fatalf("frame %d was not duplicated to stereo: %d != %d", frame, left, right)
		}
	}
}
