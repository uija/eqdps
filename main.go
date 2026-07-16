package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/skyquest"
	"github.com/uija/eqdps/internal/xp"
)

func main() {
	textMode := flag.Bool("text", false, "print the DPS table to stdout instead of opening the terminal UI")
	idleTimeout := flag.Duration("idle-timeout", combat.DefaultIdleTimeout, "end the current fight after this much time without combat")
	backMinutes := flag.Int("back", 0, "parse this many minutes before the current end of the log before tailing; 0 disables backfill")
	sinceText := flag.String("since", "", "parse from this log timestamp, format: YYYY-MM-DD HH:MM")
	historyLimit := flag.Int("history", combat.DefaultFightHistory, "completed fights to keep/show; 0 keeps all")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <everquest-log-file>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	logPath := flag.Arg(0)
	since, err := parseSince(*sinceText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	skyDatabase, err := skyquest.LoadDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	skyStateExists, err := skyquest.StateExists(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	var skyTracker *skyquest.PersistentTracker
	if skyStateExists {
		skyTracker, err = skyquest.OpenPersistentTracker(logPath, skyDatabase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
	if *textMode {
		if !skyStateExists {
			fmt.Fprintln(os.Stderr, "Plane of Sky quest tracking is not initialized; launch the TUI once to enable it")
		}
		tracker, xpSession, err := replayLog(logPath, *idleTimeout, backDuration(*backMinutes), since, *historyLimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		printText(tracker, xpSession)
		return
	}

	if err := runApp(logPath, *idleTimeout, backDuration(*backMinutes), since, *historyLimit, skyDatabase, skyTracker, !skyStateExists); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func backDuration(minutes int) time.Duration {
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

func parseSince(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		timestamp, err := time.Parse(layout, value)
		if err == nil {
			return timestamp, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse --since: expected YYYY-MM-DD HH:MM, got %q", value)
}

func replayLog(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int) (*combat.FightTracker, *xp.Session, error) {
	return replayLogWithProgress(logPath, idleTimeout, back, since, historyLimit, 0, nil, nil)
}

type replayProgress struct {
	Bytes int64
	Total int64
	Lines int
}

var errReplayCancelled = errors.New("replay cancelled")

func replayLogWithProgress(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int, maxBytes int64, onProgress func(replayProgress), cancel <-chan struct{}) (*combat.FightTracker, *xp.Session, error) {
	cutoff, err := replayCutoff(logPath, back, since)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	if maxBytes <= 0 {
		info, statErr := file.Stat()
		if statErr != nil {
			return nil, nil, fmt.Errorf("stat log: %w", statErr)
		}
		maxBytes = info.Size()
	}

	tracker := combat.NewFightTrackerWithHistory(historyLimit)
	xpSession := xp.NewSession()
	var latest time.Time
	var bytesRead int64
	linesRead := 0
	scanner := bufio.NewScanner(io.LimitReader(file, maxBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if linesRead%1000 == 0 && replayCancelled(cancel) {
			return nil, nil, errReplayCancelled
		}
		line := scanner.Text()
		bytesRead += int64(len(scanner.Bytes()) + 1)
		linesRead++
		record, hasTimestamp := eqlog.ParseRecordAfter(line, cutoff)
		if hasTimestamp && record.Time.After(latest) {
			latest = record.Time
		}
		if onProgress != nil && linesRead%5000 == 0 {
			onProgress(replayProgress{Bytes: min(bytesRead, maxBytes), Total: maxBytes, Lines: linesRead})
		}
		if hasTimestamp {
			processRecord(record, tracker, xpSession, idleTimeout)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read log: %w", err)
	}
	if !latest.IsZero() {
		tracker.EndIdleAtLogTime(latest, idleTimeout)
	}
	if replayCancelled(cancel) {
		return nil, nil, errReplayCancelled
	}
	if onProgress != nil {
		onProgress(replayProgress{Bytes: maxBytes, Total: maxBytes, Lines: linesRead})
	}
	return tracker, xpSession, nil
}

func replayCancelled(cancel <-chan struct{}) bool {
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

func replayCutoff(logPath string, back time.Duration, since time.Time) (time.Time, error) {
	if !since.IsZero() {
		return since, nil
	}
	if back <= 0 {
		return time.Time{}, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	var latest time.Time
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if timestamp, ok := eqlog.ParseTime(scanner.Text()); ok && timestamp.After(latest) {
			latest = timestamp
		}
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, fmt.Errorf("read log: %w", err)
	}
	if latest.IsZero() {
		return time.Time{}, nil
	}
	return latest.Add(-back), nil
}

func printText(tracker *combat.FightTracker, xpSession *xp.Session) {
	if snapshot := xpSession.SnapshotAtLatestLog(); snapshot.Gains > 0 {
		fmt.Printf("Session XP: %.3f%%, %.2f%%/h over %s active (%d gains)\n\n",
			snapshot.Percent,
			snapshot.PercentPerHour,
			formatDuration(snapshot.ActiveDuration),
			snapshot.Gains,
		)
	}
	sections := tracker.DisplaySections()
	if len(sections) == 0 {
		fmt.Println("No fight found.")
		return
	}

	for index, section := range sections {
		if index > 0 {
			fmt.Println()
		}
		fmt.Println(fightTitle(section.Fight, section.Current))
		fmt.Printf("%-24s %4s %8s %6s %6s %5s %5s %6s %6s %6s\n", "Combatant", "%", "Damage", "DPS", "SDPS", "Hits", "Crits", "Min", "Max", "Active")
		duration := section.Fight.ActiveDuration()
		for _, player := range section.Fight.Meter.Players() {
			dps, sdps := playerDPSColumns(player, section.Fight.Meter.Ended(), duration)
			fmt.Printf("%-24s %4d %8d %6s %6s %5d %5d %6d %6d %6s\n",
				player.Name,
				100,
				player.Damage,
				dps,
				sdps,
				player.Hits,
				player.Crits,
				player.MinHit,
				player.MaxHit,
				formatDuration(player.ActiveDuration()),
			)
		}
	}
}

func runApp(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int, skyDatabase skyquest.Database, skyTracker *skyquest.PersistentTracker, skyNeedsSetup bool) error {
	app := tview.NewApplication()
	tracker := combat.NewFightTrackerWithHistory(historyLimit)
	xpSession := xp.NewSession()
	var mu sync.Mutex
	var skyMu sync.Mutex
	expandedRows := make(map[string]bool)
	expandableRows := make(map[int]string)
	terminalWidth := 100
	fightFilter := ""
	skyViewOpen := false
	var renderSkyView = func() {}
	skyStartOffset := int64(0)
	if skyTracker != nil {
		skyStartOffset = skyTracker.Offset()
	} else {
		info, err := os.Stat(logPath)
		if err != nil {
			return fmt.Errorf("stat log for live tail: %w", err)
		}
		skyStartOffset = info.Size()
	}

	if back > 0 || !since.IsZero() {
		backfill, backfillXP, err := replayLog(logPath, idleTimeout, back, since, historyLimit)
		if err != nil {
			return err
		}
		tracker = backfill
		xpSession = backfillXP
	}

	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	header := tview.NewTextView().
		SetDynamicColors(true)
	status := tview.NewTextView().
		SetDynamicColors(true)

	render := func() {
		mu.Lock()
		defer mu.Unlock()

		sections := filterSections(tracker.DisplaySections(), fightFilter)
		header.SetText(titleText(logPath, terminalWidth))
		status.SetText(statusText(xpSession.SnapshotLive(time.Now()), fightFilter))
		expandableRows = fillTable(table, sections, expandedRows, terminalWidth)
		if skyViewOpen {
			renderSkyView()
		}
	}
	render()

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, _ := screen.Size()
		if width > 0 && width != terminalWidth {
			terminalWidth = width
			render()
		}
		return false
	})

	done := make(chan struct{})
	errCh := make(chan error, 1)
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
	go func() {
		if err := followLog(logPath, skyStartOffset, done, func(line string, endOffset int64) {
			mu.Lock()
			processLine(line, tracker, xpSession, idleTimeout)
			mu.Unlock()
			skyMu.Lock()
			activeSkyTracker := skyTracker
			if activeSkyTracker != nil {
				if err := activeSkyTracker.ProcessLine(line, endOffset); err != nil {
					skyMu.Unlock()
					errCh <- err
					app.Stop()
					return
				}
			}
			skyMu.Unlock()
			app.QueueUpdateDraw(render)
		}); err != nil {
			errCh <- err
			app.Stop()
		}
	}()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				mu.Lock()
				tracker.EndIdle(now, idleTimeout)
				mu.Unlock()
				app.QueueUpdateDraw(render)
			}
		}
	}()

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(status, 1, 0, false)
	pages := tview.NewPages().
		AddPage("main", layout, true, true)
	skyTable := tview.NewTable().SetBorders(false).SetSelectable(true, false).SetFixed(1, 0)
	skyHeader := tview.NewTextView().SetDynamicColors(true)
	skyFooter := tview.NewTextView().SetDynamicColors(true).
		SetText("[gray]↑/↓ PgUp/PgDn[::-] browse   [gray]p/Esc[::-] close")
	character, server, _ := skyquest.CharacterIdentity(logPath)
	renderSkyView = func() {
		skyMu.Lock()
		active := skyTracker
		if active == nil {
			skyMu.Unlock()
			return
		}
		progress := active.QuestProgress()
		inventory := active.Inventory()
		skyMu.Unlock()
		skyHeader.SetText(fmt.Sprintf("[::b]Plane of Sky Quest Tracker[::-]  %s / %s", character, server))
		fillSkyQuestTable(skyTable, progress, inventory)
	}
	skyLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(skyHeader, 1, 0, false).
		AddItem(skyTable, 0, 1, true).
		AddItem(skyFooter, 1, 0, false)
	skySetupOpen := false
	skyScanOpen := false
	var skyScanCancel chan struct{}
	var skyScanView *tview.TextView
	historyOpen := false
	filterOpen := false
	replayOpen := false
	var replayCancel chan struct{}
	var replayView *tview.TextView

	openSkyView := func() {
		skyMu.Lock()
		enabled := skyTracker != nil
		skyMu.Unlock()
		if !enabled {
			return
		}
		skyViewOpen = true
		renderSkyView()
		pages.AddPage("sky-view", skyLayout, true, true)
		app.SetFocus(skyTable)
		skyTable.ScrollToBeginning()
		skyTable.Select(1, 0)
	}

	closeSkyView := func() {
		pages.RemovePage("sky-view")
		skyViewOpen = false
		app.SetFocus(table)
	}

	closeSkySetup := func() {
		pages.RemovePage("sky-setup")
		skySetupOpen = false
		app.SetFocus(table)
	}

	closeSkyScan := func() {
		pages.RemovePage("sky-scan")
		skyScanOpen = false
		skyScanCancel = nil
		skyScanView = nil
		app.SetFocus(table)
	}

	startSkyScan := func() {
		info, err := os.Stat(logPath)
		if err != nil {
			skyScanOpen = true
			skyScanView = showProgressOverlay(app, pages, "sky-scan", " Plane of Sky scan failed — Esc close ")
			skyScanView.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
			return
		}

		skyScanOpen = true
		skyScanCancel = make(chan struct{})
		cancel := skyScanCancel
		skyScanView = showProgressOverlay(app, pages, "sky-scan", " Initial Plane of Sky scan — Esc cancel ")
		skyScanView.SetText(operationProgressText("Scanning existing loot history…", 0, info.Size(), 0))

		go func(snapshotSize int64) {
			created, scanErr := skyquest.InitializePersistentTracker(
				logPath, skyDatabase, snapshotSize,
				func(progress skyquest.ScanProgress) {
					app.QueueUpdateDraw(func() {
						if skyScanOpen && skyScanView != nil {
							skyScanView.SetText(operationProgressText("Scanning existing loot history…", progress.Bytes, progress.Total, progress.Lines))
						}
					})
				},
				cancel,
			)
			if scanErr == nil {
				skyMu.Lock()
				scanErr = created.SyncLog(logPath)
				if scanErr == nil {
					skyTracker = created
					skyNeedsSetup = false
				}
				skyMu.Unlock()
			}
			app.QueueUpdateDraw(func() {
				if errors.Is(scanErr, skyquest.ErrScanCancelled) {
					closeSkyScan()
					return
				}
				if scanErr != nil {
					skyScanCancel = nil
					if skyScanView != nil {
						skyScanView.SetTitle(" Plane of Sky scan failed — Esc close ")
						skyScanView.SetText("[red]" + tview.Escape(scanErr.Error()) + "[::-]")
					}
					return
				}
				closeSkyScan()
			})
		}(info.Size())
	}

	showSkySetup := func() {
		skySetupOpen = true
		info, _ := os.Stat(logPath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		modal := tview.NewModal().
			SetText(fmt.Sprintf(
				"Plane of Sky Quest Tracker\n\nTo determine which quest items you already own, eqdps needs to scan your existing logfile once.\n\nLog: %s\nSize: %s\n\nNo state file is created if you choose Not Now.",
				filepath.Base(logPath), formatByteSize(size),
			)).
			AddButtons([]string{"Enable and Scan", "Not Now"}).
			SetDoneFunc(func(_ int, label string) {
				closeSkySetup()
				if label == "Enable and Scan" {
					startSkyScan()
				}
			})
		pages.AddPage("sky-setup", modal, true, true)
		app.SetFocus(modal)
	}

	closeHistoryModal := func() {
		pages.RemovePage("history")
		historyOpen = false
		app.SetFocus(table)
	}

	closeReplayModal := func() {
		pages.RemovePage("replay")
		replayOpen = false
		replayCancel = nil
		replayView = nil
		app.SetFocus(table)
	}

	showReplayModal := func() {
		replayView = showProgressOverlay(app, pages, "replay", " Loading history — Esc cancel ")
	}

	startReplay := func(duration time.Duration) {
		info, err := os.Stat(logPath)
		if err != nil {
			replayOpen = true
			showReplayModal()
			replayView.SetTitle(" Replay failed — Esc close ")
			replayView.SetText("[red]" + tview.Escape(err.Error()) + "[::-]")
			return
		}
		replayOpen = true
		replayCancel = make(chan struct{})
		cancel := replayCancel
		showReplayModal()
		replayView.SetText(operationProgressText("Loading combat history…", 0, info.Size(), 0))

		go func(snapshotSize int64) {
			nextTracker, nextXP, replayErr := replayLogWithProgress(
				logPath, idleTimeout, duration, time.Time{}, historyLimit, snapshotSize,
				func(progress replayProgress) {
					app.QueueUpdateDraw(func() {
						if replayOpen && replayView != nil {
							replayView.SetText(operationProgressText("Loading combat history…", progress.Bytes, progress.Total, progress.Lines))
						}
					})
				},
				cancel,
			)
			app.QueueUpdateDraw(func() {
				if errors.Is(replayErr, errReplayCancelled) {
					closeReplayModal()
					return
				}
				if replayErr != nil {
					replayCancel = nil
					if replayView != nil {
						replayView.SetTitle(" Replay failed — Esc close ")
						replayView.SetText("[red]" + tview.Escape(replayErr.Error()) + "[::-]")
					}
					return
				}

				mu.Lock()
				tracker = nextTracker
				xpSession = nextXP
				expandedRows = make(map[string]bool)
				expandableRows = make(map[int]string)
				mu.Unlock()
				closeReplayModal()
				render()
				resetTableView(table)
			})
		}(info.Size())
	}

	openHistoryModal := func() {
		if historyOpen {
			return
		}
		historyOpen = true
		modal := tview.NewModal().
			SetText("Open history").
			AddButtons([]string{"Now", "Last Hour", "Last 4 Hours", "Last 8 Hours", "Last Day", "Full", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				closeHistoryModal()
				if buttonLabel == "Cancel" {
					return
				}

				duration, ok := historyDuration(buttonLabel)
				if !ok {
					return
				}

				if duration != 0 {
					startReplay(duration)
					return
				}

				mu.Lock()
				tracker = combat.NewFightTrackerWithHistory(historyLimit)
				xpSession = xp.NewSession()
				expandedRows = make(map[string]bool)
				expandableRows = make(map[int]string)
				mu.Unlock()
				render()
				resetTableView(table)
			})
		pages.AddPage("history", modal, true, true)
		app.SetFocus(modal)
	}

	closeFilter := func() {
		pages.RemovePage("filter")
		filterOpen = false
		app.SetFocus(table)
	}

	openFilter := func() {
		if filterOpen {
			return
		}
		filterOpen = true
		input := tview.NewInputField().
			SetLabel(" Mob name: ").
			SetText(fightFilter).
			SetFieldWidth(36)
		input.SetBorder(true).SetTitle(" Filter fights — Enter apply, Esc cancel ")
		input.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEsc {
				closeFilter()
				return
			}
			if key != tcell.KeyEnter {
				return
			}
			fightFilter = strings.TrimSpace(input.GetText())
			closeFilter()
			render()
			resetTableView(table)
		})
		centered := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(input, 56, 0, true).
				AddItem(nil, 0, 1, false), 3, 0, true).
			AddItem(nil, 0, 1, false)
		pages.AddPage("filter", centered, true, true)
		app.SetFocus(input)
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if skyScanOpen {
			if event.Key() == tcell.KeyEsc {
				if skyScanCancel != nil {
					close(skyScanCancel)
					skyScanCancel = nil
					if skyScanView != nil {
						skyScanView.SetTitle(" Cancelling Plane of Sky scan… ")
					}
				} else {
					closeSkyScan()
				}
				return nil
			}
			return nil
		}
		if skySetupOpen {
			return event
		}
		if skyViewOpen {
			if event.Key() == tcell.KeyEsc {
				closeSkyView()
				return nil
			}
			switch event.Rune() {
			case 'p', 'P':
				closeSkyView()
				return nil
			}
			return event
		}
		if replayOpen {
			if event.Key() == tcell.KeyEsc {
				if replayCancel != nil {
					close(replayCancel)
					replayCancel = nil
					if replayView != nil {
						replayView.SetTitle(" Cancelling replay… ")
					}
				} else {
					closeReplayModal()
				}
				return nil
			}
			return nil
		}
		if historyOpen {
			if event.Key() == tcell.KeyEsc {
				closeHistoryModal()
				return nil
			}
			return event
		}
		if filterOpen {
			if event.Key() == tcell.KeyEsc {
				closeFilter()
				return nil
			}
			return event
		}

		switch event.Key() {
		case tcell.KeyEsc:
			stop()
			app.Stop()
			return nil
		case tcell.KeyEnter:
			row, _ := table.GetSelection()
			if key, ok := expandableRows[row]; ok {
				rowOffset, columnOffset := table.GetOffset()
				expandedRows[key] = !expandedRows[key]
				render()
				restoreTablePosition(table, expandableRows, key, rowOffset, columnOffset)
				return nil
			}
		}
		switch event.Rune() {
		case 'q', 'Q':
			stop()
			app.Stop()
			return nil
		case 'r', 'R':
			mu.Lock()
			tracker = combat.NewFightTrackerWithHistory(historyLimit)
			xpSession = xp.NewSession()
			expandedRows = make(map[string]bool)
			expandableRows = make(map[int]string)
			mu.Unlock()
			render()
			resetTableView(table)
			return nil
		case 'a', 'A':
			row, _ := table.GetSelection()
			key, ok := expandableRows[row]
			if !ok {
				return nil
			}
			rowOffset, columnOffset := table.GetOffset()
			mu.Lock()
			sections := filterSections(tracker.DisplaySections(), fightFilter)
			toggleRowTree(key, sections, expandedRows)
			mu.Unlock()
			render()
			restoreTablePosition(table, expandableRows, key, rowOffset, columnOffset)
			return nil
		case 'o', 'O':
			openHistoryModal()
			return nil
		case '/':
			openFilter()
			return nil
		case 'p', 'P':
			skyMu.Lock()
			enabled := skyTracker != nil
			skyMu.Unlock()
			if enabled {
				openSkyView()
			} else if !skySetupOpen && !skyScanOpen {
				showSkySetup()
			}
			return nil
		}
		return event
	})
	if skyNeedsSetup {
		showSkySetup()
	}

	app.SetRoot(pages, true)
	if !skyNeedsSetup {
		app.SetFocus(table)
	}
	err := app.Run()
	stop()
	if skyScanCancel != nil {
		close(skyScanCancel)
		skyScanCancel = nil
	}
	skyMu.Lock()
	if skyTracker != nil {
		if saveErr := skyTracker.Save(); saveErr != nil && err == nil {
			err = saveErr
		}
	}
	skyMu.Unlock()
	select {
	case tailErr := <-errCh:
		return tailErr
	default:
	}
	return err
}

func resetTableView(table *tview.Table) {
	table.ScrollToBeginning()
	table.Select(1, 0)
}

func restoreTablePosition(table *tview.Table, expandableRows map[int]string, selectedKey string, rowOffset, columnOffset int) {
	for row, key := range expandableRows {
		if key == selectedKey {
			table.Select(row, 0)
			table.SetOffset(rowOffset, columnOffset)
			return
		}
	}
}

func processLine(line string, tracker *combat.FightTracker, xpSession *xp.Session, idleTimeout time.Duration) {
	record, ok := eqlog.ParseRecord(line)
	if ok {
		processRecord(record, tracker, xpSession, idleTimeout)
	}
}

func processRecord(record eqlog.Record, tracker *combat.FightTracker, xpSession *xp.Session, idleTimeout time.Duration) {
	xpSession.Observe(record.Time, time.Now())
	switch record.Kind {
	case eqlog.RecordCast:
		tracker.AddCast(record.Cast)
	case eqlog.RecordDamage:
		xpSession.AddCombat(record.Damage.Time)
		tracker.AddDamageWithIdle(record.Damage, idleTimeout)
	case eqlog.RecordExperience:
		xpSession.AddGain(record.Experience.Time, record.Experience.Percent)
	case eqlog.RecordLevelUp:
		xpSession.AddLevelUp(record.LevelUp.Time)
	case eqlog.RecordAggroClear:
		tracker.ForgetEnemies(record.Time)
	case eqlog.RecordDeath:
		tracker.AddDeath(record.Death)
	}
}

func followLog(logPath string, startOffset int64, done <-chan struct{}, onLine func(string, int64)) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seek log checkpoint: %w", err)
	}

	reader := bufio.NewReader(file)
	offset := startOffset
	for {
		select {
		case <-done:
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			offset += int64(len(line))
			onLine(line, offset)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(line) > 0 {
				if _, seekErr := file.Seek(offset, io.SeekStart); seekErr != nil {
					return fmt.Errorf("rewind partial log line: %w", seekErr)
				}
				reader.Reset(file)
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		return fmt.Errorf("read log: %w", err)
	}
}

func titleText(logPath string, terminalWidth int) string {
	title := "EverQuest DPS Meter"
	maxPathWidth := terminalWidth - len(title) - 4
	if maxPathWidth < 12 {
		maxPathWidth = 12
	}
	return fmt.Sprintf("[::b]%s[::-]  %s", title, fitText(logPath, maxPathWidth))
}

func statusText(snapshot xp.Snapshot, fightFilter string) string {
	controls := "[gray]o[::-] history   [gray]p[::-] Sky quests   [gray]/[::-] filter   [gray]Enter[::-] details   [gray]a[::-] toggle tree   [gray]r[::-] reset   [gray]q/Esc[::-] quit"
	if fightFilter != "" {
		controls = fmt.Sprintf("[yellow]filter: %s[::-]   %s", tview.Escape(fightFilter), controls)
	}
	if snapshot.Gains == 0 {
		return controls
	}
	etaText := "~--:-- to level"
	if eta, ok := snapshot.TimeToLevel(); ok {
		etaText = "~" + formatHoursMinutes(eta) + " to level"
	}
	progressPrefix := "~"
	if snapshot.ProgressKnown {
		progressPrefix = ""
	}
	return fmt.Sprintf("[green]XP %s%.1f%%  %.1f%%/h  %s[::-]   %s",
		progressPrefix,
		snapshot.LevelPercent,
		snapshot.PercentPerHour,
		etaText,
		controls,
	)
}

func historyDuration(label string) (time.Duration, bool) {
	switch label {
	case "Now":
		return 0, true
	case "Last Hour":
		return time.Hour, true
	case "Last 4 Hours":
		return 4 * time.Hour, true
	case "Last 8 Hours":
		return 8 * time.Hour, true
	case "Last Day":
		return 24 * time.Hour, true
	case "Full":
		return -time.Nanosecond, true
	default:
		return 0, false
	}
}

func filterSections(sections []combat.DisplaySection, query string) []combat.DisplaySection {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return sections
	}
	filtered := make([]combat.DisplaySection, 0, len(sections))
	for _, section := range sections {
		if section.Fight != nil && strings.Contains(strings.ToLower(section.Fight.Mob), query) {
			filtered = append(filtered, section)
		}
	}
	return filtered
}

func toggleRowTree(selectedKey string, sections []combat.DisplaySection, expandedRows map[string]bool) bool {
	keys := rowTreeKeys(selectedKey, sections)
	if len(keys) == 0 {
		return false
	}
	open := false
	for _, key := range keys {
		if expandedRows[key] {
			open = true
			break
		}
	}
	for _, key := range keys {
		expandedRows[key] = !open
	}
	return true
}

func rowTreeKeys(selectedKey string, sections []combat.DisplaySection) []string {
	for _, section := range sections {
		sectionKey := sectionRowKey(section)
		mobKey := "mob:" + sectionKey
		if selectedKey == mobKey {
			keys := []string{mobKey}
			for _, player := range section.Fight.Meter.Players() {
				keys = append(keys, combatantTreeKeys(sectionKey, player)...)
			}
			return keys
		}
		for _, player := range section.Fight.Meter.Players() {
			combatantKey := "combatant:" + sectionKey + ":" + player.Name
			if selectedKey == combatantKey {
				return combatantTreeKeys(sectionKey, player)
			}
			for _, entry := range player.DamageBreakdown() {
				categoryKey := combatantKey + ":category:" + entry.Name
				if selectedKey == categoryKey {
					return []string{categoryKey}
				}
			}
		}
	}
	return nil
}

func combatantTreeKeys(sectionKey string, player combat.PlayerStats) []string {
	combatantKey := "combatant:" + sectionKey + ":" + player.Name
	keys := []string{combatantKey}
	for _, entry := range player.DamageBreakdown() {
		keys = append(keys, combatantKey+":category:"+entry.Name)
	}
	return keys
}

func operationProgressText(message string, bytes, total int64, lines int) string {
	const barWidth = 32
	percentComplete := 0.0
	if total > 0 {
		percentComplete = float64(bytes) / float64(total)
	}
	percentComplete = min(max(percentComplete, 0), 1)
	filled := int(percentComplete * barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return fmt.Sprintf("\n%s\n\n[green]%s[::-]  %3.0f%%\n\n%s / %s   %d lines processed",
		message, bar, percentComplete*100, formatByteSize(bytes), formatByteSize(total), lines)
}

func formatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	divisor, exponent := int64(unit), 0
	for value := bytes / unit; value >= unit && exponent < 3; value /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(divisor), "KMGT"[exponent])
}

func centeredView(view tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(view, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)
}

func showProgressOverlay(app *tview.Application, pages *tview.Pages, page, title string) *tview.TextView {
	view := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	view.SetBorder(true).SetTitle(title)
	pages.AddPage(page, centeredView(view, 68, 9), true, true)
	app.SetFocus(view)
	return view
}

func fillSkyQuestTable(table *tview.Table, progress []skyquest.QuestProgress, inventory map[string]int) {
	table.Clear()
	headers := []string{"Quest / Required Item", "Status", "Have", "Need", "Source / Reward"}
	for column, header := range headers {
		cell := tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false)
		if column == 0 {
			cell.SetExpansion(1).SetMaxWidth(46)
		} else if column == 4 {
			cell.SetExpansion(1)
		} else {
			cell.SetAlign(tview.AlignRight)
		}
		table.SetCell(0, column, cell)
	}

	row := 1
	ready := make([]skyquest.QuestProgress, 0)
	for _, item := range progress {
		if item.Ready {
			ready = append(ready, item)
		}
	}
	table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("READY TO TURN IN (%d)", len(ready))).SetTextColor(tcell.ColorGreen).SetSelectable(false))
	row++
	if len(ready) == 0 {
		table.SetCell(row, 0, tview.NewTableCell("  No quests currently have every required item").SetTextColor(tcell.ColorGray).SetSelectable(false))
		row++
	} else {
		for _, item := range ready {
			setSkyRow(table, row, "  ✓ "+item.Class+" — "+skyQuestDisplayName(item.Class, item.Quest.Name), "READY", "", "", questDetails(item.Quest), tcell.ColorGreen, false)
			row++
		}
	}

	table.SetCell(row, 0, tview.NewTableCell("ALL CLASSES").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	row++
	for index := 0; index < len(progress); {
		className := progress[index].Class
		end := index
		readyCount := 0
		for end < len(progress) && progress[end].Class == className {
			if progress[end].Ready {
				readyCount++
			}
			end++
		}
		giver := ""
		if index < end {
			giver = progress[index].Quest.QuestGiver
		}
		setSkyRow(table, row, className+" — "+giver, fmt.Sprintf("%d ready / %d", readyCount, end-index), "", "", "", tcell.ColorYellow, false)
		row++
		for _, item := range progress[index:end] {
			status := fmt.Sprintf("missing %d", len(item.Missing))
			color := tcell.ColorWhite
			if item.Ready {
				status = "READY"
				color = tcell.ColorGreen
			}
			setSkyRow(table, row, "  "+skyQuestDisplayName(item.Class, item.Quest.Name), status, "", "", "Reward: "+strings.Join(item.Quest.Rewards, " / "), color, true)
			row++
			for _, requirement := range item.Quest.Requirements {
				owned := inventory[requirement.Name]
				mark := "✗"
				requirementColor := tcell.ColorRed
				if owned >= requirement.Quantity {
					mark = "✓"
					requirementColor = tcell.ColorGreen
				}
				setSkyRow(table, row, "      "+mark+" "+requirement.Name, "", fmt.Sprint(owned), fmt.Sprint(requirement.Quantity), skyRequirementSource(requirement), requirementColor, false)
				row++
			}
		}
		index = end
	}
}

