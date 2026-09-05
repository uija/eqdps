package view

import (
	"fmt"
	"image/color"
	"log"
	"reflect"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/colorpicker"
	"github.com/ncruces/zenity"
	"github.com/uija/eqdps/internal/module"
	"github.com/uija/eqdps/internal/native"
	"github.com/uija/eqdps/internal/style"
	"github.com/uija/eqdps/internal/ui"
)

type Preferences struct {
	ctx  *module.Context
	list widget.List

	config_changed atomic.Bool

	overlay_opacity           widget.Float
	overlay_font_scale        widget.Float
	mainwindow_font_scale     widget.Float
	combat_timeout            widget.Float
	check_for_updates         widget.Bool
	open_eqlconnection_window widget.Clickable
	upload_sky_items          widget.Bool
	allow_eqldb_contribution  widget.Bool
	sky_parse_inventory       widget.Bool

	color_buttons       []widget.Clickable
	color_reset_buttons []widget.Clickable
	picker              colorpicker.State
	picker_ok           widget.Clickable
	picker_cancel       widget.Clickable

	select_font_click widget.Clickable
	reset_font_click  widget.Clickable

	color_pick_idx int

	stop chan struct{}
}

func NewPreferences(ctx *module.Context) *Preferences {
	p := &Preferences{
		ctx:            ctx,
		stop:           make(chan struct{}),
		color_pick_idx: -1,
	}
	p.list.Axis = layout.Vertical
	p.overlay_font_scale.Value = (ctx.Config.UIConfig.OverlayFontScale - 0.8) / 0.4
	p.config_changed.Store(false)
	p.upload_sky_items.Value = ctx.Config.EQLDbConfig.UploadSkyData
	p.allow_eqldb_contribution.Value = ctx.Config.EQLDbConfig.ContributeKillAndLootData
	p.overlay_opacity.Value = (ctx.Config.UIConfig.OverlayOpacity - 0.5) * 2
	p.check_for_updates.Value = ctx.Config.CheckForUpdates
	p.sky_parse_inventory.Value = ctx.Config.SkyConfig.ParseInventoryData
	p.mainwindow_font_scale.Value = (ctx.Config.UIConfig.MainWindowFontScale - 0.5) * 0.9
	p.combat_timeout.Value = float32(ctx.Config.CombatTimeout-20) / 60.0

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.config_changed.Load() {
					p.ctx.Config.Save()
					p.config_changed.Store(false)
				}
			case <-p.stop:
				return
			}
		}
	}()

	return p
}
func (p *Preferences) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.RenderHeader(style, gtx) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(style.Theme, &p.list)
					return list.Layout(
						gtx,
						6,
						func(gtx layout.Context, index int) layout.Dimensions {
							switch index {
							case 0:
								return layout.Flex{}.Layout(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return p.RenderWindowSettings(style, gtx)
								}))
							case 1:
								return p.RenderCombatSettings(style, gtx)
							case 2:
								return p.RenderUpdatesSettings(style, gtx)
							case 3:
								return p.RenderEQLDbSettings(style, gtx)
							case 4:
								return p.RenderFontSettings(style, gtx)
							case 5:
								return p.RenderColorSettings(style, gtx)
							}
							return layout.Dimensions{}
						},
					)
				}),
			)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if p.color_pick_idx >= 0 {
				return ui.Overlay(gtx, 420, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
					picker := colorpicker.Picker(style.Theme, &p.picker, "Color")
					return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {

						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, picker.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, ui.IconLink(style, &p.picker_ok, ui.Check, "OK").Layout),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return ui.IconLink(style, &p.picker_cancel, ui.Close, "Cancel").Layout(gtx)
										})
									}),
								)
							}),
						)
					})
				})
			}
			return layout.Dimensions{}
		}),
	)
}
func (p *Preferences) Update(gtx layout.Context) {
	if p.overlay_font_scale.Dragging() {
		p.ctx.Config.UIConfig.OverlayFontScale = 0.8 + (p.overlay_font_scale.Value * 0.4)
		p.config_changed.Store(true)
	}
	if p.overlay_opacity.Dragging() {
		p.ctx.Config.UIConfig.OverlayOpacity = 0.5 + (p.overlay_opacity.Value * 0.5)
		p.config_changed.Store(true)
		if p.ctx.Overlay != nil {
			p.ctx.Overlay.Send(p.ctx.Config.UIConfig.OverlayOpacity)
		}
	}
	if p.mainwindow_font_scale.Dragging() {
		val := 0.5 + (p.mainwindow_font_scale.Value * 0.9)
		p.ctx.Config.UIConfig.MainWindowFontScale = min(1.4, max(val, 0.5))
		ui.FontScaling = p.ctx.Config.UIConfig.MainWindowFontScale
		p.config_changed.Store(true)
	}
	if p.combat_timeout.Dragging() {
		val := 20 + int(p.combat_timeout.Value*60.0)
		p.ctx.Config.CombatTimeout = val
		p.ctx.Config.Save()
	}
	if p.allow_eqldb_contribution.Value != p.ctx.Config.EQLDbConfig.ContributeKillAndLootData {
		p.ctx.Config.EQLDbConfig.ContributeKillAndLootData = p.allow_eqldb_contribution.Value
		p.ctx.Config.Save()
	}
	if p.upload_sky_items.Value != p.ctx.Config.EQLDbConfig.UploadSkyData {
		p.ctx.Config.EQLDbConfig.UploadSkyData = p.upload_sky_items.Value
		p.ctx.Config.Save()
	}
	if p.check_for_updates.Value != p.ctx.Config.CheckForUpdates {
		p.ctx.Config.CheckForUpdates = p.check_for_updates.Value
		p.ctx.Config.Save()
	}
	if p.sky_parse_inventory.Value != p.ctx.Config.SkyConfig.ParseInventoryData {
		p.ctx.Config.SkyConfig.ParseInventoryData = p.sky_parse_inventory.Value
		p.ctx.Config.Save()
	}
	if p.picker_cancel.Clicked(gtx) {
		p.color_pick_idx = -1
	}
	if p.picker_ok.Clicked(gtx) {
		paletteValue := reflect.ValueOf(&style.Style.Palette).Elem()
		paletteType := paletteValue.Type()
		if paletteType.NumField() > p.color_pick_idx {
			dest := paletteValue.Field(p.color_pick_idx)
			dest.Set(reflect.ValueOf(p.picker.Color()))
			style.Style.Theme.Palette.Bg = style.Style.Palette.Window
			style.Style.Theme.Palette.Fg = style.Style.Palette.Text
			p.ctx.Config.UIConfig.Palette = &style.Style.Palette
			p.ctx.Config.Save()
		}
		p.color_pick_idx = -1
	}
	if p.color_buttons != nil {
		for i := range p.color_buttons {
			if p.color_buttons[i].Clicked(gtx) {
				paletteValue := reflect.ValueOf(&style.Style.Palette).Elem()
				paletteType := paletteValue.Type()
				if paletteType.NumField() > i {
					value := paletteValue.Field(i).Interface().(color.NRGBA)
					p.color_pick_idx = i
					p.picker.SetColor(value)
				}
			}
			if p.color_reset_buttons[i].Clicked(gtx) {
				paletteValue := reflect.ValueOf(&style.Style.Palette).Elem()
				defaultValue := reflect.ValueOf(&style.Style.DefaultPalette).Elem()
				paletteType := paletteValue.Type()
				if paletteType.NumField() > i {
					source := defaultValue.Field(i)
					dest := paletteValue.Field(i)
					dest.Set(source)
					style.Style.Theme.Palette.Bg = style.Style.Palette.Window
					style.Style.Theme.Palette.Fg = style.Style.Palette.Text
					p.ctx.Config.UIConfig.Palette = &style.Style.Palette
					p.ctx.Config.Save()
				}
			}
		}
	}
	if p.select_font_click.Clicked(gtx) {
		go func() {
			path, err := zenity.SelectFile(
				zenity.Title("Open Font"),
				zenity.FileFilters{
					{
						Name:     "Truetype Font",
						Patterns: []string{"*.ttf", "*.otf"},
					},
				},
			)
			if err != nil {
				return
			}
			if err := style.Style.LoadFont(path); err != nil {
				log.Printf("Unable to load font. %v", err)
				return
			}
			p.ctx.Config.UIConfig.FontPath = path
		}()
	}
	if p.reset_font_click.Clicked(gtx) {
		p.ctx.Config.UIConfig.FontPath = ""
		style.Style.ResetFont()
	}
}
func (p *Preferences) RenderFontSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{}.Layout(gtx, ui.HeaderLabel(style, "Font").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ui.ColorLabel(style.Palette.Muted, ui.Label(style, "Selecting custom fonts can break the layout of the app, as fonts have completely different dimensions.")).Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(16)}.Layout(gtx, ui.Label(style, "User font:").Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							children := make([]layout.FlexChild, 0)
							if p.ctx.Config.UIConfig.FontPath == "" {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.ColorLabel(style.Palette.Muted, ui.Label(style, "No font selected.")).Layout(gtx)
								}))
							} else {
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.ColorLabel(style.Palette.Muted, ui.Label(style, p.ctx.Config.UIConfig.FontPath)).Layout(gtx)
								}))
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &p.reset_font_click, ui.Close, "Reset").Layout)
								}))
							}
							children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &p.select_font_click, ui.Open, "Select").Layout)
							}))
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
						}),
					)
				})
			}),
		)
	})
}
func (p *Preferences) RenderColorSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	paletteValue := reflect.ValueOf(&style.Palette).Elem()
	paletteType := paletteValue.Type()
	if p.color_buttons == nil {
		p.color_buttons = make([]widget.Clickable, paletteType.NumField())
		p.color_reset_buttons = make([]widget.Clickable, paletteType.NumField())
	}
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{}.Layout(gtx, ui.HeaderLabel(style, "Colors").Layout)
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.ColorLabel(style.Palette.Muted, ui.Label(style, "You can change all the colors used in this app. Press 'Reset' to set them back to the default color.")).Layout(gtx)
		})
	}))
	for i := 0; i < paletteType.NumField(); i++ {
		fieldType := paletteType.Field(i)
		fieldValue := paletteValue.Field(i)

		label := fieldType.Tag.Get("label")
		colorValue := fieldValue.Interface().(color.NRGBA)

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = min(200, gtx.Constraints.Max.X)
					return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(8)}.Layout(gtx, ui.Label(style, label).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(style.Theme, &p.color_buttons[i], "          ")
					button.Background = colorValue
					button.TextSize = ui.Sp(15)
					return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.ColoredBorderedRow(gtx, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(1)).Layout(gtx, button.Layout)
						})
					})
				}),
				layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Link(style, &p.color_reset_buttons[i], "Reset").Layout(gtx)
					})
				}),
			)
		}))
	}
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
func (p *Preferences) RenderHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(style.Theme, ui.Sp(16), "Preferences").Layout)
}
func (p *Preferences) RenderWindowSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0)
	if native.SupportWindowOppacity() {
		children = append(children, p.RenderOverlayOpacity(style, gtx))
	}
	children = append(children, p.RenderOverlayFontScale(style, gtx))
	children = append(children, p.RenderMainWindowFontScale(style, gtx))
	return layout.Inset{Left: unit.Dp(32)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
func (p *Preferences) RenderOverlayOpacity(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(ui.Label(style, "DPS Overlay opacity").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(600, gtx.Constraints.Max.X)
				return material.Slider(style.Theme, &p.overlay_opacity).Layout(gtx)
			}),
		)
	})
}
func (p *Preferences) RenderMainWindowFontScale(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(ui.Label(style, "Main font scale").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(600, gtx.Constraints.Max.X)
				return material.Slider(style.Theme, &p.mainwindow_font_scale).Layout(gtx)
			}),
		)
	})
}
func (p *Preferences) RenderOverlayFontScale(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(ui.Label(style, "DPS Overlay font scale").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(600, gtx.Constraints.Max.X)
				return material.Slider(style.Theme, &p.overlay_font_scale).Layout(gtx)
			}),
		)
	})
}
func (p *Preferences) RenderCombatSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{}.Layout(gtx, ui.HeaderLabel(style, "Combat").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(ui.Label(style, "Combat Timeout").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = min(600, gtx.Constraints.Max.X)
								return material.Slider(style.Theme, &p.combat_timeout).Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(8)}.Layout(gtx, ui.Label(style, fmt.Sprintf("%d Seconds", p.ctx.Config.CombatTimeout)).Layout)
							}),
						)
					}),
				)
			}),
		)
	})
}
func (p *Preferences) RenderUpdatesSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			p.RenderCheckForUpdates(style, gtx),
			p.RenderSkyParseInventory(style, gtx),
		)
	})
}
func (p *Preferences) RenderCheckForUpdates(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.CheckBox(style.Theme, &p.check_for_updates, "Check for updates").Layout),
			layout.Rigid(ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(14), "Checks GitHub for newly published releases when the application starts")).Layout),
		)
	})
}
func (p *Preferences) RenderSkyParseInventory(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.CheckBox(style.Theme, &p.sky_parse_inventory, "Parse Inventory for Plane of Sky Items").Layout),
			layout.Rigid(ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(14), "Parses inventory exports you do and updates your plane of sky items with items found in your inventory. Does not work for Wind Runes.")).Layout),
		)
	})
}

