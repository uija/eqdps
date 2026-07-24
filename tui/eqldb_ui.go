package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/uija/eqdps/internal/eqldb"
	"github.com/uija/eqdps/internal/eqlog"
	"github.com/uija/eqdps/internal/inventorysync"
	"github.com/uija/eqdps/internal/platform"
)

const (
	eqldbIntroDuration  = 30 * time.Second
	eqldbExportGrace    = 2 * time.Second
	eqldbUploadCooldown = 15 * time.Second
)

var eqldbClassOptions = []string{
	"",
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

var eqldbRaceOptions = append([]string{""}, eqldb.ManualRaces...)

type eqldbTUI struct {
	app       *tview.Application
	pages     *tview.Pages
	mainFocus tview.Primitive
	store     eqldb.Store
	client    *eqldb.Client
	state     eqldb.State
	observer  *inventorysync.Observer
	notify    func(string, tcell.Color, time.Duration)
	context   context.Context
	cancel    context.CancelFunc
	modal     string
	lastError string
	character string

	introAttempted bool
	introDeadline  time.Time
	introTimer     bool
	introClose     *tview.Button

	authSequence int
	authCancel   context.CancelFunc
	authInfo     eqldb.DeviceAuthorization
	authText     *tview.TextView

	pendingExport *inventorysync.Request
	exportTimer   *time.Timer
	cooldownUntil time.Time
	uploading     bool
}

func newEQLDBTUI(app *tview.Application, pages *tview.Pages, mainFocus tview.Primitive, logPath string, notify func(string, tcell.Color, time.Duration)) (*eqldbTUI, error) {
	store, err := eqldb.DefaultStore()
	if err != nil {
		return nil, err
	}
	state, loadErr := store.Load()
	observer, err := inventorysync.NewObserver(logPath)
	if err != nil {
		return nil, err
	}
	character, _, err := inventorysync.CharacterIdentity(logPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ui := &eqldbTUI{
		app:       app,
		pages:     pages,
		mainFocus: mainFocus,
		store:     store,
		client:    eqldb.NewClient(),
		state:     state,
		observer:  observer,
		notify:    notify,
		context:   ctx,
		cancel:    cancel,
		character: character,
	}
	if loadErr != nil {
		ui.lastError = loadErr.Error()
	}
	return ui, nil
}

func (ui *eqldbTUI) Close() {
	ui.cancel()
	if ui.authCancel != nil {
		ui.authCancel()
		ui.authCancel = nil
	}
	if ui.exportTimer != nil {
		ui.exportTimer.Stop()
		ui.exportTimer = nil
	}
}

func (ui *eqldbTUI) Observe(record eqlog.Record) {
	request, ok := ui.observer.Observe(record)
	if !ok {
		return
	}
	select {
	case <-ui.context.Done():
		return
	default:
	}
	ui.app.QueueUpdateDraw(func() {
		ui.scheduleExport(request, time.Now())
	})
}

func (ui *eqldbTUI) Tick(now time.Time) {
	if !ui.introAttempted && !ui.state.IntroductionShown && ui.state.AccessToken == "" && ui.modal == "" {
		page, _ := ui.pages.GetFrontPage()
		if page == "main" {
			ui.showIntroduction(now)
		}
	}
	if ui.modal == "eqldb-intro" {
		if ui.introTimer {
			remaining := ui.introDeadline.Sub(now)
			if remaining <= 0 {
				ui.closeModal()
			} else if ui.introClose != nil {
				ui.introClose.SetLabel(fmt.Sprintf("Close (%ds)", int(math.Ceil(remaining.Seconds()))))
			}
		}
	}
	if ui.modal == "eqldb-auth" && ui.authText != nil && !ui.authInfo.ExpiresAt.IsZero() {
		ui.updateAuthText(now, "")
	}
}

func (ui *eqldbTUI) ModalOpen() bool {
	return ui.modal != ""
}

func (ui *eqldbTUI) OpenManagement() {
	if ui.modal != "" {
		return
	}
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" EQLDB connection ")
	form.SetButtonsAlign(tview.AlignCenter)
	if ui.state.AccessToken == "" {
		text := "EQLDB is not connected.\n\nConnect eqdps to upload inventory exports to your EQLDB account automatically.\n\n" +
			eqldbMacroExplanation
		if ui.lastError != "" {
			text += "\n\nLast error: " + ui.lastError
		}
		form.AddTextView("", text, 68, 8, true, false)
		addEQLDBMacroField(form, ui.character, nil)
		form.AddButton("Connect", func() {
			ui.markIntroductionShown()
			ui.startAuthentication()
		})
		form.AddButton("Close", ui.closeModal)
	} else {
		text := "EQLDB is connected.\n\nNew inventory exports will be uploaded automatically.\n\n" +
			eqldbMacroExplanation +
			"\n\nRevoke access from EQLDB's Connected apps page when needed."
		if ui.lastError != "" {
			text += "\n\nLast error: " + ui.lastError
		}
		form.AddTextView("", text, 68, 10, true, false)
		addEQLDBMacroField(form, ui.character, nil)
		form.AddButton("Forget on This Computer", func() {
			ui.state.AccessToken = ""
			ui.state.ConnectionID = ""
			ui.lastError = ""
			if err := ui.store.Save(ui.state); err != nil {
				ui.lastError = err.Error()
			}
			ui.closeModal()
			ui.notify("EQLDB connection removed from this computer", infoBarColor, 8*time.Second)
		})
		form.AddButton("Close", ui.closeModal)
	}
	form.SetCancelFunc(ui.closeModal)
	ui.showModal("eqldb-manage", centeredPrimitive(form, 82, 22), form)
}

func (ui *eqldbTUI) showIntroduction(now time.Time) {
	ui.introAttempted = true
	ui.introDeadline = now.Add(eqldbIntroDuration)
	ui.introTimer = true
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Connect eqdps to EQLDB ")
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddTextView("",
		"EQLDB can turn EverQuest Legends inventory exports into shareable character profiles.\n\n"+
			"After connecting, eqdps can upload an inventory automatically whenever the game finishes exporting it. "+
			"No game or EQLDB password is entered into eqdps.\n\n"+
			eqldbMacroExplanation+"\n\n"+
			"You can connect later at any time by pressing e.",
		72, 11, true, false,
	)
	addEQLDBMacroField(form, ui.character, ui.stopIntroductionTimer)
	form.AddButton("Connect to EQLDB", func() {
		ui.markIntroductionShown()
		ui.startAuthentication()
	})
	form.AddButton("Close (30s)", func() {
		ui.markIntroductionShown()
		ui.closeModal()
	})
	ui.introClose = form.GetButton(2)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		ui.stopIntroductionTimer()
		return event
	})
	form.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		ui.stopIntroductionTimer()
		return action, event
	})
	form.SetCancelFunc(func() {
		ui.markIntroductionShown()
		ui.closeModal()
	})
	ui.showModal("eqldb-intro", centeredPrimitive(form, 84, 23), form)
}

