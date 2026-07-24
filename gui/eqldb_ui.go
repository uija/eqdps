package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/inventorysync"
	"github.com/uija/eqdps/internal/platform"
)

const (
	eqldbGUIIntroDuration  = 30 * time.Second
	eqldbGUIExportGrace    = 2 * time.Second
	eqldbGUIUploadCooldown = 15 * time.Second
)

var eqldbGUIClassOptions = []string{
	"Not selected",
	"WAR — Warrior",
	"CLR — Cleric",
	"PAL — Paladin",
	"RNG — Ranger",
	"SHD — Shadow Knight",
	"DRU — Druid",
	"MNK — Monk",
	"BRD — Bard",
	"ROG — Rogue",
	"SHM — Shaman",
	"NEC — Necromancer",
	"WIZ — Wizard",
	"MAG — Magician",
	"ENC — Enchanter",
	"BST — Beastlord",
	"BER — Berserker",
}

var eqldbGUIRaceOptions = append([]string{"Select race"}, eqldb.ManualRaces...)

type eqldbGUIEventKind uint8

const (
	eqldbGUIExportDetected eqldbGUIEventKind = iota + 1
	eqldbGUIProcessExport
	eqldbGUIAuthReady
	eqldbGUIAuthDone
	eqldbGUIAuthFailed
	eqldbGUIUploadDone
	eqldbGUIUploadFailed
)

type eqldbGUIEvent struct {
	kind          eqldbGUIEventKind
	sequence      int
	request       inventorysync.Request
	authorization eqldb.DeviceAuthorization
	token         eqldb.Token
	result        eqldb.UploadResult
	err           error
}

type eqldbGUI struct {
	window appWindow
	store  eqldb.Store
	client *eqldb.Client
	state  eqldb.State

	context context.Context
	cancel  context.CancelFunc
	events  chan eqldbGUIEvent

	observerMu sync.Mutex
	observer   *inventorysync.Observer
	logPath    string
	character  string

	modal           string
	lastError       string
	introAttempted  bool
	introDeadline   time.Time
	introTimer      bool
	macroSelection  [2]int
	authSequence    int
	authCancel      context.CancelFunc
	authInfo        eqldb.DeviceAuthorization
	authExtra       string
	pendingExport   *inventorysync.Request
	exportTimer     *time.Timer
	uploading       bool
	cooldownUntil   time.Time
	metadataRequest inventorysync.Request
	metadataError   string
	classPicker     int
	pickerAbove     bool
	pickerItem      int
	metadataFields  [4]int
	classSelected   [3]int
	raceSelected    int

	noticeText  string
	noticeColor color.NRGBA
	noticeUntil time.Time

	connectClick widget.Clickable
	closeClick   widget.Clickable
	retryClick   widget.Clickable
	browserClick widget.Clickable
	forgetClick  widget.Clickable
	uploadClick  widget.Clickable
	cancelClick  widget.Clickable
	classClicks  [3]widget.Clickable
	classChoices []widget.Clickable
	raceClick    widget.Clickable
	raceChoices  []widget.Clickable
	levelEditor  widget.Editor
	macroEditor  widget.Editor
	copyClick    widget.Clickable
	dialogList   widget.List
	dialogModal  string
	metadataList widget.List
	pickerList   widget.List
}

// appWindow is the subset of app.Window used by the controller. Keeping this
// interface small makes the asynchronous state machine straightforward to test.
type appWindow interface {
	Invalidate()
}