func setSkyRow(table *tview.Table, row int, name, status, owned, needed, detail string, color tcell.Color, selectable bool) {
	values := []string{name, status, owned, needed, detail}
	for column, value := range values {
		cell := tview.NewTableCell(value).SetTextColor(color).SetSelectable(selectable)
		if column == 0 {
			cell.SetExpansion(1).SetMaxWidth(46)
		} else if column == 4 {
			cell.SetExpansion(1)
		} else {
			cell.SetAlign(tview.AlignRight)
		}
		table.SetCell(row, column, cell)
	}
}

func questDetails(quest skyquest.Quest) string {
	return "Reward: " + strings.Join(quest.Rewards, " / ") + " — " + quest.QuestGiver
}

func skyQuestDisplayName(className, questName string) string {
	return strings.TrimPrefix(questName, className+" ")
}

func skyRequirementSource(requirement skyquest.Requirement) string {
	if requirement.Island > 0 && requirement.DropsFrom != "" {
		return fmt.Sprintf("Island %d — %s", requirement.Island, requirement.DropsFrom)
	}
	if requirement.Island > 0 {
		return fmt.Sprintf("Island %d", requirement.Island)
	}
	if requirement.Kind == "rune" {
		return "Plane of Sky random drop"
	}
	return "Plane of Sky"
}

