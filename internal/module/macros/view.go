package macros

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/uija/eqdps/internal/ui"
)

func (m *Module) Layout(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return m.RenderList(style, gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if m.selected_index >= 0 && m.list_macros != nil && len(m.list_macros) > m.selected_index {
				return m.shadow_click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ui.Fill(gtx, style.Palette.Shadow)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				})
			}
			return layout.Dimensions{}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return m.RenderOverlay(style, gtx)
		}),
	)
}
func (m *Module) RenderList(style *ui.Style, gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if m.config_path == "" {
			return material.Label(style.Theme, ui.Sp(15), "You need to open a log.").Layout(gtx)
		} else if m.loading.Load() {
			return material.Label(style.Theme, ui.Sp(15), "Loading please wait.").Layout(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Label(style.Theme, ui.Sp(17), "Macros").Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), "Filter:").Layout)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return ui.MaxedTextField(&m.filter_editor, "Filter by name or macro content", style, gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: unit.Dp(7), Left: unit.Dp(16)}.Layout(gtx, ui.IconLink(style, &m.filter_clear_click, ui.Close, "Clear").Layout)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.ColoredRow(gtx, style.Palette.Panel, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(style.Theme, ui.Sp(15), "Name")
								label.Font.Weight = font.SemiBold
								return label.Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(style.Theme, ui.Sp(15), "Loadout")
								label.Font.Weight = font.SemiBold
								return label.Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(style.Theme, ui.Sp(15), "Macro")
								label.Font.Weight = font.SemiBold
								return label.Layout(gtx)
							}),
						)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				list := material.List(style.Theme, &m.list)

				return list.Layout(gtx, len(m.list_macros), func(gtx layout.Context, index int) layout.Dimensions {
					mac := m.list_macros[index]
					col := style.Palette.Panel
					if index%2 == 0 {
						col = style.Palette.Window
					}
					return ui.ColoredRow(gtx, col, func(gtx layout.Context) layout.Dimensions {
						return m.list_macros[index].Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							pointer.CursorPointer.Add(gtx.Ops)
							return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return material.Label(style.Theme, ui.Sp(15), mac.Name).Layout(gtx)
									}),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("LO%d", mac.Loadout)).Layout(gtx)
										})
									}),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return material.Label(style.Theme, ui.Sp(15), mac.Location).Layout(gtx)
										})
									}),
								)
							})
						})
					})
				})
			}),
		)
	})
}
func (m *Module) RenderOverlay(style *ui.Style, gtx layout.Context) layout.Dimensions {
	if m.selected_index >= 0 && m.list_macros != nil && len(m.list_macros) > m.selected_index {
		mac := m.list_macros[m.selected_index]
		return ui.Overlay(gtx, 420, style.Palette.Panel, style.Palette.Border, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				createRow := func(r int) layout.FlexChild {
					return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {

							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										label := material.Label(style.Theme, ui.Sp(15), fmt.Sprintf("Line %d:", r))
										label.Color = style.Palette.Muted
										return label.Layout(gtx)

									})
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(16)}.Layout(gtx, material.Label(style.Theme, ui.Sp(15), mac.Rows[r]).Layout)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(mac.Rows[r]) == "" {
										return material.Label(style.Theme, ui.Sp(15), "").Layout(gtx)
									}
									return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, ui.IconLink(style, &m.line_copy_click[r], ui.Copy, "").Layout)
								}),
							)
						})
					})
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									label := material.Label(style.Theme, ui.Sp(17), mac.Name)
									label.Color = style.Palette.Accent
									return label.Layout(gtx)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return ui.IconLink(style, &m.overlay_click, ui.Close, "Close").Layout(gtx)
									})
								})
							}),
						)
					}),
					createRow(0),
					createRow(1),
					createRow(2),
					createRow(3),
					createRow(4),
				)
			})
		})
	}
	return layout.Dimensions{}
}