func newEQLDBGUI(window appWindow, logPath string) *eqldbGUI {
	store, storeErr := eqldb.DefaultStore()
	state, loadErr := store.Load()
	ctx, cancel := context.WithCancel(context.Background())
	ui := &eqldbGUI{
		window:       window,
		store:        store,
		client:       eqldb.NewClient(),
		state:        state,
		context:      ctx,
		cancel:       cancel,
		events:       make(chan eqldbGUIEvent, 32),
		classPicker:  -1,
		pickerItem:   -1,
		classChoices: make([]widget.Clickable, len(eqldbGUIClassOptions)),
		raceChoices:  make([]widget.Clickable, len(eqldbGUIRaceOptions)),
		dialogList:   widget.List{List: layout.List{Axis: layout.Vertical}},
		metadataList: widget.List{List: layout.List{Axis: layout.Vertical}},
		pickerList:   widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	ui.levelEditor.SingleLine = true
	ui.macroEditor.ReadOnly = true
	ui.macroEditor.SetText(eqldbGUIMacroText(ui.character))
	if storeErr != nil {
		ui.lastError = storeErr.Error()
	} else if loadErr != nil {
		ui.lastError = loadErr.Error()
	}
	ui.SetLog(logPath)
	return ui
}

func (ui *eqldbGUI) Close() {
	ui.cancel()
	if ui.authCancel != nil {
		ui.authCancel()
	}
	if ui.exportTimer != nil {
		ui.exportTimer.Stop()
	}
}

func (ui *eqldbGUI) SetLog(path string) {
	ui.observerMu.Lock()
	defer ui.observerMu.Unlock()
	if path == ui.logPath {
		return
	}
	ui.logPath = path
	ui.observer = nil
	ui.character = ""
	ui.macroEditor.SetText(eqldbGUIMacroText(""))
	if path == "" {
		return
	}
	observer, err := inventorysync.NewObserver(path)
	if err != nil {
		ui.lastError = err.Error()
		return
	}
	character, _, err := inventorysync.CharacterIdentity(path)
	if err != nil {
		ui.lastError = err.Error()
		return
	}
	ui.observer = observer
	ui.character = character
	ui.macroEditor.SetText(eqldbGUIMacroText(character))
}

func (ui *eqldbGUI) Observe(record eqlog.Record) {
	ui.observerMu.Lock()
	observer := ui.observer
	if observer == nil {
		ui.observerMu.Unlock()
		return
	}
	request, ok := observer.Observe(record)
	ui.observerMu.Unlock()
	if !ok {
		return
	}
	ui.send(eqldbGUIEvent{kind: eqldbGUIExportDetected, request: request})
}

func (ui *eqldbGUI) send(event eqldbGUIEvent) {
	select {
	case <-ui.context.Done():
		return
	case ui.events <- event:
		if ui.window != nil {
			ui.window.Invalidate()
		}
	}
}

func (ui *eqldbGUI) Notice(now time.Time) (string, color.NRGBA, bool) {
	if ui.noticeText == "" || !now.Before(ui.noticeUntil) {
		return "", color.NRGBA{}, false
	}
	return ui.noticeText, ui.noticeColor, true
}

func (ui *eqldbGUI) notify(message string, foreground color.NRGBA, duration time.Duration) {
	ui.noticeText = message
	ui.noticeColor = foreground
	ui.noticeUntil = time.Now().Add(duration)
}

func (ui *eqldbGUI) OpenManagement() {
	ui.modal = "manage"
	ui.classPicker = -1
}

func (ui *eqldbGUI) Update(gtx layout.Context, shell *shell) {
	ui.processEvents()
	now := time.Now()
	if !ui.introAttempted && !ui.state.IntroductionShown && ui.state.AccessToken == "" &&
		ui.modal == "" && !shell.loading && !shell.skyLoading && !shell.skySetupOpen &&
		!shell.waylandHelp && !shell.aboutOpen {
		ui.introAttempted = true
		ui.introDeadline = now.Add(eqldbGUIIntroDuration)
		ui.introTimer = true
		start, end := ui.macroEditor.Selection()
		ui.macroSelection = [2]int{start, end}
		ui.modal = "intro"
	}
	if ui.modal == "intro" {
		start, end := ui.macroEditor.Selection()
		if gtx.Focused(&ui.macroEditor) || ui.macroSelection != [2]int{start, end} {
			ui.stopIntroductionTimer()
		}
		ui.macroSelection = [2]int{start, end}
		if ui.introTimer {
			if !now.Before(ui.introDeadline) {
				ui.modal = ""
				ui.introTimer = false
			} else {
				gtx.Execute(op.InvalidateCmd{At: now.Add(time.Second)})
			}
		}
	}
	if ui.modal == "auth" && !ui.authInfo.ExpiresAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: now.Add(time.Second)})
	}

	if ui.connectClick.Clicked(gtx) {
		ui.markIntroductionShown()
		ui.startAuthentication()
	}
	if ui.closeClick.Clicked(gtx) {
		if ui.modal == "intro" {
			ui.markIntroductionShown()
		}
		ui.closeModal()
	}
	if ui.cancelClick.Clicked(gtx) {
		if ui.modal == "auth" {
			ui.cancelAuthentication()
		} else {
			ui.closeModal()
		}
	}
	if ui.retryClick.Clicked(gtx) {
		ui.startAuthentication()
	}
	if ui.browserClick.Clicked(gtx) && ui.authInfo.VerificationURIComplete != "" {
		if err := platform.OpenURL(ui.authInfo.VerificationURIComplete); err != nil {
			ui.authExtra = "Could not open the browser: " + err.Error()
		}
	}
	if ui.forgetClick.Clicked(gtx) {
		ui.state.AccessToken = ""
		ui.state.ConnectionID = ""
		ui.lastError = ""
		if err := ui.store.Save(ui.state); err != nil {
			ui.lastError = err.Error()
		}
		ui.closeModal()
		ui.notify("EQLDB connection removed from this computer", palette.text, 8*time.Second)
	}
	for index := range ui.classClicks {
		if ui.classClicks[index].Clicked(gtx) {
			if ui.classPicker == index {
				ui.classPicker = -1
			} else {
				ui.openMetadataPicker(index, ui.classSelected[index])
			}
		}
	}
	if ui.raceClick.Clicked(gtx) {
		if ui.classPicker == -2 {
			ui.classPicker = -1
		} else {
			ui.openMetadataPicker(-2, ui.raceSelected)
		}
	}
	if ui.classPicker >= 0 {
		for index := range ui.classChoices {
			if ui.classChoices[index].Clicked(gtx) {
				ui.classSelected[ui.classPicker] = index
				ui.classPicker = -1
			}
		}
	} else if ui.classPicker == -2 {
		for index := range ui.raceChoices {
			if ui.raceChoices[index].Clicked(gtx) {
				ui.raceSelected = index
				ui.classPicker = -1
			}
		}
	}
	if ui.uploadClick.Clicked(gtx) {
		ui.submitMetadata()
	}
	if ui.copyClick.Clicked(gtx) {
		ui.stopIntroductionTimer()
		gtx.Execute(clipboard.WriteCmd{
			Type: "text/plain",
			Data: io.NopCloser(strings.NewReader(eqldbGUIMacroText(ui.character))),
		})
		ui.notify("EQLDB macro copied to clipboard", palette.success, 5*time.Second)
	}
}

