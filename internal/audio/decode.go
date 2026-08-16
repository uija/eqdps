package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/go-mp3"
)

const (
	outputSampleRate = 44100
	maxDecodedBytes  = 64 << 20
)

func loadPCM(id, audioDir string) ([]byte, error) {
	var (
		data []byte
		name string
		err  error
	)
	switch {
	case strings.HasPrefix(id, EmbeddedPrefix):
		data, err = ReadEmbedded(id)
		name = strings.TrimPrefix(id, EmbeddedPrefix)
	case strings.HasPrefix(id, UserPrefix):
		name = strings.TrimPrefix(id, UserPrefix)
		if name == "" || filepath.Base(name) != name {
			return nil, fmt.Errorf("invalid user sound ID %q", id)
		}
		data, err = os.ReadFile(filepath.Join(audioDir, name))
	default:
		return nil, fmt.Errorf("invalid sound ID %q", id)
	}
	if err != nil {
		return nil, err
	}
	if len(data) > maxDecodedBytes {
		return nil, errors.New("sound file is too large")
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3":
		return decodeMP3(data)
	case ".wav":
		return decodeWAV(data)
	default:
		return nil, fmt.Errorf("unsupported sound format %q", filepath.Ext(name))
	}
}

func decodeMP3(data []byte) ([]byte, error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode MP3: %w", err)
	}
	decoded, err := io.ReadAll(io.LimitReader(decoder, maxDecodedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read decoded MP3: %w", err)
	}
	if len(decoded) > maxDecodedBytes {
		return nil, errors.New("decoded MP3 is too large")
	}
	if len(decoded)%4 != 0 {
		return nil, errors.New("decoded MP3 has an incomplete stereo frame")
	}
	if decoder.SampleRate() == outputSampleRate {
		return decoded, nil
	}
	return resamplePCM16Stereo(decoded, decoder.SampleRate()), nil
}

func decodeWAV(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("invalid WAV header")
	}

	var formatData, sampleData []byte
	for offset := 12; offset+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + size
		if end < start || end > len(data) {
			return nil, errors.New("invalid WAV chunk size")
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			formatData = data[start:end]
		case "data":
			sampleData = data[start:end]
		}
		offset = end + size%2
	}
	if len(formatData) < 16 {
		return nil, errors.New("WAV has no valid format chunk")
	}
	if sampleData == nil {
		return nil, errors.New("WAV has no data chunk")
	}

	format := binary.LittleEndian.Uint16(formatData[0:2])
	channels := int(binary.LittleEndian.Uint16(formatData[2:4]))
	sampleRate := int(binary.LittleEndian.Uint32(formatData[4:8]))
	blockAlign := int(binary.LittleEndian.Uint16(formatData[12:14]))
	bits := int(binary.LittleEndian.Uint16(formatData[14:16]))
	if format == 0xfffe && len(formatData) >= 26 {
		format = binary.LittleEndian.Uint16(formatData[24:26])
	}
	if channels < 1 || sampleRate < 1 || blockAlign < 1 {
		return nil, errors.New("WAV has an invalid audio format")
	}
	bytesPerSample := (bits + 7) / 8
	if bytesPerSample == 0 || channels*bytesPerSample > blockAlign {
		return nil, errors.New("WAV has an invalid block alignment")
	}

	frameCount := len(sampleData) / blockAlign
	outputFrames := (int64(frameCount)*outputSampleRate + int64(sampleRate) - 1) / int64(sampleRate)
	if outputFrames*4 > maxDecodedBytes {
		return nil, errors.New("decoded WAV is too large")
	}
	stereo := make([]float64, frameCount*2)
	for frame := range frameCount {
		frameData := sampleData[frame*blockAlign : (frame+1)*blockAlign]
		left, err := wavSample(frameData[:bytesPerSample], format, bits)
		if err != nil {
			return nil, err
		}
		right := left
		if channels > 1 {
			right, err = wavSample(frameData[bytesPerSample:2*bytesPerSample], format, bits)
			if err != nil {
				return nil, err
			}
		}
		stereo[frame*2] = left
		stereo[frame*2+1] = right
	}
	return encodeResampledStereo(stereo, sampleRate), nil
}

func wavSample(data []byte, format uint16, bits int) (float64, error) {
	switch format {
	case 1:
		switch bits {
		case 8:
			return (float64(data[0]) - 128) / 128, nil
		case 16:
			return float64(int16(binary.LittleEndian.Uint16(data))) / 32768, nil
		case 24:
			value := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16
			if value&0x800000 != 0 {
				value |= ^0xffffff
			}
			return float64(value) / 8388608, nil
		case 32:
			return float64(int32(binary.LittleEndian.Uint32(data))) / 2147483648, nil
		}
	case 3:
		switch bits {
		case 32:
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(data))), nil
		case 64:
			return math.Float64frombits(binary.LittleEndian.Uint64(data)), nil
		}
	}
	return 0, fmt.Errorf("unsupported WAV encoding %d with %d bits", format, bits)
}

func resamplePCM16Stereo(data []byte, sourceRate int) []byte {
	stereo := make([]float64, len(data)/2)
	for i := range stereo {
		stereo[i] = float64(int16(binary.LittleEndian.Uint16(data[i*2:]))) / 32768
	}
	return encodeResampledStereo(stereo, sourceRate)
}

func encodeResampledStereo(source []float64, sourceRate int) []byte {
	sourceFrames := len(source) / 2
	if sourceFrames == 0 {
		return nil
	}
	outputFrames := int(math.Round(float64(sourceFrames) * outputSampleRate / float64(sourceRate)))
	output := make([]byte, outputFrames*4)
	for frame := range outputFrames {
		position := float64(frame) * float64(sourceRate) / outputSampleRate
		before := min(int(position), sourceFrames-1)
		after := min(before+1, sourceFrames-1)
		fraction := position - float64(before)
		for channel := range 2 {
			sample := source[before*2+channel]*(1-fraction) + source[after*2+channel]*fraction
			if math.IsNaN(sample) {
				sample = 0
			}
			sample = max(-1, min(1, sample))
			value := int16(math.Round(sample * 32767))
			binary.LittleEndian.PutUint16(output[(frame*2+channel)*2:], uint16(value))
		}
	}
	return output
}
