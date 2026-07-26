package audio

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

type Playback struct {
	context  *oto.Context
	ready    <-chan struct{}
	audioDir string
	cache    map[string][]byte
	mu       sync.Mutex
}

func NewPlayback(audioDir string) (*Playback, error) {
	audioContext, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   outputSampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   20 * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize audio: %w", err)
	}
	return &Playback{
		context:  audioContext,
		ready:    ready,
		audioDir: audioDir,
		cache:    make(map[string][]byte),
	}, nil
}

func (p *Playback) Play(ctx context.Context, id string, volume float64, reportError func(error)) error {
	select {
	case <-ctx.Done():
		return nil
	case <-p.ready:
	}

	p.mu.Lock()
	pcm, ok := p.cache[id]
	if !ok {
		var err error
		pcm, err = loadPCM(id, p.audioDir)
		if err != nil {
			p.mu.Unlock()
			return err
		}
		p.cache[id] = pcm
	}
	p.mu.Unlock()

	player := p.context.NewPlayer(bytes.NewReader(pcm))
	player.SetBufferSize(4096)
	player.SetVolume(volume)
	player.Play()
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for player.IsPlaying() {
			select {
			case <-ctx.Done():
				player.Pause()
				return
			case <-ticker.C:
			}
		}
		if err := player.Err(); err != nil {
			reportError(err)
		}
	}()
	return nil
}