func (ui *eqldbGUI) processEvents() {
	for {
		select {
		case event := <-ui.events:
			switch event.kind {
			case eqldbGUIExportDetected:
				ui.scheduleExport(event.request, time.Now())
			case eqldbGUIProcessExport:
				ui.processPendingExport()
			case eqldbGUIAuthReady:
				if event.sequence != ui.authSequence || ui.modal != "auth" {
					continue
				}
				ui.authInfo = event.authorization
				ui.authExtra = ""
				if err := platform.OpenURL(event.authorization.VerificationURIComplete); err != nil {
					ui.authExtra = "Could not open the browser: " + err.Error()
				}
			case eqldbGUIAuthDone:
				if event.sequence != ui.authSequence || ui.modal != "auth" {
					continue
				}
				ui.authCancel = nil
				ui.state.IntroductionShown = true
				ui.state.AccessToken = event.token.AccessToken
				ui.state.ConnectionID = event.token.ConnectionID
				ui.lastError = ""
				if err := ui.store.Save(ui.state); err != nil {
					ui.lastError = err.Error()
					ui.modal = "error"
					continue
				}
				ui.modal = "connected"
				ui.notify("EQLDB connected", palette.success, 8*time.Second)
			case eqldbGUIAuthFailed:
				if event.sequence != ui.authSequence || ui.modal != "auth" {
					continue
				}
				ui.authCancel = nil
				if errors.Is(event.err, context.Canceled) {
					continue
				}
				ui.lastError = event.err.Error()
				ui.modal = "error"
			case eqldbGUIUploadDone:
				ui.uploading = false
				ui.lastError = ""
				if event.result.Status == "pending" {
					message := "EQLDB: inventory uploaded — loadout assignment needed"
					if event.result.Message != "" {
						message = "EQLDB: " + event.result.Message
					}
					ui.notify(message, palette.accent, 12*time.Second)
				} else {
					ui.notify(fmt.Sprintf("EQLDB: %s inventory uploaded", event.result.Character), palette.success, 10*time.Second)
				}
			case eqldbGUIUploadFailed:
				ui.uploading = false
				ui.handleUploadError(event.err)
			}
		default:
			return
		}
	}
}

