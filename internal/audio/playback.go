package audio

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

type PlaybackRequest struct {
	id          string
	volume      float64
	errorReport func(error)
}

type Playback struct {
	context       *oto.Context
	ready         <-chan struct{}
	audioDir      string
	cache         map[string][]byte
	mu            sync.Mutex
	stop          chan struct{}
	playbackQueue chan PlaybackRequest
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
		context:       audioContext,
		ready:         ready,
		audioDir:      audioDir,
		cache:         make(map[string][]byte),
		stop:          make(chan struct{}, 1),
		playbackQueue: make(chan PlaybackRequest, 1),
	}, nil
}
func (p *Playback) RunAudioQueue() {
	ctx := context.Background()
	go func() {
		select {
		case <-p.ready:
		case <-p.stop:
			return
		}
		for {
			select {
			case request := <-p.playbackQueue:
				if err := p.PlayRequest(ctx, request); err != nil {
					request.errorReport(err)
				}
			case <-p.stop:
				return
			}
		}
	}()
}
func (p *Playback) Shutdown() {
	p.stop <- struct{}{}
}
func (p *Playback) AudioDir() string {
	return p.audioDir
}

func (p *Playback) Play(id string, volume float64, reportError func(error)) error {
	select {
	case p.playbackQueue <- PlaybackRequest{id: id, volume: volume, errorReport: reportError}:
	default:
		return fmt.Errorf("Sound queue is full.")
	}
	return nil
}
func (p *Playback) PlayRequest(ctx context.Context, pr PlaybackRequest) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	p.mu.Lock()
	pcm, ok := p.cache[pr.id]
	if !ok {
		var err error
		pcm, err = loadPCM(pr.id, p.audioDir)
		if err != nil {
			p.mu.Unlock()
			return err
		}
		p.cache[pr.id] = pcm
	}
	p.mu.Unlock()

	player := p.context.NewPlayer(bytes.NewReader(pcm))
	player.SetBufferSize(4096)
	player.SetVolume(pr.volume)
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
			pr.errorReport(err)
		}
	}()
	return nil
}