func fightTitle(fight *combat.Fight, current bool) string {
	if current {
		return fmt.Sprintf("Active mob: %s, %d damage events", fight.Mob, fight.Meter.Events())
	}
	if fight.Death.Victim != "" {
		if sameDisplayName(fight.Death.Victim, "You") {
			return fmt.Sprintf("Mob ended: %s; You slain by %s, %d damage events", fight.Mob, fight.Death.Killer, fight.Meter.Events())
		}
		return fmt.Sprintf("Mob slain: %s by %s, %d damage events", fight.Mob, fight.Death.Killer, fight.Meter.Events())
	}
	if fight.EndReason != "" {
		return fmt.Sprintf("Mob ended: %s; %s, %d damage events", fight.Mob, fight.EndReason, fight.Meter.Events())
	}
	return fmt.Sprintf("Mob: %s, %d damage events", fight.Mob, fight.Meter.Events())
}

func fillTable(table *tview.Table, sections []combat.DisplaySection, expandedRows map[string]bool, terminalWidth int) map[int]string {
	table.Clear()
	expandableRows := make(map[int]string)
	layout := tableLayoutForWidth(terminalWidth)
	headers := []string{"Combatant", "%", "Damage", "DPS", "SDPS", "Hits", "Crits", "Min", "Max", "Active"}
	for col, header := range headers {
		table.SetCell(0, col, tableCell(header, col, layout).SetTextColor(tcell.ColorYellow).SetSelectable(false))
	}

	row := 1
	for index, section := range sections {
		sectionKey := sectionRowKey(section)
		if index > 0 {
			for col := 1; col < len(headers); col++ {
				table.SetCell(row, col, tableCell("", col, layout).SetTextColor(tcell.ColorGray).SetSelectable(false))
			}
			table.SetCell(row, 0, tableCell("----------------------------------------", 0, layout).SetTextColor(tcell.ColorGray).SetSelectable(false))
			row++
		}

		mobRowKey := "mob:" + sectionKey
		expanded, seen := expandedRows[mobRowKey]
		if !seen {
			expanded = section.Current
			expandedRows[mobRowKey] = expanded
		}
		arrow := "▶"
		if expanded {
			arrow = "▼"
		}
		duration := section.Fight.ActiveDuration()
		started := fightStartColumns(section.Fight)
		values := []string{
			arrow + " " + section.Fight.Mob + " (" + mobStatus(section) + ")",
			"", "", localPlayerDPS(section.Fight, duration), started[0], started[1], started[2], started[3], "", formatDuration(duration),
		}
		color := tcell.ColorGray
		if section.Current {
			color = tcell.ColorYellow
		}
		for col, value := range values {
			table.SetCell(row, col, tableCell(value, col, layout).SetTextColor(color))
		}
		expandableRows[row] = mobRowKey
		row++
		if !expanded {
			continue
		}

		for _, player := range section.Fight.Meter.Players() {
			rowKey := "combatant:" + sectionKey + ":" + player.Name
			dps, sdps := playerDPSColumns(player, section.Fight.Meter.Ended(), duration)
			name := "    " + player.Name
			if len(player.Breakdown) > 0 {
				arrow := "▶"
				if expandedRows[rowKey] {
					arrow = "▼"
				}
				name = "  " + arrow + " " + player.Name
			}
			values := []string{
				name, "100", fmt.Sprintf("%d", player.Damage), dps, sdps,
				fmt.Sprintf("%d", player.Hits), fmt.Sprintf("%d", player.Crits),
				fmt.Sprintf("%d", player.MinHit), fmt.Sprintf("%d", player.MaxHit), formatDuration(player.ActiveDuration()),
			}
			for col, value := range values {
				table.SetCell(row, col, tableCell(value, col, layout))
			}
			if len(player.Breakdown) == 0 {
				row++
				continue
			}
			expandableRows[row] = rowKey
			row++
			if expandedRows[rowKey] {
				row = addDamageBreakdownRows(table, row, player, duration, layout, rowKey, expandedRows, expandableRows)
			}
		}
	}
	return expandableRows
}

