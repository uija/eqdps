package eqldb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

const (
	UPLOAD_WHO_TIMEOUT          = 5 * time.Second
	UPLOAD_NOTIFICATION_TIMEOUT = 5 * time.Second
)

type ClassInfo struct {
	Level     int
	Classes   []string
	Race      string
	Timestamp time.Time
}

type UploadData struct {
	Filename  string
	Timestamp time.Time
}

type Module struct {
	mu         sync.RWMutex
	ctx        *module.Context
	invalidate func()

	replay atomic.Bool

	start_authorization_click  widget.Clickable
	revoke_authorization_click widget.Clickable

	process_stage     int
	authorization     DeviceAuthorization
	connection_error  string
	connection_cancel context.CancelFunc

	current_charname string
	current_logpath  string

	last_who_result *ClassInfo
	last_export     *UploadData
	stop            chan struct{}

	upload_status int
	upload_timer  time.Time
}

func NewModule() *Module {
	return &Module{
		process_stage: connectionIdle,
		stop:          make(chan struct{}),
		upload_status: -1,
	}
}
func (m *Module) Init(ctx *module.Context, invalidate func()) error {
	ctx.RegisterLogRow(m.OnLogRow)
	ctx.RegisterUpdate(m.Update)
	ctx.RegisterReplayStart(m.OnReplayStart)
	ctx.RegisterReplayEnd(m.OnReplayEnd)
	ctx.RegisterLogOpen(m.OnLogOpen)
	ctx.RegisterGeneralStatus(m.GeneralStatus)
	ctx.AddSidebarItem("eqldb", func() {
		ctx.SetMainView(m.Layout)
	})
	m.ctx = ctx
	m.invalidate = invalidate
	go m.CheckUploadData()
	return nil
}
func (m *Module) Update(gtx layout.Context) {
	if m.start_authorization_click.Clicked(gtx) {
		m.StartConnection()
	}
	if m.revoke_authorization_click.Clicked(gtx) {
		m.ctx.Config.EQLDbConfig.AccessToken = ""
		m.ctx.Config.EQLDbConfig.AuthorizationTime = time.Time{}
		m.ctx.Config.Save()
	}
}
func (m *Module) OnLogOpen(characterName string, serverName string, size int64, path string) bool {
	m.current_charname = characterName
	m.current_logpath = path
	return true
}
func (m *Module) CheckUploadData() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			if m.upload_status >= 0 {
				if m.upload_timer.Add(UPLOAD_NOTIFICATION_TIMEOUT).Before(now) {
					m.upload_status = -1
					m.upload_timer = time.Time{}
					m.invalidate()
				}
			}
			m.mu.Lock()
			if m.last_export != nil {
				if m.last_who_result != nil {
					m.UploadFile()
					m.last_export = nil
					m.last_who_result = nil
				} else if m.last_export.Timestamp.Add(UPLOAD_WHO_TIMEOUT).Before(now) {
					m.UploadFile()
					m.last_export = nil
					m.last_who_result = nil
				}
			}
			if m.last_who_result != nil {
				if m.last_who_result.Timestamp.Add(UPLOAD_WHO_TIMEOUT).Before(now) {
					m.last_who_result = nil
				}
			}
			m.mu.Unlock()
		case <-m.stop:
			return
		}
	}
}
func (m *Module) OnLogRow(e *data.LogRowEvent) {
	if m.replay.Load() {
		return
	}
	switch e.Type {
	case data.LogRowEventTypeWho:
		name := e.Data[3]
		if !strings.EqualFold(name, m.current_charname) {
			return
		}

		level, err := strconv.Atoi(e.Data[1])
		if err != nil {
			return
		}
		classes := strings.Split(e.Data[2], "/")
		race := e.Data[4]
		m.mu.Lock()
		m.upload_status = upload_detected
		m.upload_timer = time.Now()
		m.invalidate()
		if m.last_who_result == nil {
			m.last_who_result = &ClassInfo{}
		}
		m.last_who_result.Classes = classes
		m.last_who_result.Timestamp = time.Now()
		m.last_who_result.Level = level
		m.last_who_result.Race = race
		m.mu.Unlock()
	case data.LogRowEventTypeInventoryExport:
		m.mu.Lock()
		m.last_export = &UploadData{
			Filename:  e.Data[1],
			Timestamp: time.Now(),
		}
		m.mu.Unlock()
	}
}

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx, ui.PageTitle(style, "eqldb.org integration", gtx).Layout)
				}),
				layout.Rigid(ui.ColoredLabel(style.Theme, 15, style.Palette.Muted,
					"eqldb.org is a website that presents player profiles to others like Magelo does for EQ Live.\nTo use eqldb.org, you upload an export of you profile to the website, then configure your level, race and classes.\neqdps can do that for you. All you need to do is connect eqdps to the website and create a macro:",
				).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					name := m.current_charname
					if name == "" {
						name = "YOURCHARNAME"
					}
					return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						ui.ColoredLabel(style.Theme, 15, style.Palette.Text, fmt.Sprintf("/pause 5, /who %s\n/outputfile inventory", name)).Layout,
					)
				}),
				layout.Rigid(ui.ColoredLabel(style.Theme, 15, style.Palette.Muted,
					"eqdps watches for the message in the log and uploads that data to the website.",
				).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return m.RenderConnectProcess(style, gtx)
				}),
			)
		},
	)
}
func (m *Module) RenderConnectProcess(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(32)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m.mu.RLock()
		stage := m.process_stage
		authorization := m.authorization
		errorMessage := m.connection_error
		accessToken := m.ctx.Config.EQLDbConfig.AccessToken
		authorizationTime := m.ctx.Config.EQLDbConfig.AuthorizationTime
		m.mu.RUnlock()

		if accessToken != "" {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, unit.Sp(15), fmt.Sprintf("You connected eqdps to eqldb.org on %s.", authorizationTime.Format("2006-01-02 15:04"))).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					link := ui.IconLink(style, &m.revoke_authorization_click, ui.Close, "Revoke connection")
					link.TextColor = style.Palette.No
					return link.Layout(gtx)
				}),
			)
		} else if stage == connectionRequesting {
			return material.Label(style.Theme, unit.Sp(15), "Sending request to eqldb.org").Layout(gtx)
		} else if stage == connectionWaiting {
			message := fmt.Sprintf("Please go to\n%s\nWith code: %s", authorization.VerificationURI, authorization.UserCode)
			if errorMessage != "" {
				message += "\n\n" + errorMessage
			}
			return material.Label(style.Theme, unit.Sp(15), message).Layout(gtx)
		} else if stage == connectionError {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Label(style.Theme, unit.Sp(15), errorMessage).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, material.Button(style.Theme, &m.start_authorization_click, "Try again").Layout)
				}),
			)
		} else {
			return material.Button(style.Theme, &m.start_authorization_click, "Connect eqdps with eqldb.org").Layout(gtx)
		}
	})
}
func (m *Module) OnReplayStart() {
	m.replay.Store(true)
}
func (m *Module) OnReplayEnd() {
	m.replay.Store(false)
}
func (m *Module) Shutdown() {
	m.mu.Lock()
	if m.connection_cancel != nil {
		m.connection_cancel()
		m.connection_cancel = nil
	}
	m.mu.Unlock()
	m.stop <- struct{}{}
}
