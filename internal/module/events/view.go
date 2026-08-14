package events

import (
	"gioui.org/font"
	"gioui.org/layout"
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
	return layout.Stack{}.Layout(gtx,
		stacks...,
	)
}
func (m *Module) RenderOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.event_form.LayoutInputLayer(gtx)
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
	children = append(children, layout.Rigid(m.TextFieldRow(m.title_field, "Title", "the title", style, gtx)))
	if t == data.EventTypeSpell || t == data.EventTypeTimer {
		children = append(children, layout.Rigid(m.SelectBoxRow(m.class_select, "Class", style, gtx)))
		children = append(children, layout.Rigid(m.SelectBoxRow(m.spell_select, "Spell", style, gtx)))
		children = append(children, layout.Rigid(m.SelectBoxRow(m.target_select, "Check for fade on", style, gtx)))

	} else {
		m.event_form.SetVisible("class", false)
		m.event_form.SetVisible("spell", false)
		m.event_form.SetVisible("target", false)
	}
	switch t {
	case data.EventTypeString:
		children = append(children, layout.Rigid(m.TextFieldRow(m.text_field, "Text", "the text", style, gtx)))
		children = append(children, layout.Rigid(m.CheckBoxRow(m.full_message_check, "", "Full message match", style, gtx)))
	case data.EventTypeRegexp:
		children = append(children, layout.Rigid(m.TextFieldRow(m.text_field, "RegExp", "the text", style, gtx)))
		m.event_form.SetVisible("full", false)
	default:
		m.event_form.SetVisible("text", false)
		m.event_form.SetVisible("full", false)
	}
	if t == data.EventTypeTimer {
		children = append(children, layout.Rigid(m.TextFieldRow(m.duration_field, "Duration", "the duration", style, gtx)))
	} else {
		m.event_form.SetVisible("duration", false)
	}
	children = append(children, layout.Rigid(m.TextFieldRow(m.notification_field, "Notification", "the notification", style, gtx)))
	children = append(children, layout.Rigid(m.CheckBoxRow(m.persistent_check, "", "Request persistent notification", style, gtx)))
	children = append(children, layout.Rigid(m.SelectBoxRow(m.sound_select, "Sound", style, gtx)))
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(style.Theme, &m.save_button_click, "Save").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, material.Button(style.Theme, &m.close_button_click, "Cancel").Layout)
				}),
				layout.Flexed(1, material.Body1(style.Theme, "").Layout),
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
					return ui.TextField(field, hint, style, gtx)
				})
			}),
		)
	}
}
func (m *Module) RenderMainPage(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderPageHeader(style, gtx) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.RenderEventsTable(style, gtx) }),
	)
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
						return ui.Icon(gtx, icon_color, icon)
					})
				})
			}),
			layout.Flexed(4, func(gtx layout.Context) layout.Dimensions {
				link := ui.Link(style, &m.row_click[index], event.Title)
				return layout.Inset{Left: unit.Dp(0)}.Layout(gtx, link.Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, material.Label(style.Theme, unit.Sp(14), "Type").Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.Icon(gtx, style.Palette.Text, not_icon)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
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
			layout.Flexed(1,
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
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(RenderLinkAsButton(style, &m.add_spell_click, ui.Book, "Add Spell")),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_timer_click, ui.Timer, "Add Timer"))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_text_click, ui.Text, "Add Text"))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, RenderLinkAsButton(style, &m.add_regexp_click, ui.RegExp, "Add RegExp"))
						}),
					)
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