func (ui *eqldbGUI) markIntroductionShown() {
	if ui.state.IntroductionShown {
		return
	}
	ui.state.IntroductionShown = true
	if err := ui.store.Save(ui.state); err != nil {
		ui.lastError = err.Error()
	}
}

func (ui *eqldbGUI) startAuthentication() {
	ui.cancelAuthentication()
	ui.authSequence++
	sequence := ui.authSequence
	ctx, cancel := context.WithCancel(ui.context)
	ui.authCancel = cancel
	ui.authInfo = eqldb.DeviceAuthorization{}
	ui.authExtra = ""
	ui.modal = "auth"
	go func() {
		authorization, err := ui.client.StartConnection(ctx, "eqdps GUI")
		if err != nil {
			ui.send(eqldbGUIEvent{kind: eqldbGUIAuthFailed, sequence: sequence, err: err})
			return
		}
		ui.send(eqldbGUIEvent{kind: eqldbGUIAuthReady, sequence: sequence, authorization: authorization})
		token, err := ui.client.WaitForToken(ctx, authorization)
		if err != nil {
			ui.send(eqldbGUIEvent{kind: eqldbGUIAuthFailed, sequence: sequence, err: err})
			return
		}
		ui.send(eqldbGUIEvent{kind: eqldbGUIAuthDone, sequence: sequence, token: token})
	}()
}

func (ui *eqldbGUI) cancelAuthentication() {
	if ui.authCancel != nil {
		ui.authCancel()
		ui.authCancel = nil
	}
	ui.authSequence++
	if ui.modal == "auth" {
		ui.modal = ""
	}
}

func (ui *eqldbGUI) closeModal() {
	if ui.modal == "auth" {
		ui.cancelAuthentication()
		return
	}
	ui.modal = ""
	ui.introTimer = false
	ui.classPicker = -1
}

func (ui *eqldbGUI) stopIntroductionTimer() {
	if ui.modal == "intro" {
		ui.introTimer = false
	}
}

func (ui *eqldbGUI) openMetadataPicker(picker, selected int) {
	fieldSlot := picker + 1
	if picker == -2 {
		fieldSlot = 0
	}
	fieldItem := ui.metadataFields[fieldSlot]
	position := ui.metadataList.Position
	before := fieldItem - position.First
	after := position.First + position.Count - 1 - fieldItem
	ui.pickerAbove = before > after

	// Remove the old picker's contribution when switching directly between
	// fields. The new picker occupies the selected row's old index when placed
	// above it, and follows that index when placed below it.
	baseItem := fieldItem
	if ui.pickerItem >= 0 && ui.pickerItem < fieldItem {
		baseItem--
	}
	ui.classPicker = picker
	ui.pickerList.ScrollTo(selected)
	ui.metadataList.ScrollTo(baseItem)
}

func (ui *eqldbGUI) scheduleExport(request inventorysync.Request, now time.Time) {
	if ui.state.AccessToken == "" {
		return
	}
	if ui.uploading || now.Before(ui.cooldownUntil) {
		remaining := max(ui.cooldownUntil.Sub(now), time.Second)
		ui.notify(fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))), palette.accent, 8*time.Second)
		return
	}
	copy := request
	ui.pendingExport = &copy
	if ui.exportTimer != nil {
		ui.exportTimer.Stop()
	}
	ui.exportTimer = time.AfterFunc(eqldbGUIExportGrace, func() {
		ui.send(eqldbGUIEvent{kind: eqldbGUIProcessExport})
	})
	ui.notify("EQLDB: inventory export detected…", palette.text, 4*time.Second)
}

