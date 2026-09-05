package events

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/data"
	"github.com/uija/eqdps/internal/ui"
	"github.com/uija/eqdps/internal/ui/form"
)

func (m *Module) RenderOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	m.event_form.LayoutModalInputLayer(gtx)
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ui.FillOverlay(gtx, style.Palette.Panel, style.Palette.Border)
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			m.mu.Lock()
			editing := m.edit_index >= 0 && m.edit_index < len(m.ctx.Config.Events)
			m.mu.Unlock()

			children := make([]layout.FlexChild, 0)
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := "Add Event"
							if editing {
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
			helpText := m.GetHelpText()
			if helpText != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := ui.ColorLabel(style.Palette.Muted, ui.Label(style, helpText))
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, label.Layout)
				}))
			}
			children = append(children, m.GenerateFormFieldRows(style, gtx)...)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				children...,
			)
		})
	})
}
func (m *Module) GetHelpText() string {
	var t data.EventType = m.create_type
	if m.edit_index >= 0 {
		t = m.ctx.Config.Events[m.edit_index].Type
	}
	switch t {
	case data.EventTypeString:
		return "Text events trigger when a logfile message contains the text you entered.\nEnable full message matching when the complete message must match instead of only a part of it.\nThe timestamp at the beginning of the logfile line is not included in the message."
	case data.EventTypeRegexp:
		return "Regular-expression events let you match more flexible logfile messages using a Go regular expression.\nThe expression is checked against the message without the timestamp at the beginning of the logfile line.\nUse Validate to check the expression before saving the event."
	case data.EventTypeTimer:
		return "Timers detect you casting the selected spell and start a timer using the duration you set in seconds.\nThe timer is shown in the overlay. After the timer runs out, a notification is triggered.\nYou have to define the duration yourself, because the duration depends on the level of the spell, some AAs and the focus effects you are using."
	case data.EventTypeSpell:
		return "Spell events are predefined text events that detect when a spell fades.\nThe available spells and their fade messages are extracted from the EverQuest client data.\nIf a spell is not listed here, it is either missing from the client data or does not have a fade message defined."
	default:
		return ""
	}
}
func (m *Module) GenerateFormFieldRows(style *ui.Style, gtx layout.Context) []layout.FlexChild {
	t := m.create_type
	if t == data.EventTypeUndefined && m.edit_index >= 0 {
		m.mu.Lock()
		if m.edit_index < len(m.ctx.Config.Events) {
			t = m.ctx.Config.Events[m.edit_index].Type
		}
		m.mu.Unlock()
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
	case data.EventTypeSpell:
		children = append(children, layout.Rigid(m.EnhancedTextFieldRow(m.text_field, "Fade message", "", true, style, gtx)))
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
			layout.Flexed(1, ui.Label(style, "").Layout),
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
						return ui.Label(style, "").Layout(gtx)
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
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, ui.Label(style, title).Layout)
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
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, ui.Label(style, title).Layout)
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
	return m.EnhancedTextFieldRow(field, title, hint, false, style, gtx)
}

func (m *Module) EnhancedTextFieldRow(field *widget.Editor, title string, hint string, readonly bool, style *ui.Style, gtx layout.Context) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, ui.Label(style, title).Layout)
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
						if readonly {
							val := field.Text()
							if val == "" {
								val = hint
							}
							return layout.UniformInset(unit.Dp(4)).Layout(gtx, ui.Label(style, val).Layout)
						}
						return ui.TextField(field, hint, style, gtx)
					}
				})
			}),
		)
	}
}