func (ui *eqldbTUI) markIntroductionShown() {
	if ui.state.IntroductionShown {
		return
	}
	ui.state.IntroductionShown = true
	if err := ui.store.Save(ui.state); err != nil {
		ui.lastError = err.Error()
	}
}

func (ui *eqldbTUI) stopIntroductionTimer() {
	if ui.modal != "eqldb-intro" || !ui.introTimer {
		return
	}
	ui.introTimer = false
	if ui.introClose != nil {
		ui.introClose.SetLabel("Close")
	}
}

func (ui *eqldbTUI) startAuthentication() {
	ui.closeModal()
	ui.authSequence++
	sequence := ui.authSequence
	ctx, cancel := context.WithCancel(ui.context)
	ui.authCancel = cancel
	ui.authInfo = eqldb.DeviceAuthorization{}

	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Connect eqdps to EQLDB ")
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddTextView("", "Requesting a connection code from EQLDB…", 72, 10, true, false)
	ui.authText, _ = form.GetFormItem(0).(*tview.TextView)
	form.AddButton("Open Browser Again", func() {
		if ui.authInfo.VerificationURIComplete != "" {
			if err := platform.OpenURL(ui.authInfo.VerificationURIComplete); err != nil {
				ui.updateAuthText(time.Now(), "Could not open the browser: "+err.Error())
			}
		}
	})
	form.AddButton("Cancel", ui.cancelAuthentication)
	form.SetCancelFunc(ui.cancelAuthentication)
	ui.showModal("eqldb-auth", centeredPrimitive(form, 82, 18), form)

	go func() {
		defer cancel()
		authorization, err := ui.client.StartConnection(ctx, "eqdps TUI")
		ui.app.QueueUpdateDraw(func() {
			if sequence != ui.authSequence || ui.modal != "eqldb-auth" {
				return
			}
			if err != nil {
				ui.authCancel = nil
				ui.lastError = err.Error()
				ui.showAuthenticationError(err)
				return
			}
			ui.authInfo = authorization
			ui.updateAuthText(time.Now(), "")
			if openErr := platform.OpenURL(authorization.VerificationURIComplete); openErr != nil {
				ui.updateAuthText(time.Now(), "Could not open the browser: "+openErr.Error())
			}
		})
		if err != nil {
			return
		}
		token, err := ui.client.WaitForToken(ctx, authorization)
		ui.app.QueueUpdateDraw(func() {
			if sequence != ui.authSequence || ui.modal != "eqldb-auth" {
				return
			}
			ui.authCancel = nil
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				ui.lastError = err.Error()
				ui.showAuthenticationError(err)
				return
			}
			ui.state.IntroductionShown = true
			ui.state.AccessToken = token.AccessToken
			ui.state.ConnectionID = token.ConnectionID
			ui.lastError = ""
			if saveErr := ui.store.Save(ui.state); saveErr != nil {
				ui.lastError = saveErr.Error()
				ui.showAuthenticationError(fmt.Errorf("connected, but could not save the access token: %w", saveErr))
				return
			}
			ui.showConnected()
		})
	}()
}

