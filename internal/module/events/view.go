package events

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	stacks := make([]layout.StackChild, 0)
	stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.RenderMainPage(style, gtx) }))
	if m.edit_index >= 0 && m.edit_index < len(m.ctx.Config.Events) || m.create_type != data.EventTypeUndefined {
		stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions { return m.RenderOverlay(style, gtx) }))
	}
	if m.delete_id >= 0 && m.delete_id < len(m.ctx.Config.Events) {
		stacks = append(stacks, layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return m.RenderDeleteOverlay(style, gtx)
		}))
	}
	return layout.Stack{}.Layout(gtx,
		stacks...,
	)
}
func (m *Module) RenderDeleteOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(unit.Dp(400)), gtx.Constraints.Max.X)
		gtx.Constraints.Max.X = width
		gtx.Constraints.Min.X = width

		ui.FillOverlay(gtx, style.Palette.Accent, style.Palette.Border)

		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Label(style.Theme, 18, "Delete Event")
							label.Font.Weight = font.SemiBold
							label.Alignment = text.Middle
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							str := fmt.Sprintf("Do you really want to delete the event \"%s\"", m.ctx.Config.Events[m.delete_id].Title)
							return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Label(style.Theme, 14, str).Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.E.Layout(gtx, ui.IconLink(style, &m.do_delete_click, ui.Check, "Delete it!").Layout)
									})
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.W.Layout(gtx, ui.IconLink(style, &m.cancel_delete_click, ui.Close, "Cancel").Layout)
									})
								}),
							)
						}),
					)
				})
			}),
		)
	})
}
func (m *Module) RenderOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.event_form.LayoutModalInputLayer(gtx)
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0)
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							var edit *data.EventConfig = nil
							title := "Add Event"
							if m.edit_index >= 0 {
								edit = &m.ctx.Config.Events[m.edit_index]
							}
							if edit != nil {
								title = "Edit Event"
							}
							label := material.Label(style.Theme, 16, title)
							label.Font.Weight = font.SemiBold
							return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, label.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.IconLink(style, &m.overlay_close, ui.Close, "Cancel").Layout(gtx)
						}),
					)
				}),
			)
			children = append(children, m.GenerateFormFieldRows(style, gtx)...)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				children...,
			)
		})
	})
}
func (m *Module) GenerateFormFieldRows(style *ui.Style, gtx layout.Context) []layout.FlexChild {
	t := m.create_type
	if t == data.EventTypeUndefined && m.edit_index >= 0 {
		t = m.ctx.Config.Events[m.edit_index].Type
	}
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(m.TextFieldRow(m.title_field, "Title", "", style, gtx)))
	if t == data.EventTypeSpell || t == data.EventTypeTimer {
		children = append(children, layout.Rigid(m.SelectBoxRow(m.class_select, "Class", style, gtx)))
		children = append(children, layout.Rigid(m.SelectBoxRow(m.spell_select, "Spell", style, gtx)))
		if t == data.EventTypeSpell {
			children = append(children, layout.Rigid(m.SelectBoxRow(m.target_select, "Check for fade on", style, gtx)))
		} else {
			m.event_form.SetVisible("target", false)
		}
	} else {
		m.event_form.SetVisible("class", false)
		m.event_form.SetVisible("spell", false)
		m.event_form.SetVisible("target", false)
	}
	switch t {
	case data.EventTypeString:
		children = append(children, layout.Rigid(m.TextFieldRow(m.text_field, "Text", "", style, gtx)))
		children = append(children, layout.Rigid(m.CheckBoxRow(m.full_message_check, "", "Full message match", style, gtx)))
	case data.EventTypeRegexp:
		children = append(children, layout.Rigid(m.TextFieldRow(m.text_field, "RegExp", "", style, gtx)))
		m.event_form.SetVisible("full", false)
	default:
		m.event_form.SetVisible("text", false)
		m.event_form.SetVisible("full", false)
	}
	if t == data.EventTypeTimer {
		children = append(children, layout.Rigid(m.TextFieldRow(m.duration_field, "Duration", "", style, gtx)))
	} else {
		m.event_form.SetVisible("duration", false)
	}
	children = append(children, layout.Rigid(m.TextFieldRow(m.notification_field, "Notification", "", style, gtx)))
	children = append(children, layout.Rigid(m.CheckBoxRow(m.persistent_check, "", "Request persistent notification", style, gtx)))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(m.SelectBoxRow(m.sound_select, "Sound", style, gtx)),
			layout.Rigid(material.Button(style.Theme, &m.play_sound_click, "Play").Layout),
			layout.Flexed(1, material.Body1(style.Theme, "").Layout),
		)
	}))
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(style.Theme, &m.save_button_click, "Save").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, material.Button(style.Theme, &m.close_button_click, "Cancel").Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if m.edit_index >= 0 {
						return layout.E.Layout(gtx, material.Button(style.Theme, &m.delete_button_click, "Delete").Layout)
					} else {
						return material.Body1(style.Theme, "").Layout(gtx)
					}
				}),
			)
		}),
	)
	return children
}
func (m *Module) CheckBoxRow(field *widget.Bool, title string, hint string, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Body1(style.Theme, title).Layout)
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(0)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(style.Theme, field, hint).Layout(gtx)
				})
			}),
		)
	}
}
func (m *Module) SelectBoxRow(field *form.SelectBox, title string, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Body1(style.Theme, title).Layout)
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return field.Layout(style, gtx, unit.Dp(300))
				})
			}),
		)
	}
}
func (m *Module) TextFieldRow(field *widget.Editor, title string, hint string, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Body1(style.Theme, title).Layout)
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if strings.EqualFold(title, "regexp") {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.TextField(field, hint, style, gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								switch m.validation_state {
								case -1:
									return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return ui.ColoredIconLabel(gtx, style.Theme, 15, ui.Check, style.Palette.No, "Error")
									})
								case 1:
									return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return ui.ColoredIconLabel(gtx, style.Theme, 15, ui.Check, style.Palette.Yes, "Validated!")
									})
								default:
									return layout.UniformInset(unit.Dp(10)).Layout(gtx, ui.IconLink(style, &m.validate_regexp_click, ui.Check, "Validate").Layout)
								}
							}),
						)
					} else {
						return ui.TextField(field, hint, style, gtx)
					}
				})
			}),
		)
	}
}
func (m *Module) RenderMainPage(style *ui.Style, gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderPageHeader(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderVolumeRow(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderSpellIconRow(style, gtx) }))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderEventsTable(style, gtx) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
func (m *Module) RenderVolumeRow(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(400, gtx.Constraints.Max.X)
				return layout.Inset{Top: unit.Dp(6), Right: unit.Dp(16)}.Layout(gtx, material.Body1(style.Theme, "Notification Volume:").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(300, gtx.Constraints.Max.X)
				return material.Slider(style.Theme, &m.volume).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, material.Body1(style.Theme, fmt.Sprintf("%.02f%%", m.ctx.Config.Volume)).Layout)
			}),
		)
	})
}
func (m *Module) RenderSpellIconRow(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = min(400, gtx.Constraints.Max.X)
				return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, unit.Sp(15), "Spell Icon Set:").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.spell_icon_select.Layout(style, gtx, unit.Dp(gtx.Dp(200)))
			}),
			layout.Flexed(1, material.Body1(style.Theme, "").Layout),
		)
	})
}
func (m *Module) RenderEventsTable(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.RenderEventsTableHeader(style, gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return m.RenderEventsTableList(style, gtx)
			}),
		)
	})
}
func (m *Module) RenderEventsTableList(style *ui.Style, gtx layout.Context) layout.Dimensions {
	size := len(m.ctx.Config.Events)
	if size != len(m.row_click) {
		m.row_click = make([]widget.Clickable, size)
		m.activate_click = make([]widget.Clickable, size)
	}
	list := material.List(style.Theme, &m.events_list)
	return list.Layout(
		gtx,
		size,
		func(gtx layout.Context, index int) layout.Dimensions {
			return m.RenderEventsTableRow(index, style, gtx)
		},
	)
}
func (m *Module) RenderEventsTableRow(index int, style *ui.Style, gtx layout.Context) layout.Dimensions {
	event := m.ctx.Config.Events[index]
	icon := ui.Close
	icon_color := style.Palette.No
	if event.Active {
		icon = ui.Check
		icon_color = style.Palette.Yes
	}
	not_icon := ui.Close
	if event.Notification != "" {
		not_icon = ui.Check
	}
	sound := "No sound"
	if event.Sound != "" {
		sound = event.Sound
	}
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						link := ui.IconLink(style, &m.activate_click[index], icon, "")
						link.TextColor = icon_color
						return link.Layout(gtx)
					})
				})
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				link := ui.Link(style, &m.row_click[index], event.Title)
				return layout.Inset{Left: unit.Dp(0)}.Layout(gtx, link.Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				str := "Unknown"
				switch event.Type {
				case data.EventTypeRegexp:
					str = "RegExp"
				case data.EventTypeSpell:
					str = "Spell"
				case data.EventTypeString:
					str = "Text"
				case data.EventTypeTimer:
					str = "Timer"
				}
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, unit.Sp(14), str).Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Icon(gtx, style.Palette.Text, not_icon)
					})
				})
			}),
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, unit.Sp(14), sound).Layout)
			}),
		)
	})
}
func (m *Module) RenderEventsTableHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16)}.Layout(gtx,
						material.Label(style.Theme, unit.Sp(14), "ACTIVE").Layout,
					)
				},
			),
			layout.Flexed(4,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, unit.Sp(14), "TITLE").Layout,
					)
				},
			),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, unit.Sp(14), "TYPE").Layout,
					)
				},
			),
			layout.Flexed(1,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, unit.Sp(14), "NOTIFY").Layout,
					)
				},
			),
			layout.Flexed(2,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx,
						material.Label(style.Theme, unit.Sp(14), "SOUND").Layout,
					)
				},
			),
		)
	})

}
func (m *Module) RenderPageHeader(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return ui.ColoredRow(gtx, style.Palette.Window, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(style.Theme, 18, "Events")
			label.Font.Weight = font.SemiBold
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, label.Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_spell_click, ui.Book, "Add Spell"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_timer_click, ui.Timer, "Add Timer"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_text_click, ui.Text, "Add Text"))
						}),
					)
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_regexp_click, ui.RegExp, "Add RegExp"))
						}),
					)

					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				}),
			)
		})
	})
}
func RenderLinkAsButton(style *ui.Style, clickable *widget.Clickable, icon *widget.Icon, text string) layout.Widget {
	link := ui.IconLink(style, clickable, icon, text)
	link.Padding[ui.PADDING_TOP] = 4
	link.Padding[ui.PADDING_BOTTOM] = 4
	link.Padding[ui.PADDING_LEFT] = 8
	link.Padding[ui.PADDING_RIGHT] = 8
	return func(gtx layout.Context) layout.Dimensions {
		return ui.ColoredBorderedRow(gtx, style.Palette.Panel, link.Layout)
	}
}