func fightStartColumns(fight *combat.Fight) [4]string {
	if fight == nil || fight.Meter == nil || fight.Meter.Started().IsZero() {
		return [4]string{}
	}
	started := fight.Meter.Started()
	return [4]string{"Start", started.Format("2006"), started.Format("01-02"), started.Format("15:04")}
}

func localPlayerDPS(fight *combat.Fight, duration time.Duration) string {
	if fight == nil || fight.Meter == nil {
		return ""
	}
	for _, player := range fight.Meter.Players() {
		if player.Name == "You" {
			dps, _ := playerDPSColumns(player, fight.Meter.Ended(), duration)
			return dps
		}
	}
	return ""
}

func addDamageBreakdownRows(table *tview.Table, row int, player combat.PlayerStats, duration time.Duration, layout tableLayout, playerKey string, expandedRows map[string]bool, expandableRows map[int]string) int {
	for _, entry := range player.DamageBreakdown() {
		categoryKey := playerKey + ":category:" + entry.Name
		name := "      " + entry.Name
		if len(entry.Children) > 0 {
			arrow := "▶"
			if expandedRows[categoryKey] {
				arrow = "▼"
			}
			name = "    " + arrow + " " + entry.Name
			expandableRows[row] = categoryKey
		}
		setBreakdownRow(table, row, name, entry, player.Damage, duration, layout)
		row++
		if !expandedRows[categoryKey] {
			continue
		}
		for _, detail := range entry.SortedChildren() {
			setBreakdownRow(table, row, "        "+detail.Name, detail, player.Damage, duration, layout)
			row++
		}
	}
	return row
}

