package spellicon

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/uija/eqdps/internal/catalog"
)

func TestDetectAndExtract(t *testing.T) {
	gameDir := t.TempDir()
	logDir := filepath.Join(gameDir, "Logs")
	sheetDir := filepath.Join(gameDir, "uifiles", "default")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sheetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(gameDir, "eqgame.exe"),
		filepath.Join(gameDir, "spells_us.txt"),
		filepath.Join(sheetDir, "EQUI_SpellIcons.xml"),
	} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	logFile := filepath.Join(logDir, "eqlog_test.txt")
	if err := os.WriteFile(logFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sheet := makeTestTGA(t, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	if err := os.WriteFile(filepath.Join(sheetDir, "Spells01.tga"), sheet, 0o600); err != nil {
		t.Fatal(err)
	}

	spells := []catalog.Spell{{Name: "Spirit of Wolf", IconID: 4}}
	source, ok := Detect(logFile, spells)
	if !ok {
		t.Fatal("Detect() did not recognize the test EverQuest installation")
	}
	targetDir := filepath.Join(t.TempDir(), "icons")
	if err := Extract(source, targetDir, spells); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(IconPath(targetDir, 4))
	if err != nil {
		t.Fatal(err)
	}
	icon, err := png.Decode(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	got := color.NRGBAModel.Convert(icon.At(20, 20)).(color.NRGBA)
	want := (color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	if got != want {
		t.Fatalf("icon pixel = %#v, want %#v", got, want)
	}
}

func TestExtractAllDeduplicatesEffectiveUISkins(t *testing.T) {
	gameDir := t.TempDir()
	logDir := filepath.Join(gameDir, "Logs")
	defaultDir := filepath.Join(gameDir, "uifiles", "default")
	modernDir := filepath.Join(gameDir, "uifiles", "default_modern")
	lightDir := filepath.Join(gameDir, "uifiles", "default_light")
	for _, directory := range []string{logDir, defaultDir, modernDir, lightDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(gameDir, "eqgame.exe"),
		filepath.Join(gameDir, "spells_us.txt"),
		filepath.Join(defaultDir, "EQUI_SpellIcons.xml"),
	} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	logFile := filepath.Join(logDir, "eqlog_test.txt")
	if err := os.WriteFile(logFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	classic := makeTestTGA(t, 4, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	modern := makeTestTGA(t, 4, color.NRGBA{R: 100, G: 110, B: 120, A: 255})
	if err := os.WriteFile(filepath.Join(defaultDir, "Spells01.tga"), classic, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lightDir, "spells01.tga"), classic, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modernDir, "spells01.tga"), modern, 0o600); err != nil {
		t.Fatal(err)
	}

	spells := []catalog.Spell{{Name: "Spirit of Wolf", IconID: 4}}
	source, ok := Detect(logFile, spells)
	if !ok {
		t.Fatal("Detect() did not recognize the test EverQuest installation")
	}
	targetDir := filepath.Join(t.TempDir(), "icons")
	names, err := ExtractAll(source, targetDir, spells)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"default", "default_modern"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("extracted icon sets = %#v, want %#v", names, want)
	}
	for _, test := range []struct {
		name string
		want color.NRGBA
	}{
		{name: "default", want: color.NRGBA{R: 10, G: 20, B: 30, A: 255}},
		{name: "default_modern", want: color.NRGBA{R: 100, G: 110, B: 120, A: 255}},
	} {
		file, err := os.Open(SetIconPath(targetDir, test.name, 4))
		if err != nil {
			t.Fatal(err)
		}
		icon, decodeErr := png.Decode(file)
		closeErr := file.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if got := color.NRGBAModel.Convert(icon.At(20, 20)).(color.NRGBA); got != test.want {
			t.Fatalf("%s icon pixel = %#v, want %#v", test.name, got, test.want)
		}
	}
}

func TestDecodeRLETGA(t *testing.T) {
	var data bytes.Buffer
	header := make([]byte, 18)
	header[2] = 10
	binary.LittleEndian.PutUint16(header[12:14], 2)
	binary.LittleEndian.PutUint16(header[14:16], 1)
	header[16] = 32
	header[17] = 0x28
	data.Write(header)
	data.WriteByte(0x81)
	data.Write([]byte{30, 20, 10, 255})

	decoded, err := decodeTGA(&data)
	if err != nil {
		t.Fatal(err)
	}
	want := (color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	for x := range 2 {
		got := color.NRGBAModel.Convert(decoded.At(x, 0)).(color.NRGBA)
		if got != want {
			t.Fatalf("pixel %d = %#v, want %#v", x, got, want)
		}
	}
}

func makeTestTGA(t *testing.T, iconID int, iconColor color.NRGBA) []byte {
	t.Helper()
	var data bytes.Buffer
	header := make([]byte, 18)
	header[2] = 2
	binary.LittleEndian.PutUint16(header[12:14], 256)
	binary.LittleEndian.PutUint16(header[14:16], 256)
	header[16] = 32
	header[17] = 0x28
	data.Write(header)

	index := iconID % iconsPerSheet
	startX := (index % iconsPerRow) * iconSize
	startY := (index / iconsPerRow) * iconSize
	for y := range 256 {
		for x := range 256 {
			pixel := color.NRGBA{}
			if x >= startX && x < startX+iconSize && y >= startY && y < startY+iconSize {
				pixel = iconColor
			}
			data.Write([]byte{pixel.B, pixel.G, pixel.R, pixel.A})
		}
	}
	return data.Bytes()
}
