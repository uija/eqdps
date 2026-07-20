package main

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/engine"
	"github.com/uija/eqdps/internal/skyquest"
)

const skyProgressThreshold = 5 * 1024 * 1024

type skyAsyncUpdate struct {
	path     string
	progress *engine.ReplayProgress
	tracker  *skyquest.PersistentTracker
	message  string
	err      error
	done     bool
	live     bool
}

func (s *shell) startSkyForLog(logPath string) {
	if s.skyCancel != nil {
		close(s.skyCancel)
		s.skyCancel = nil
	}
	s.skyMu.Lock()
	s.skyTracker = nil
	s.skyMu.Unlock()
	s.skySetupOpen = false
	s.skyLoading = false
	if logPath == "" || s.skyDenied {
		return
	}
	character, server, err := skyquest.CharacterIdentity(logPath)
	if err != nil {
		s.skyLoading = false
		s.skyMessage = err.Error()
		return
	}
	s.skyIdentity = character + " / " + server
	exists, err := skyquest.StateExists(logPath)
	if err != nil {
		s.skyMessage = err.Error()
		return
	}
	if !exists {
		s.skyMessage = "Plane of Sky tracking has not been enabled for this character."
		s.skySetupOpen = true
		return
	}
	tracker, err := skyquest.LoadPersistentTracker(logPath, s.skyDatabase)
	if err != nil {
		s.skyMessage = err.Error()
		return
	}
	s.applySkySnapshot(tracker, "Loaded saved Plane of Sky state; checking for missed activity…")
	s.startSkyCatchup(logPath, tracker)
}

func (s *shell) startSkyCatchup(logPath string, tracker *skyquest.PersistentTracker) {
	info, err := os.Stat(logPath)
	if err != nil {
		s.skyMessage = err.Error()
		return
	}
	target, start := info.Size(), tracker.Offset()
	cancel := make(chan struct{})
	s.skyCancel = cancel
	if target-start >= skyProgressThreshold {
		s.skyLoading = true
		s.skyLoadTitle = "Catching up Plane of Sky tracker…"
		s.skyLoadBytes, s.skyLoadTotal, s.skyLoadLines = 0, target-start, 0
	}
	go func() {
		err := tracker.SyncLogWithProgress(logPath, target, func(progress skyquest.ScanProgress) {
			processed := progress.Bytes - start
			if processed < 0 {
				processed = 0
			}
			s.sendSkyUpdate(skyAsyncUpdate{path: logPath, progress: &engine.ReplayProgress{Bytes: processed, Total: target - start, Lines: progress.Lines}})
		}, cancel)
		if errors.Is(err, skyquest.ErrScanCancelled) {
			return
		}
		if err == nil {
			err = s.activateSkyTracker(logPath, tracker)
		}
		s.sendSkyUpdate(skyAsyncUpdate{path: logPath, tracker: tracker, message: "Plane of Sky tracker is caught up and following live activity.", err: err, done: true})
	}()
}

func (s *shell) startSkyInitialScan() {
	logPath := s.currentLog
	info, err := os.Stat(logPath)
	if err != nil {
		s.skyMessage = err.Error()
		return
	}
	cancel := make(chan struct{})
	s.skyCancel = cancel
	s.skyLoading = true
	s.skyLoadTitle = "Scanning existing Plane of Sky loot history…"
	s.skyLoadBytes, s.skyLoadTotal, s.skyLoadLines = 0, info.Size(), 0
	go func(target int64) {
		tracker, err := skyquest.InitializePersistentTracker(logPath, s.skyDatabase, target, func(progress skyquest.ScanProgress) {
			s.sendSkyUpdate(skyAsyncUpdate{path: logPath, progress: &engine.ReplayProgress{Bytes: progress.Bytes, Total: progress.Total, Lines: progress.Lines}})
		}, cancel)
		if errors.Is(err, skyquest.ErrScanCancelled) {
			return
		}
		if err == nil {
			err = s.activateSkyTracker(logPath, tracker)
		}
		s.sendSkyUpdate(skyAsyncUpdate{path: logPath, tracker: tracker, message: "Plane of Sky tracking is enabled and following live activity.", err: err, done: true})
	}(info.Size())
}