func setBreakdownRow(table *tview.Table, row int, name string, entry combat.BreakdownStats, totalDamage int, duration time.Duration, layout tableLayout) {
	dps, sdps := breakdownDPSColumns(entry, duration)
	values := []string{
		name, fmt.Sprintf("%.0f", percent(entry.Damage, totalDamage)), fmt.Sprintf("%d", entry.Damage), dps, sdps,
		fmt.Sprintf("%d", entry.Hits), fmt.Sprintf("%d", entry.Crits), fmt.Sprintf("%d", entry.MinHit),
		fmt.Sprintf("%d", entry.MaxHit), formatDuration(entry.ActiveDuration()),
	}
	for col, value := range values {
		table.SetCell(row, col, tableCell(value, col, layout).SetTextColor(tcell.ColorLightCyan))
	}
}

type tableLayout struct {
	combatantWidth int
}

func tableLayoutForWidth(width int) tableLayout {
	if width <= 0 {
		width = 100
	}

	return tableLayout{combatantWidth: max(width-61, 20)}
}

func tableCell(value string, col int, layout tableLayout) *tview.TableCell {
	width := columnWidth(col, layout)
	text := value
	if col != 0 {
		text = fitText(value, width)
	}
	cell := tview.NewTableCell(text).SetMaxWidth(width)
	switch col {
	case 1, 2, 3, 4, 5, 6, 7, 8:
		cell.SetAlign(tview.AlignRight)
	}
	if col == 0 {
		cell.SetExpansion(1)
	}
	return cell
}