func (ui *eqldbGUI) processPendingExport() {
	ui.exportTimer = nil
	if ui.pendingExport == nil {
		return
	}
	now := time.Now()
	if ui.state.AccessToken == "" {
		ui.pendingExport = nil
		return
	}
	if ui.uploading || now.Before(ui.cooldownUntil) {
		ui.pendingExport = nil
		remaining := max(ui.cooldownUntil.Sub(now), time.Second)
		ui.notify(fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))), palette.accent, 8*time.Second)
		return
	}
	request := *ui.pendingExport
	if request.Metadata == nil {
		ui.pendingExport = nil
		ui.metadataRequest = request
		ui.metadataError = ""
		ui.levelEditor.SetText("")
		ui.raceSelected = 0
		ui.classSelected = [3]int{}
		ui.modal = "metadata"
		return
	}
	if ui.modal != "" {
		ui.exportTimer = time.AfterFunc(time.Second, func() {
			ui.send(eqldbGUIEvent{kind: eqldbGUIProcessExport})
		})
		return
	}
	ui.pendingExport = nil
	ui.startUpload(request, eqldb.UploadMetadata{
		Level: request.Metadata.Level, Classes: append([]string(nil), request.Metadata.Classes...), Race: request.Metadata.Race,
	})
}

func (ui *eqldbGUI) submitMetadata() {
	level, err := strconv.Atoi(strings.TrimSpace(ui.levelEditor.Text()))
	if err != nil || level < 1 {
		ui.metadataError = "Enter a valid level."
		return
	}
	if ui.raceSelected == 0 {
		ui.metadataError = "Select a race."
		return
	}
	race := eqldbGUIRaceOptions[ui.raceSelected]
	if ui.classSelected[0] == 0 || ui.classSelected[1] == 0 {
		ui.metadataError = "Select at least the primary and second classes."
		return
	}
	seen := make(map[string]bool)
	classes := make([]string, 0, 3)
	for _, selected := range ui.classSelected {
		if selected == 0 {
			continue
		}
		code := strings.Fields(eqldbGUIClassOptions[selected])[0]
		if seen[code] {
			ui.metadataError = "Classes must be distinct."
			return
		}
		seen[code] = true
		classes = append(classes, code)
	}
	request := ui.metadataRequest
	ui.closeModal()
	ui.startUpload(request, eqldb.UploadMetadata{Level: level, Classes: classes, Race: race})
}

func (ui *eqldbGUI) startUpload(request inventorysync.Request, metadata eqldb.UploadMetadata) {
	now := time.Now()
	acquired, remaining, err := ui.store.AcquireUploadLease(now, eqldbGUIUploadCooldown)
	if err != nil {
		ui.lastError = err.Error()
		ui.notify("EQLDB upload failed — see Tools → EQLDB connection", color.NRGBA{R: 220, G: 150, B: 150, A: 255}, 12*time.Second)
		return
	}
	if !acquired {
		ui.cooldownUntil = maxTimeGUI(ui.cooldownUntil, now.Add(remaining))
		ui.notify(fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))), palette.accent, 8*time.Second)
		return
	}
	ui.uploading = true
	ui.cooldownUntil = now.Add(eqldbGUIUploadCooldown)
	token := ui.state.AccessToken
	ui.notify("EQLDB: uploading inventory…", palette.text, 8*time.Second)
	go func() {
		result, err := ui.client.UploadInventory(ui.context, token, request.Path, metadata)
		if err != nil {
			ui.send(eqldbGUIEvent{kind: eqldbGUIUploadFailed, err: err})
			return
		}
		ui.send(eqldbGUIEvent{kind: eqldbGUIUploadDone, result: result})
	}()
}

func (ui *eqldbGUI) handleUploadError(err error) {
	ui.lastError = err.Error()
	var apiErr *eqldb.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusUnauthorized {
			ui.state.AccessToken = ""
			ui.state.ConnectionID = ""
			if saveErr := ui.store.Save(ui.state); saveErr != nil {
				ui.lastError += "; " + saveErr.Error()
			}
			ui.notify("EQLDB: connection was revoked — reconnect under Tools", color.NRGBA{R: 220, G: 150, B: 150, A: 255}, 12*time.Second)
			return
		}
		if apiErr.RetryAfter > 0 {
			ui.cooldownUntil = maxTimeGUI(ui.cooldownUntil, time.Now().Add(apiErr.RetryAfter))
		}
	}
	ui.notify("EQLDB upload failed — see Tools → EQLDB connection", color.NRGBA{R: 220, G: 150, B: 150, A: 255}, 12*time.Second)
}

