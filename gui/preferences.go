package main

import (
	"fmt"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func settingToSlider(value, minimum, maximum float32) float32 {
	return (value - minimum) / (maximum - minimum)
}

func sliderToSetting(value, minimum, maximum float32) float32 {
	return minimum + value*(maximum-minimum)
}

func (s *shell) layoutPreferences(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labelWeight(gtx, s.theme, "Preferences", unit.Sp(27), palette.text, text.Start, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.layoutPreferenceSlider(gtx, "Main window font scale", &s.mainScale, .75, 1.5, true, true)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.layoutPreferenceSlider(gtx, "DPS overlay font scale", &s.dpsScale, .5, 1.5, true, true)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.layoutPreferenceSlider(gtx, "DPS overlay opacity", &s.dpsOpacity, .35, 1, nativeOpacityAvailable(), true)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.layoutPreferenceSlider(gtx, "Combat and overlay idle timeout", &s.idleTimeoutSlider, 5, 60, true, false)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			message := "Opacity is stored, but this platform requires compositor configuration. See Help → Wayland overlay setup."
			if nativeOpacityAvailable() {
				message = "Opacity is applied immediately to the DPS overlay."
			}
			return inset(0, unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, message, unit.Sp(14), palette.muted, text.Start)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				before := s.dropCollection.Value
				dimensions := material.CheckBox(s.theme, &s.dropCollection, "Contribute kill and loot observations to EQLDB").Layout(gtx)
				if before != s.dropCollection.Value {
					s.setDropCollectionEnabled(s.dropCollection.Value)
				}
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, "Collected observations upload automatically while EQLDB is connected.", unit.Sp(14), palette.muted, text.Start)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(22)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				before := s.skyCompareEnabled.Value
				dimensions := material.CheckBox(s.theme, &s.skyCompareEnabled, "Compare inventory exports with Plane of Sky progress").Layout(gtx)
				if before != s.skyCompareEnabled.Value {
					s.settings.CompareSkyInventory = s.skyCompareEnabled.Value
					if err := saveSettings(s.settings); err != nil {
						s.statusText = "Inventory comparison preference could not be saved"
					}
				}
				return dimensions
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return label(gtx, s.theme, "Wind Runes are excluded from this comparison.", unit.Sp(14), palette.muted, text.Start)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inset(0, unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return label(gtx, s.theme, "Plane of Sky tracker", unit.Sp(17), palette.text, text.Start)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return label(gtx, s.theme, "Delete the saved progress and rebuild it from the current logfile.", unit.Sp(14), palette.muted, text.Start)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.currentLog == "" || s.skyLoading {
							gtx = gtx.Disabled()
						}
						return betaCleanupButton(gtx, s.theme, &s.skyResetClick, "Reset and rescan", false)
					}),
				)
			})
		}),
	)
}

func (s *shell) setDropCollectionEnabled(enabled bool) {
	s.dropCollectorMu.Lock()
	defer s.dropCollectorMu.Unlock()
	if s.dropCollector == nil {
		s.dropCollection.Value = false
		s.statusText = "Open a logfile before enabling drop collection"
		return
	}
	if err := s.dropCollector.SetEnabled(enabled); err != nil {
		s.dropCollection.Value = !enabled
		s.statusText = "Drop collection preference could not be saved"
	}
}

func (s *shell) layoutPreferenceSlider(gtx layout.Context, title string, state *widget.Float, minimum, maximum float32, enabled, percentage bool) layout.Dimensions {
	value := sliderToSetting(state.Value, minimum, maximum)
	displayValue := fmt.Sprintf("%d seconds", int(value+.5))
	if percentage {
		displayValue = fmt.Sprintf("%d%%", int(value*100+.5))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return label(gtx, s.theme, title, unit.Sp(17), palette.text, text.Start)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return labelWeight(gtx, s.theme, displayValue, unit.Sp(16), palette.accent, text.End, font.SemiBold)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(520)))
			if !enabled {
				gtx = gtx.Disabled()
			}
			slider := material.Slider(s.theme, state)
			slider.Color = palette.accent
			dimensions := slider.Layout(gtx)
			s.applyPreferenceValues()
			return dimensions
		}),
	)
}

func (s *shell) applyPreferenceValues() {
	mainScale := sliderToSetting(s.mainScale.Value, .75, 1.5)
	dpsScale := sliderToSetting(s.dpsScale.Value, .5, 1.5)
	opacity := sliderToSetting(s.dpsOpacity.Value, .35, 1)
	idleTimeout := int(sliderToSetting(s.idleTimeoutSlider.Value, 5, 60) + .5)
	opacityChanged := opacity != s.settings.DPSOpacity
	if mainScale != s.settings.MainFontScale || dpsScale != s.settings.DPSFontScale || opacity != s.settings.DPSOpacity || idleTimeout != s.settings.IdleTimeoutSec {
		s.settings.MainFontScale = mainScale
		s.settings.DPSFontScale = dpsScale
		s.dpsFontMilli.Store(int64(effectiveFontScale(dpsScale)*1000 + .5))
		s.settings.DPSOpacity = opacity
		s.settings.IdleTimeoutSec = idleTimeout
		s.combatIdleNanos.Store(int64(time.Duration(idleTimeout) * time.Second))
		s.theme.TextSize = unit.Sp(16 * effectiveFontScale(mainScale))
		s.pushOverlay(s.fights)
		if opacityChanged {
			s.applyOverlayOpacity(opacity)
		}
		s.prefsDirty = true
	}
	if s.prefsDirty && !s.mainScale.Dragging() && !s.dpsScale.Dragging() && !s.dpsOpacity.Dragging() {
		s.prefsDirty = false
		if err := saveSettings(s.settings); err != nil {
			s.statusText = "Preferences could not be saved"
		}
	}
}