func columnWidth(col int, layout tableLayout) int {
	switch col {
	case 0:
		return layout.combatantWidth
	case 1:
		return 4
	case 2:
		return 8
	case 3, 4, 7, 8:
		return 6
	case 5, 6:
		return 5
	case 9:
		return 6
	default:
		return 0
	}
}

func fitText(value string, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func sectionRowKey(section combat.DisplaySection) string {
	started := time.Time{}
	if section.Fight != nil && section.Fight.Meter != nil {
		started = section.Fight.Meter.Started()
	}
	return fmt.Sprintf("%s:%s", section.Fight.Mob, started.Format(time.RFC3339Nano))
}

func percent(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func damageDPS(damage int, activeDuration time.Duration) float64 {
	seconds := activeDuration.Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(damage) / seconds
}

func playerDPSColumns(player combat.PlayerStats, ended time.Time, duration time.Duration) (string, string) {
	sustained := player.DPSForDuration(duration)
	active := player.DPS()
	if player.Name == "You" {
		if engaged, ok := player.EngagedDPS(ended); ok {
			active = engaged
		}
	}
	return dpsColumns(active, sustained)
}

func breakdownDPSColumns(entry combat.BreakdownStats, duration time.Duration) (string, string) {
	return dpsColumns(entry.DPS(), damageDPS(entry.Damage, duration))
}

func dpsColumns(active, sustained float64) (string, string) {
	dps := fmt.Sprintf("%.0f", active)
	if sustained == 0 || math.Abs(active-sustained)/sustained < 0.10 {
		return dps, ""
	}
	return dps, fmt.Sprintf("%.0f", sustained)
}

func mobStatus(section combat.DisplaySection) string {
	if section.Current {
		return "active"
	}
	if section.Fight.Death.Victim != "" {
		if sameDisplayName(section.Fight.Death.Victim, "You") {
			return fmt.Sprintf("You slain by %s", section.Fight.Death.Killer)
		}
		return fmt.Sprintf("slain by %s", section.Fight.Death.Killer)
	}
	return section.Fight.EndReason
}

func sameDisplayName(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func formatHoursMinutes(d time.Duration) string {
	minutes := int(d / time.Minute)
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