func (ui *eqldbTUI) updateAuthText(now time.Time, extra string) {
	if ui.authText == nil {
		return
	}
	remaining := max(ui.authInfo.ExpiresAt.Sub(now), 0)
	text := fmt.Sprintf(
		"Approve the connection in your browser.\n\nCode: %s\nURL:  %s\n\nWaiting for approval…\nExpires in %s",
		ui.authInfo.UserCode,
		ui.authInfo.VerificationURIComplete,
		formatCountdown(remaining),
	)
	if extra != "" {
		text += "\n\n" + extra
	}
	ui.authText.SetText(text)
}

func (ui *eqldbTUI) showAuthenticationError(err error) {
	ui.authText = nil
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" EQLDB connection failed ")
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddTextView("", err.Error(), 68, 6, true, false)
	form.AddButton("Try Again", ui.startAuthentication)
	form.AddButton("Close", ui.closeModal)
	form.SetCancelFunc(ui.closeModal)
	ui.replaceModal("eqldb-auth", centeredPrimitive(form, 78, 13), form)
}

func (ui *eqldbTUI) showConnected() {
	ui.authText = nil
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" EQLDB connected ")
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddTextView("", "eqdps is connected to EQLDB.\n\nNew inventory exports will be uploaded automatically.", 68, 6, true, false)
	form.AddButton("Close", ui.closeModal)
	form.SetCancelFunc(ui.closeModal)
	ui.replaceModal("eqldb-auth", centeredPrimitive(form, 78, 13), form)
	ui.notify("EQLDB connected", infoNoticeColor, 8*time.Second)
}

func (ui *eqldbTUI) cancelAuthentication() {
	ui.authSequence++
	if ui.authCancel != nil {
		ui.authCancel()
		ui.authCancel = nil
	}
	ui.authText = nil
	ui.closeModal()
}

func (ui *eqldbTUI) scheduleExport(request inventorysync.Request, now time.Time) {
	if ui.state.AccessToken == "" {
		return
	}
	if ui.uploading || now.Before(ui.cooldownUntil) {
		remaining := max(time.Until(ui.cooldownUntil), time.Second)
		ui.notify(
			fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))),
			eqldbWarningColor,
			8*time.Second,
		)
		return
	}
	copy := request
	ui.pendingExport = &copy
	if ui.exportTimer != nil {
		ui.exportTimer.Stop()
	}
	ui.exportTimer = time.AfterFunc(eqldbExportGrace, func() {
		select {
		case <-ui.context.Done():
			return
		default:
		}
		ui.app.QueueUpdateDraw(ui.processPendingExport)
	})
	ui.notify("EQLDB: inventory export detected…", infoBarColor, 4*time.Second)
}