func (s *shell) layoutEQLDB(gtx layout.Context) layout.Dimensions {
	if s.eqldb == nil || s.eqldb.modal == "" {
		return layout.Dimensions{}
	}
	return s.eqldb.Layout(gtx, s.theme)
}

func (ui *eqldbGUI) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	paint.Fill(gtx.Ops, color.NRGBA{A: 180})
	if ui.dialogModal != ui.modal {
		ui.dialogModal = ui.modal
		ui.dialogList.ScrollTo(0)
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		margin := gtx.Dp(unit.Dp(32))
		width := min(gtx.Dp(unit.Dp(690)), max(gtx.Constraints.Max.X-margin, 1))
		height := min(gtx.Dp(unit.Dp(500)), max(gtx.Constraints.Max.Y-margin, 1))
		if ui.modal == "metadata" {
			height = min(gtx.Dp(unit.Dp(560)), max(gtx.Constraints.Max.Y-margin, 1))
		}
		gtx.Constraints.Min = image.Pt(width, height)
		gtx.Constraints.Max = gtx.Constraints.Min
		return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.panel)
			return layout.UniformInset(unit.Dp(25)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutModalContent(gtx, theme)
			})
		})
	})
}

func (ui *eqldbGUI) layoutModalContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	switch ui.modal {
	case "intro":
		closeLabel := "Close"
		if ui.introTimer {
			remaining := max(ui.introDeadline.Sub(time.Now()), 0)
			closeLabel = fmt.Sprintf("Close (%ds)", int(math.Ceil(remaining.Seconds())))
		}
		return ui.layoutTextDialog(gtx, theme,
			"Connect eqdps to EQLDB",
			"EQLDB can turn EverQuest Legends inventory exports into shareable character profiles.\n\nAfter connecting, eqdps can upload an inventory automatically whenever the game finishes exporting it. No game or EQLDB password is entered into eqdps.\n\n"+
				eqldbGUIMacroExplanation+"\n\nYou can connect later through Tools → EQLDB connection.",
			[]dialogButton{{"Connect to EQLDB", &ui.connectClick, true}, {closeLabel, &ui.closeClick, false}},
			true,
		)
	case "manage":
		if ui.state.AccessToken == "" {
			body := "EQLDB is not connected.\n\nConnect eqdps to upload inventory exports to your EQLDB account automatically.\n\n" +
				eqldbGUIMacroExplanation
			if ui.lastError != "" {
				body += "\n\nLast error: " + ui.lastError
			}
			return ui.layoutTextDialog(gtx, theme, "EQLDB connection", body,
				[]dialogButton{{"Connect", &ui.connectClick, true}, {"Close", &ui.closeClick, false}}, true)
		}
		body := "EQLDB is connected.\n\nNew inventory exports will be uploaded automatically.\n\n" +
			eqldbGUIMacroExplanation +
			"\n\nRevoke access from EQLDB’s Connected apps page when needed."
		if ui.lastError != "" {
			body += "\n\nLast error: " + ui.lastError
		}
		buttons := []dialogButton{{"Forget on this computer", &ui.forgetClick, false}, {"Close", &ui.closeClick, false}}
		return ui.layoutTextDialog(gtx, theme, "EQLDB connection", body, buttons, true)
	case "auth":
		body := "Requesting a connection code from EQLDB…"
		buttons := []dialogButton{{"Cancel", &ui.cancelClick, false}}
		if !ui.authInfo.ExpiresAt.IsZero() {
			body = fmt.Sprintf(
				"Approve the connection in your browser.\n\nCode: %s\nURL: %s\n\nWaiting for approval…\nExpires in %s",
				ui.authInfo.UserCode,
				ui.authInfo.VerificationURIComplete,
				formatCountdownGUI(max(ui.authInfo.ExpiresAt.Sub(time.Now()), 0)),
			)
			if ui.authExtra != "" {
				body += "\n\n" + ui.authExtra
			}
			buttons = []dialogButton{{"Open browser again", &ui.browserClick, true}, {"Cancel", &ui.cancelClick, false}}
		}
		return ui.layoutTextDialog(gtx, theme, "Connect eqdps to EQLDB", body, buttons, false)
	case "connected":
		return ui.layoutTextDialog(gtx, theme, "EQLDB connected",
			"eqdps is connected to EQLDB.\n\nNew inventory exports will be uploaded automatically.",
			[]dialogButton{{"Close", &ui.closeClick, true}}, true)
	case "error":
		return ui.layoutTextDialog(gtx, theme, "EQLDB connection failed", ui.lastError,
			[]dialogButton{{"Try again", &ui.retryClick, true}, {"Close", &ui.closeClick, false}}, false)
	case "metadata":
		return ui.layoutMetadata(gtx, theme)
	default:
		return layout.Dimensions{}
	}
}