func (p *Preferences) RenderEQLDbSettings(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return ui.HeaderLabel(style, "eqldb.org").Layout(gtx)
				})
			}),
			p.RenderEQLDbConnect(style, gtx),
			p.RenderEQLDbUploadSky(style, gtx),
		)
	})
}
func (p *Preferences) RenderEQLDbConnect(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if p.ctx.Config.EQLDbConfig.AccessToken == "" {
						return ui.IconLink(style, &p.open_eqlconnection_window, ui.ActionVisibility, "Connect to eqldb.org").Layout(gtx)
					} else {
						return ui.IconLabel(gtx, style.Theme, 15, ui.Check, "eqldb.org is already connected")
					}
				}),
				layout.Rigid(ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(14), "TODO: Render text that explain what EQLDb is etc.")).Layout),
			)
		})
	})
}
func (p *Preferences) RenderEQLDbUploadSky(style *ui.Style, gtx layout.Context) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.CheckBox(style.Theme, &p.upload_sky_items, "Upload Wind Runes from PoS to your EQLDb Profile").Layout),
				layout.Rigid(ui.ColorLabel(style.Palette.Muted, material.Label(style.Theme, ui.Sp(14), "Enhances your profile on eqldb.org with wind runes, that are not part of the inventory export")).Layout),
			)
		})
	})
}

func (p *Preferences) Shutdown() {
	p.stop <- struct{}{}
}