func (ui *eqldbTUI) processPendingExport() {
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
		ui.notify(
			fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))),
			eqldbWarningColor,
			8*time.Second,
		)
		return
	}
	request := *ui.pendingExport
	if request.Metadata == nil {
		ui.pendingExport = nil
		if ui.modal != "" {
			ui.closeModal()
		}
		ui.showMetadataForm(request)
		return
	}
	if ui.modal != "" {
		ui.exportTimer = time.AfterFunc(time.Second, func() {
			select {
			case <-ui.context.Done():
				return
			default:
			}
			ui.app.QueueUpdateDraw(ui.processPendingExport)
		})
		return
	}
	ui.pendingExport = nil
	ui.startUpload(request, eqldb.UploadMetadata{
		Level:   request.Metadata.Level,
		Classes: append([]string(nil), request.Metadata.Classes...),
		Race:    request.Metadata.Race,
	})
}

func (ui *eqldbTUI) showMetadataForm(request inventorysync.Request) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Inventory metadata needed ")
	form.SetButtonsAlign(tview.AlignCenter)
	instructions := fmt.Sprintf(
		"No recent /who result was found for this export.\n\nFor automatic detection, use this EverQuest macro:\n/who %s\n/outputfile inventory",
		ui.character,
	)
	form.AddTextView("", instructions, 72, 7, true, false)
	form.AddInputField("Level", "", 8, func(textToCheck string, lastChar rune) bool {
		return lastChar >= '0' && lastChar <= '9'
	}, nil)
	form.AddDropDown("Race", eqldbRaceOptions, 0, nil)
	form.AddDropDown("Primary class", eqldbClassOptions, 0, nil)
	form.AddDropDown("Second class", eqldbClassOptions, 0, nil)
	form.AddDropDown("Third class", eqldbClassOptions, 0, nil)
	form.AddButton("Upload", func() {
		levelText := strings.TrimSpace(form.GetFormItem(1).(*tview.InputField).GetText())
		level, err := strconv.Atoi(levelText)
		if err != nil || level < 1 {
			form.SetTitle(" Inventory metadata needed — enter a valid level ")
			return
		}
		_, race := form.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
		if race == "" {
			form.SetTitle(" Inventory metadata needed — select a race ")
			return
		}
		classes := make([]string, 0, 3)
		seen := make(map[string]bool)
		for index := 3; index <= 5; index++ {
			_, option := form.GetFormItem(index).(*tview.DropDown).GetCurrentOption()
			class := classCode(option)
			if class == "" {
				continue
			}
			if seen[class] {
				form.SetTitle(" Inventory metadata needed — classes must be distinct ")
				return
			}
			seen[class] = true
			classes = append(classes, class)
		}
		_, primaryOption := form.GetFormItem(3).(*tview.DropDown).GetCurrentOption()
		_, secondOption := form.GetFormItem(4).(*tview.DropDown).GetCurrentOption()
		if classCode(primaryOption) == "" || classCode(secondOption) == "" {
			form.SetTitle(" Inventory metadata needed — select primary and second classes ")
			return
		}
		ui.closeModal()
		ui.startUpload(request, eqldb.UploadMetadata{Level: level, Classes: classes, Race: race})
	})
	form.AddButton("Cancel", ui.closeModal)
	form.SetCancelFunc(ui.closeModal)
	ui.showModal("eqldb-metadata", centeredPrimitive(form, 84, 22), form)
}

func (ui *eqldbTUI) startUpload(request inventorysync.Request, metadata eqldb.UploadMetadata) {
	now := time.Now()
	acquired, remaining, err := ui.store.AcquireUploadLease(now, eqldbUploadCooldown)
	if err != nil {
		ui.lastError = err.Error()
		ui.notify("EQLDB upload failed — press e for details", eqldbErrorColor, 12*time.Second)
		return
	}
	if !acquired {
		ui.cooldownUntil = maxTime(ui.cooldownUntil, now.Add(remaining))
		ui.notify(
			fmt.Sprintf("EQLDB: export ignored — please wait %d seconds", int(math.Ceil(remaining.Seconds()))),
			eqldbWarningColor,
			8*time.Second,
		)
		return
	}
	ui.uploading = true
	ui.cooldownUntil = now.Add(eqldbUploadCooldown)
	accessToken := ui.state.AccessToken
	ui.notify("EQLDB: uploading inventory…", infoBarColor, 8*time.Second)

	go func() {
		result, err := ui.client.UploadInventory(ui.context, accessToken, request.Path, metadata)
		ui.app.QueueUpdateDraw(func() {
			ui.uploading = false
			if err != nil {
				ui.handleUploadError(err)
				return
			}
			ui.lastError = ""
			if result.Status == "pending" {
				message := "EQLDB: inventory uploaded — loadout assignment needed"
				if result.Message != "" {
					message = "EQLDB: " + result.Message
				}
				ui.notify(message, eqldbWarningColor, 12*time.Second)
				return
			}
			ui.notify(
				fmt.Sprintf("EQLDB: %s inventory uploaded", result.Character),
				infoNoticeColor,
				10*time.Second,
			)
		})
	}()
}

