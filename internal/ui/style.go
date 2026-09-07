package ui

import (
	"image/color"
	"os"
	"reflect"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"
)

type Style struct {
	Theme          *material.Theme
	Palette        Palette
	DefaultPalette Palette

	OriginalShaper *text.Shaper
	OriginalFace   font.Typeface
}

func (s Style) LoadFont(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	face, err := opentype.Parse(bytes)
	if err != nil {
		return err
	}
	s.Theme.Shaper = text.NewShaper(
		text.WithCollection([]font.FontFace{{
			Font: font.Font{Typeface: "UserFont"},
			Face: face,
		}}),
	)
	s.Theme.Face = font.Typeface("UserFont")
	return nil
}
func (s *Style) ValidatePalette() bool {
	paletteValue := reflect.ValueOf(&s.Palette).Elem()
	defaultValue := reflect.ValueOf(&s.DefaultPalette).Elem()
	paletteType := paletteValue.Type()
	changed := false
	for i := 0; i < paletteType.NumField(); i++ {
		dest := paletteValue.Field(i)
		colorValue := dest.Interface().(color.NRGBA)
		if colorValue.R == 0 && colorValue.G == 0 && colorValue.B == 0 && colorValue.A == 0 {
			source := defaultValue.Field(i)
			dest.Set(source)
			changed = true
		}
	}
	if changed {
		s.Theme.Palette.Bg = s.Palette.Window
		s.Theme.Palette.Fg = s.Palette.Text
	}
	return !changed // Return if the palette was valid
}
func (s Style) ResetFont() {
	s.Theme.Shaper = s.OriginalShaper
	s.Theme.Face = s.OriginalFace
}