type dialogButton struct {
	label   string
	click   *widget.Clickable
	primary bool
}

func (ui *eqldbGUI) layoutTextDialog(gtx layout.Context, theme *material.Theme, title, body string, buttons []dialogButton, showMacro bool) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, theme, title, unit.Sp(23), palette.text, text.Start, font.SemiBold)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			count := 1
			if showMacro {
				count = 2
			}
			list := material.List(theme, &ui.dialogList)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			list.Indicator.HoverColor = palette.text
			return list.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
				if index == 0 {
					return inset(unit.Dp(4), unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, theme, body, unit.Sp(16), palette.text, text.Start)
					})
				}
				return ui.layoutMacroField(gtx, theme)
			})
		}),
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutDialogButtons(gtx, theme, buttons)
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (ui *eqldbGUI) layoutDialogButtons(gtx layout.Context, theme *material.Theme, buttons []dialogButton) layout.Dimensions {
	return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(buttons))
		for index := range buttons {
			button := buttons[index]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return inset(unit.Dp(5), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return eqldbDialogButton(gtx, theme, button.click, button.label, button.primary)
				})
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (ui *eqldbGUI) layoutMacroField(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	height := gtx.Dp(unit.Dp(72))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return inset(0, unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				editor := material.Editor(theme, &ui.macroEditor, "")
				editor.TextSize = unit.Sp(15)
				editor.Color = palette.text
				return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
					fill(gtx, palette.window)
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, editor.Layout)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return inset(unit.Dp(7), 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return eqldbDialogButton(gtx, theme, &ui.copyClick, "Copy macro", true)
				})
			}),
		)
	})
}

func (ui *eqldbGUI) layoutMetadata(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	instructions := fmt.Sprintf(
		"No recent /who result was found for this export. Enter the current loadout, or use this EverQuest macro next time:\n/who %s\n/outputfile inventory",
		ui.character,
	)
	items := []int{0, 1}
	ui.pickerItem = -1
	appendField := func(item, picker, fieldSlot int) {
		if ui.classPicker == picker && ui.pickerAbove {
			ui.pickerItem = len(items)
			items = append(items, 6)
		}
		ui.metadataFields[fieldSlot] = len(items)
		items = append(items, item)
		if ui.classPicker == picker && !ui.pickerAbove {
			ui.pickerItem = len(items)
			items = append(items, 6)
		}
	}
	appendField(2, -2, 0)
	for class := 0; class < 3; class++ {
		appendField(3+class, class, class+1)
	}
	if ui.metadataError != "" {
		items = append(items, 7)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, theme, "Inventory metadata needed", unit.Sp(23), palette.text, text.Start, font.SemiBold)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			list := material.List(theme, &ui.metadataList)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			list.Indicator.HoverColor = palette.text
			return list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				switch item := items[index]; item {
				case 0:
					return inset(unit.Dp(4), unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, theme, instructions, unit.Sp(14), palette.muted, text.Start)
					})
				case 1:
					return ui.layoutMetadataRow(gtx, theme, "Level", func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(theme, &ui.levelEditor, "")
						editor.TextSize = unit.Sp(16)
						editor.Color = palette.text
						return outline(gtx, palette.line, func(gtx layout.Context) layout.Dimensions {
							fill(gtx, palette.window)
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, editor.Layout)
						})
					})
				case 2:
					return ui.layoutMetadataRow(gtx, theme, "Race", func(gtx layout.Context) layout.Dimensions {
						return eqldbSelector(gtx, theme, &ui.raceClick, eqldbGUIRaceOptions[ui.raceSelected], ui.classPicker == -2)
					})
				case 3, 4, 5:
					class := item - 3
					labels := []string{"Primary class", "Second class", "Third class"}
					return ui.layoutMetadataRow(gtx, theme, labels[class], func(gtx layout.Context) layout.Dimensions {
						return eqldbSelector(gtx, theme, &ui.classClicks[class], eqldbGUIClassOptions[ui.classSelected[class]], ui.classPicker == class)
					})
				case 6:
					return ui.layoutInlinePicker(gtx, theme)
				case 7:
					return inset(unit.Dp(4), unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, theme, ui.metadataError, unit.Sp(14), color.NRGBA{R: 220, G: 150, B: 150, A: 255}, text.Start)
					})
				default:
					return layout.Dimensions{}
				}
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutDialogButtons(gtx, theme, []dialogButton{
				{"Upload", &ui.uploadClick, true},
				{"Cancel", &ui.cancelClick, false},
			})
		}),
	)
}

