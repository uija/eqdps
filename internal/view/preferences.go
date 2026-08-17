package view

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/ui"
)

type Preferences struct {
	ctx  *module.Context
	list widget.List

	overlay_opacity           widget.Float
	check_for_updates         widget.Bool
	open_eqlconnection_window widget.Clickable
}

func NewPreferences(ctx *module.Context) *Preferences {
	p := &Preferences{
		ctx: ctx,
	}
	p.list.Axis = layout.Vertical
	return p
}
func (p *Preferences) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.RenderHeader(style, gtx) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			list := material.List(style.Theme, &p.list)
			return list.Layout(
				gtx,
				3,
				func(gtx layout.Context, index int) layout.Dimensions {
					switch index {
					case 0:
						return p.RenderWindowSettings(style, gtx)
					case 1:
						return p.RenderUpdatesSettings(style, gtx)
					case 2:
						return p.RenderEQLDbSettings(style, gtx)
					}
					return layout.Dimensions{}
				},
			)
		}),
	)
}
func (p *Preferences) RenderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(style.Theme, unit.Sp(16), "Preferences").Layout)
}
func (p *Preferences) RenderWindowSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			p.RenderOverlayOpacity(style, gtx),
		)
	})
}
func (p *Preferences) RenderOverlayOpacity(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Label(style.Theme, unit.Sp(15), "DPS Overlay opacity").Layout),
			layout.Rigid(material.Slider(style.Theme, &p.overlay_opacity).Layout),
		)
	})
}
func (p *Preferences) RenderUpdatesSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			p.RenderCheckForUpdates(style, gtx),
		)
	})
}
func (p *Preferences) RenderCheckForUpdates(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.CheckBox(style.Theme, &p.check_for_updates, "Check for updates").Layout),
			layout.Rigid(material.Label(style.Theme, unit.Sp(14), "Checks GitHub for newly published releases when the application starts").Layout),
		)
	})
}

func (p *Preferences) RenderEQLDbSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			p.RenderEQLDbConnect(style, gtx),
		)
	})
}
func (p *Preferences) RenderEQLDbConnect(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Label(style.Theme, unit.Sp(14), "TODO: Render text that explain what EQLDb is etc.").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.ctx.Config.EQLDbConfig.AccessToken == "" {
					return ui.IconLink(style, &p.open_eqlconnection_window, ui.ActionVisibility, "Connect to eqldb.org").Layout(gtx)
				} else {
					return material.Label(style.Theme, unit.Sp(14), "eqldb.org is already connected.").Layout(gtx)
				}
			}),
		)
	})
}