func (s *shell) activateSkyTracker(logPath string, tracker *skyquest.PersistentTracker) error {
	s.skyMu.Lock()
	defer s.skyMu.Unlock()
	s.skyTracker = tracker
	return tracker.SyncLog(logPath)
}

func (s *shell) processSkyLine(logPath, line string, endOffset int64) {
	s.skyMu.RLock()
	defer s.skyMu.RUnlock()
	if s.skyTracker == nil {
		return
	}
	if err := s.skyTracker.ProcessLine(line, endOffset); err != nil {
		s.sendSkyUpdate(skyAsyncUpdate{path: logPath, err: err})
		return
	}
	s.sendSkyUpdate(skyAsyncUpdate{path: logPath, tracker: s.skyTracker, live: true})
}

func (s *shell) sendSkyUpdate(update skyAsyncUpdate) {
	select {
	case s.skyUpdates <- update:
	default:
		select {
		case <-s.skyUpdates:
		default:
		}
		s.skyUpdates <- update
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

func (s *shell) applySkyAsyncUpdate(update skyAsyncUpdate) {
	if update.path != s.currentLog {
		return
	}
	if update.progress != nil {
		s.skyLoadBytes, s.skyLoadTotal, s.skyLoadLines = update.progress.Bytes, update.progress.Total, update.progress.Lines
	}
	if update.done {
		s.skyLoading = false
	}
	if update.err != nil {
		s.skyLoading = false
		s.skyMessage = update.err.Error()
		return
	}
	if update.tracker != nil {
		before := s.skyReadyQuestKeys()
		s.applySkySnapshot(update.tracker, update.message)
		if update.live {
			s.notifyNewReadyQuests(before)
		}
	} else if update.message != "" {
		s.skyMessage = update.message
	}
}

func (s *shell) skyReadyQuestKeys() map[string]struct{} {
	ready := make(map[string]struct{})
	for _, progress := range s.skyProgress {
		if progress.Ready {
			ready[progress.Class+"\x00"+progress.Quest.Name] = struct{}{}
		}
	}
	return ready
}

func (s *shell) notifyNewReadyQuests(before map[string]struct{}) {
	newReady := 0
	for key := range s.skyReadyQuestKeys() {
		if _, existed := before[key]; !existed {
			newReady++
		}
	}
	if newReady == 0 {
		return
	}
	label := "New turn-in available"
	if newReady > 1 {
		label = fmt.Sprintf("%d new turn-ins available", newReady)
	}
	s.skyNoticeText = fmt.Sprintf("PoS: %d ready · %s", s.skyReadyCount(), label)
	s.skyNoticeUntil = time.Now().Add(8 * time.Second)
}

func (s *shell) applySkySnapshot(tracker *skyquest.PersistentTracker, message string) {
	s.skyProgress = tracker.QuestProgress()
	s.skyInventory = tracker.Inventory()
	if message != "" {
		s.skyMessage = message
	}
	s.rebuildSkyRows()
}

func (s *shell) layoutSkySetup(gtx layout.Context) layout.Dimensions {
	if !s.skySetupOpen {
		return layout.Dimensions{}
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(620)), gtx.Dp(unit.Dp(310)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				info, _ := os.Stat(s.currentLog)
				size := int64(0)
				if info != nil {
					size = info.Size()
				}
				message := fmt.Sprintf("To determine which quest items you already own, eqdps needs to scan this logfile once.\n\nLog: %s\nSize: %s\n\nNo character state file is created if you choose Not now.", filepath.Base(s.currentLog), formatSkyBytes(size))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return labelWeight(gtx, s.theme, "Enable Plane of Sky Quest Tracker", unit.Sp(21), palette.text, text.Start, font.SemiBold)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return inset(0, unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return label(gtx, s.theme, message, unit.Sp(15), palette.text, text.Start)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return skySetupButton(gtx, s.theme, &s.skyDeny, "Not now", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return skySetupButton(gtx, s.theme, &s.skyAllow, "Enable and scan", true)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func skySetupButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, accent bool) layout.Dimensions {
	return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			fill(gtx, palette.panelAlt)
			foreground := palette.text
			if accent {
				foreground = palette.accent
			}
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return labelWeight(gtx, theme, value, unit.Sp(15), foreground, text.Middle, font.SemiBold)
			})
		})
	})
}

func formatSkyBytes(bytes int64) string {
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
}
