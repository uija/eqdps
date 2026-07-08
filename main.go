package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/eqlog"
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
	if *textMode {
		tracker, err := replayLog(logPath, *idleTimeout, backDuration(*backMinutes), since, *historyLimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		printText(tracker)
		return
	}

	if err := runApp(logPath, *idleTimeout, backDuration(*backMinutes), since, *historyLimit); err != nil {
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

func replayLog(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int) (*combat.FightTracker, error) {
	cutoff, err := replayCutoff(logPath, back, since)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	tracker := combat.NewFightTrackerWithHistory(historyLimit)
	var latest time.Time
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		timestamp, hasTimestamp := eqlog.ParseTime(line)
		if hasTimestamp && timestamp.After(latest) {
			latest = timestamp
		}
		if !cutoff.IsZero() && (!hasTimestamp || timestamp.Before(cutoff)) {
			continue
		}
		processLine(line, tracker, idleTimeout)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	if !latest.IsZero() {
		tracker.EndIdleAtLogTime(latest, idleTimeout)
	}
	return tracker, nil
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

func printText(tracker *combat.FightTracker) {
	sections := tracker.DisplaySections()
	if len(sections) == 0 {
		fmt.Println("No fight found.")
		return
	}

	for index, section := range sections {
		if index > 0 {
			fmt.Println()
		}
		fmt.Println(sectionTitle(index, section))
		fmt.Printf("%-24s %10s %8s %6s %6s %8s %s\n", "Combatant", "Damage", "DPS", "Hits", "Crits", "Active", "Last Target")
		for _, player := range section.Fight.Meter.Players() {
			fmt.Printf("%-24s %10d %8.2f %6d %6d %8s %s\n",
				player.Name,
				player.Damage,
				player.DPS(),
				player.Hits,
				player.Crits,
				formatDuration(player.ActiveDuration()),
				player.LastTarget,
			)
		}
	}
}

func runApp(logPath string, idleTimeout, back time.Duration, since time.Time, historyLimit int) error {
	app := tview.NewApplication()
	tracker := combat.NewFightTrackerWithHistory(historyLimit)
	var mu sync.Mutex
	expandedRows := make(map[string]bool)
	expandableRows := make(map[int]string)
	terminalWidth := 100

	if back > 0 || !since.IsZero() {
		backfill, err := replayLog(logPath, idleTimeout, back, since, historyLimit)
		if err != nil {
			return err
		}
		tracker = backfill
	}

	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	header := tview.NewTextView().
		SetDynamicColors(true)
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetText(statusText())

	render := func() {
		mu.Lock()
		defer mu.Unlock()

		sections := tracker.DisplaySections()
		header.SetText(titleText(logPath, terminalWidth))
		expandableRows = fillTable(table, sections, expandedRows, terminalWidth)
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
		if err := followLog(logPath, done, func(line string) {
			mu.Lock()
			processLine(line, tracker, idleTimeout)
			mu.Unlock()
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
				changed := tracker.EndIdle(now, idleTimeout)
				mu.Unlock()
				if changed {
					app.QueueUpdateDraw(render)
				}
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

	openHistoryModal := func() {
		modal := tview.NewModal().
			SetText("Open history").
			AddButtons([]string{"Now", "Last Hour", "Last 4 Hours", "Last 8 Hours", "Last Day", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				pages.RemovePage("history")
				app.SetFocus(table)
				if buttonLabel == "Cancel" {
					return
				}

				duration, ok := historyDuration(buttonLabel)
				if !ok {
					return
				}

				nextTracker := combat.NewFightTrackerWithHistory(historyLimit)
				if duration > 0 {
					replayed, err := replayLog(logPath, idleTimeout, duration, time.Time{}, historyLimit)
					if err == nil {
						nextTracker = replayed
					}
				}

				mu.Lock()
				tracker = nextTracker
				expandedRows = make(map[string]bool)
				expandableRows = make(map[int]string)
				mu.Unlock()
				render()
			})
		pages.AddPage("history", modal, true, true)
		app.SetFocus(modal)
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			stop()
			app.Stop()
			return nil
		case tcell.KeyEnter:
			row, _ := table.GetSelection()
			if key, ok := expandableRows[row]; ok {
				expandedRows[key] = !expandedRows[key]
				render()
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
			expandedRows = make(map[string]bool)
			expandableRows = make(map[int]string)
			mu.Unlock()
			render()
			return nil
		case 'o', 'O':
			openHistoryModal()
			return nil
		}
		return event
	})

	err := app.SetRoot(pages, true).SetFocus(table).Run()
	stop()
	select {
	case tailErr := <-errCh:
		return tailErr
	default:
	}
	return err
}

func processLine(line string, tracker *combat.FightTracker, idleTimeout time.Duration) {
	if event, ok := eqlog.ParseLine(line); ok {
		tracker.AddDamageWithIdle(event, idleTimeout)
		return
	}
	if death, ok := eqlog.ParseDeathLine(line); ok {
		tracker.AddDeath(death)
	}
}

func followLog(logPath string, done <-chan struct{}, onLine func(string)) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek log end: %w", err)
	}

	reader := bufio.NewReader(file)
	for {
		select {
		case <-done:
			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			onLine(line)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
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

func statusText() string {
	return "[gray]o[::-] history   [gray]Enter[::-] details   [gray]r[::-] reset   [gray]q/Esc[::-] quit"
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
	default:
		return 0, false
	}
}

func sectionTitle(index int, section combat.DisplaySection) string {
	if index > 0 {
		if section.Fight.Death.Victim != "" {
			return fmt.Sprintf("Previous fight %d ended: %s slain by %s, %d damage events", index, section.Fight.Death.Victim, section.Fight.Death.Killer, section.Fight.Meter.Events())
		}
		if section.Fight.EndReason != "" {
			return fmt.Sprintf("Previous fight %d ended: %s, %d damage events", index, section.Fight.EndReason, section.Fight.Meter.Events())
		}
		return fmt.Sprintf("Previous fight %d: %d damage events", index, section.Fight.Meter.Events())
	}
	return fightTitle(section.Fight, section.Current)
}

func fightTitle(fight *combat.Fight, current bool) string {
	status := "Last fight"
	if current {
		status = "Current fight"
	}
	if !current && fight.Death.Victim != "" {
		return fmt.Sprintf("%s ended: %s slain by %s, %d damage events", status, fight.Death.Victim, fight.Death.Killer, fight.Meter.Events())
	}
	if !current && fight.EndReason != "" {
		return fmt.Sprintf("%s ended: %s, %d damage events", status, fight.EndReason, fight.Meter.Events())
	}
	return fmt.Sprintf("%s: %d damage events", status, fight.Meter.Events())
}

func fillTable(table *tview.Table, sections []combat.DisplaySection, expandedRows map[string]bool, terminalWidth int) map[int]string {
	table.Clear()
	expandableRows := make(map[int]string)
	layout := tableLayoutForWidth(terminalWidth)
	headers := []string{"Combatant", "Damage", "DPS", "Hits", "Crits", "Active", "Last Target"}
	for col, header := range headers {
		table.SetCell(0, col, tableCell(header, col, layout).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false))
	}

	if len(sections) == 0 {
		return expandableRows
	}

	row := 1
	for index, section := range sections {
		sectionKey := sectionRowKey(index, section)
		if index > 0 {
			for col := 1; col < len(headers); col++ {
				table.SetCell(row, col, tableCell("", col, layout).
					SetTextColor(tcell.ColorGray).
					SetSelectable(false))
			}
			table.SetCell(row, 0, tableCell("----------------------------------------", 0, layout).
				SetTextColor(tcell.ColorGray).
				SetSelectable(false))
			row++

			label, victim, events := previousFightCells(index, section)
			values := []string{label, "", "", "", "", "", victim}
			if events != "" {
				values[1] = events
			}
			for col, value := range values {
				table.SetCell(row, col, tableCell(value, col, layout).
					SetTextColor(tcell.ColorGray).
					SetSelectable(false))
			}
			row++
		}
		for _, player := range section.Fight.Meter.Players() {
			rowKey := sectionKey + ":" + player.Name
			values := []string{
				player.Name,
				fmt.Sprintf("%d", player.Damage),
				fmt.Sprintf("%.2f", player.DPS()),
				fmt.Sprintf("%d", player.Hits),
				fmt.Sprintf("%d", player.Crits),
				formatDuration(player.ActiveDuration()),
				player.LastTarget,
			}
			for col, value := range values {
				table.SetCell(row, col, tableCell(value, col, layout))
			}
			if player.Name == "You" && len(player.DamageTypes) > 0 {
				expandableRows[row] = rowKey
				if expandedRows[rowKey] {
					row++
					row = addDamageBreakdownRows(table, row, player, layout)
				}
			}
			row++
		}
	}
	return expandableRows
}

func addDamageBreakdownRows(table *tview.Table, row int, player combat.PlayerStats, layout tableLayout) int {
	for _, entry := range player.DamageBreakdown() {
		table.SetCell(row, 0, tableCell("  "+entry.Name, 0, layout).
			SetTextColor(tcell.ColorLightCyan).
			SetSelectable(false))
		table.SetCell(row, 1, tableCell(fmt.Sprintf("%d", entry.Damage), 1, layout).
			SetTextColor(tcell.ColorLightCyan).
			SetSelectable(false))
		table.SetCell(row, 2, tableCell(fmt.Sprintf("%.1f%%", percent(entry.Damage, player.Damage)), 2, layout).
			SetTextColor(tcell.ColorLightCyan).
			SetSelectable(false))
		for col := 3; col < 7; col++ {
			table.SetCell(row, col, tableCell("", col, layout).
				SetTextColor(tcell.ColorLightCyan).
				SetSelectable(false))
		}
		row++
	}
	return row
}

type tableLayout struct {
	combatantWidth int
	targetWidth    int
}

func tableLayoutForWidth(width int) tableLayout {
	if width <= 0 {
		width = 100
	}

	// Numeric columns plus a small allowance for table spacing.
	textBudget := width - 48
	combatantWidth := clamp(textBudget/2, 10, 28)
	targetWidth := clamp(textBudget-combatantWidth, 8, 44)
	if textBudget < 22 {
		combatantWidth = 10
		targetWidth = 8
	}

	return tableLayout{
		combatantWidth: combatantWidth,
		targetWidth:    targetWidth,
	}
}

func tableCell(value string, col int, layout tableLayout) *tview.TableCell {
	width := columnWidth(col, layout)
	cell := tview.NewTableCell(fitText(value, width)).SetMaxWidth(width)
	switch col {
	case 1, 2, 3, 4:
		cell.SetAlign(tview.AlignRight)
	}
	if col == 0 || col == 6 {
		cell.SetExpansion(1)
	}
	return cell
}

func columnWidth(col int, layout tableLayout) int {
	switch col {
	case 0:
		return layout.combatantWidth
	case 1:
		return 10
	case 2:
		return 8
	case 3, 4:
		return 6
	case 5:
		return 8
	case 6:
		return layout.targetWidth
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

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func sectionRowKey(index int, section combat.DisplaySection) string {
	started := time.Time{}
	if section.Fight != nil && section.Fight.Meter != nil {
		started = section.Fight.Meter.Started()
	}
	return fmt.Sprintf("%d:%s", index, started.Format(time.RFC3339))
}

func percent(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func previousFightCells(index int, section combat.DisplaySection) (string, string, string) {
	label := fmt.Sprintf("Previous fight %d", index)
	victim := ""
	if section.Fight.Death.Victim != "" {
		victim = fmt.Sprintf("%s slain by %s", section.Fight.Death.Victim, section.Fight.Death.Killer)
	} else if section.Fight.EndReason != "" {
		victim = section.Fight.EndReason
	}
	return label, victim, fmt.Sprintf("%d events", section.Fight.Meter.Events())
}

func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}
