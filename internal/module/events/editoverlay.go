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
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Label(style.Theme, ui.Sp(15), title).Layout)
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
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Label(style.Theme, ui.Sp(15), title).Layout)
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
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, material.Label(style.Theme, ui.Sp(15), title).Layout)
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