func (ui *eqldbGUI) layoutMetadataRow(gtx layout.Context, theme *material.Theme, name string, control layout.Widget) layout.Dimensions {
	height := gtx.Dp(unit.Dp(54))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return inset(unit.Dp(4), unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(145))
				gtx.Constraints.Max.X = gtx.Constraints.Min.X
				return label(gtx, theme, name, unit.Sp(15), palette.text, text.Start)
			}),
			layout.Flexed(1, control),
		)
	})
}

func (ui *eqldbGUI) layoutInlinePicker(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	options := eqldbGUIClassOptions
	choices := ui.classChoices
	selected := 0
	if ui.classPicker == -2 {
		options = eqldbGUIRaceOptions
		choices = ui.raceChoices
		selected = ui.raceSelected
	} else if ui.classPicker >= 0 {
		selected = ui.classSelected[ui.classPicker]
	}
	height := min(gtx.Dp(unit.Dp(230)), max(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(120))))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	return inset(unit.Dp(149), unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return outline(gtx, palette.accent, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, palette.window)
			list := material.List(theme, &ui.pickerList)
			list.AnchorStrategy = material.Occupy
			list.Indicator.Color = palette.muted
			list.Indicator.HoverColor = palette.text
			return list.Layout(gtx, len(options), func(gtx layout.Context, index int) layout.Dimensions {
				rowHeight := gtx.Dp(unit.Dp(44))
				gtx.Constraints.Min.Y = rowHeight
				gtx.Constraints.Max.Y = rowHeight
				return choices[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					pointer.CursorPointer.Add(gtx.Ops)
					background := palette.panel
					foreground := palette.text
					if index == selected {
						background = color.NRGBA{R: 70, G: 60, B: 34, A: 255}
						foreground = palette.accent
					} else if choices[index].Hovered() {
						background = palette.panelAlt
					}
					fill(gtx, background)
					value := options[index]
					if index == selected {
						value = ">  " + value
					}
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label(gtx, theme, value, unit.Sp(15), foreground, text.Start)
					})
				})
			})
		})
	})
}

func eqldbDialogButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, primary bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		background, foreground := palette.panelAlt, palette.text
		if primary {
			background, foreground = color.NRGBA{R: 70, G: 60, B: 34, A: 255}, palette.accent
		}
		fill(gtx, background)
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, theme, value, unit.Sp(15), foreground, text.Middle, font.SemiBold)
		})
	})
}

func eqldbSelector(gtx layout.Context, theme *material.Theme, click *widget.Clickable, value string, open bool) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		background := palette.panelAlt
		if open {
			background = color.NRGBA{R: 45, G: 43, B: 35, A: 255}
		}
		fill(gtx, background)
		return layout.UniformInset(unit.Dp(9)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, theme, value, unit.Sp(15), palette.text, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					arrow := "v"
					if open {
						arrow = "^"
					}
					return label(gtx, theme, arrow, unit.Sp(15), palette.accent, text.End)
				}),
			)
		})
	})
}

func formatCountdownGUI(duration time.Duration) string {
	seconds := max(int(math.Ceil(duration.Seconds())), 0)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

const eqldbGUIMacroExplanation = "The best way to export is a two-line EverQuest macro. This lets eqdps detect level, race, and classes automatically."

func eqldbGUIMacroText(character string) string {
	if character == "" {
		character = "CHARACTERNAME"
	}
	return "/who " + character + "\n/outputfile inventory"
}

func maxTimeGUI(first, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}
