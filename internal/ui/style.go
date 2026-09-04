package ui

import (
	"os"

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
func (s Style) ResetFont() {
	s.Theme.Shaper = s.OriginalShaper
	s.Theme.Face = s.OriginalFace
}