func (ui *eqldbTUI) handleUploadError(err error) {
	ui.lastError = err.Error()
	var apiErr *eqldb.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusUnauthorized {
			ui.state.AccessToken = ""
			ui.state.ConnectionID = ""
			if saveErr := ui.store.Save(ui.state); saveErr != nil {
				ui.lastError += "; " + saveErr.Error()
			}
			ui.notify("EQLDB: connection was revoked — press e to reconnect", eqldbErrorColor, 12*time.Second)
			return
		}
		if apiErr.RetryAfter > 0 {
			ui.cooldownUntil = maxTime(ui.cooldownUntil, time.Now().Add(apiErr.RetryAfter))
		}
	}
	ui.notify("EQLDB upload failed — press e for details", eqldbErrorColor, 12*time.Second)
}

func (ui *eqldbTUI) showModal(name string, primitive, focus tview.Primitive) {
	ui.modal = name
	ui.pages.AddPage(name, primitive, true, true)
	ui.app.SetFocus(focus)
}

func (ui *eqldbTUI) replaceModal(name string, primitive, focus tview.Primitive) {
	ui.pages.RemovePage(name)
	ui.modal = ""
	ui.showModal(name, primitive, focus)
}

func (ui *eqldbTUI) closeModal() {
	if ui.modal == "" {
		return
	}
	ui.pages.RemovePage(ui.modal)
	ui.modal = ""
	ui.introTimer = false
	ui.introClose = nil
	ui.app.SetFocus(ui.mainFocus)
}

func centeredPrimitive(primitive tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(primitive, width, 0, true).
				AddItem(nil, 0, 1, false),
			height, 0, true,
		).
		AddItem(nil, 0, 1, false)
}

func classCode(option string) string {
	fields := strings.Fields(option)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

const eqldbMacroExplanation = "The best way to export is a two-line EverQuest macro. This lets eqdps detect your level, race, and classes automatically. Focus the field to select with Shift+arrows; Ctrl+L selects all and Ctrl+Q copies."

func eqldbMacroText(character string) string {
	return "/who " + character + "\n/outputfile inventory"
}

func addEQLDBMacroField(form *tview.Form, character string, onInteract func()) {
	form.SetFieldBackgroundColor(tcell.NewHexColor(0x202428))
	field := tview.NewTextArea().
		SetText(eqldbMacroText(character), false).
		SetSize(2, 0)
	field.SetLabel("Macro ")
	field.SetClipboard(func(text string) {
		if err := platform.CopyText(text); err != nil {
			form.SetTitle(" EQLDB connection — copy failed: " + err.Error() + " ")
			return
		}
		form.SetTitle(" EQLDB connection — selection copied ")
	}, nil)
	field.SetMovedFunc(func() {
		if onInteract != nil {
			onInteract()
		}
	})
	field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if onInteract != nil {
			onInteract()
		}
		switch event.Key() {
		case tcell.KeyRune,
			tcell.KeyEnter,
			tcell.KeyBackspace,
			tcell.KeyBackspace2,
			tcell.KeyDelete,
			tcell.KeyCtrlD,
			tcell.KeyCtrlK,
			tcell.KeyCtrlU,
			tcell.KeyCtrlV,
			tcell.KeyCtrlW,
			tcell.KeyCtrlX,
			tcell.KeyCtrlY,
			tcell.KeyCtrlZ:
			return nil
		default:
			return event
		}
	})
	form.AddFormItem(field)
	form.AddButton("Copy Macro", func() {
		if err := platform.CopyText(eqldbMacroText(character)); err != nil {
			form.SetTitle(" EQLDB connection — copy failed: " + err.Error() + " ")
			return
		}
		form.SetTitle(" EQLDB connection — macro copied ")
	})
}

func formatCountdown(duration time.Duration) string {
	seconds := max(int(math.Ceil(duration.Seconds())), 0)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func maxTime(first, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}
