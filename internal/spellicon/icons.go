package spellicon

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/uija/eqdps/internal/catalog"
)

const (
	iconsPerSheet = 36
	iconSize      = 40
	iconsPerRow   = 6
)

type Source struct {
	GameDir  string
	SheetDir string
}

type iconSet struct {
	name   string
	sheets map[int]string
}

func Detect(logFile string, spells []catalog.Spell) (Source, bool) {
	resolvedLog, err := filepath.EvalSymlinks(logFile)
	if err != nil {
		return Source{}, false
	}
	logDir := filepath.Dir(resolvedLog)
	if !strings.EqualFold(filepath.Base(logDir), "Logs") {
		return Source{}, false
	}

	gameDir := filepath.Dir(logDir)
	sheetDir := filepath.Join(gameDir, "uifiles", "default")
	requiredFiles := []string{
		filepath.Join(gameDir, "eqgame.exe"),
		filepath.Join(gameDir, "spells_us.txt"),
		filepath.Join(sheetDir, "EQUI_SpellIcons.xml"),
	}
	for _, path := range requiredFiles {
		if !isRegularFile(path) {
			return Source{}, false
		}
	}
	source := Source{GameDir: gameDir, SheetDir: sheetDir}
	if sets, err := discoverIconSets(source, spells); err != nil || len(sets) == 0 {
		return Source{}, false
	}
	return source, true
}

func Extract(source Source, targetDir string, spells []catalog.Spell) error {
	sets, err := discoverIconSets(source, spells)
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return fmt.Errorf("no compatible spell icon sets found")
	}
	return extractSet(sets[0], targetDir, spells)
}

func ExtractAll(source Source, targetDir string, spells []catalog.Spell) ([]string, error) {
	sets, err := discoverIconSets(source, spells)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(sets))
	for _, set := range sets {
		if err := extractSet(set, filepath.Join(targetDir, set.name), spells); err != nil {
			return nil, fmt.Errorf("extract icon set %q: %w", set.name, err)
		}
		names = append(names, set.name)
	}
	return names, nil
}

func extractSet(set iconSet, targetDir string, spells []catalog.Spell) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create spell icon directory: %w", err)
	}

	idsBySheet := make(map[int][]int)
	for _, spell := range spells {
		sheet := spell.IconID/iconsPerSheet + 1
		idsBySheet[sheet] = append(idsBySheet[sheet], spell.IconID)
	}

	for sheet, iconIDs := range idsBySheet {
		file, err := os.Open(set.sheets[sheet])
		if err != nil {
			return fmt.Errorf("open spell icon sheet %d: %w", sheet, err)
		}
		sheetImage, decodeErr := decodeTGA(file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode spell icon sheet %d: %w", sheet, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close spell icon sheet %d: %w", sheet, closeErr)
		}

		for _, iconID := range uniqueInts(iconIDs) {
			index := iconID % iconsPerSheet
			sourcePoint := image.Pt(
				(index%iconsPerRow)*iconSize,
				(index/iconsPerRow)*iconSize,
			)
			sourceRect := image.Rectangle{Min: sourcePoint, Max: sourcePoint.Add(image.Pt(iconSize, iconSize))}
			if !sourceRect.In(sheetImage.Bounds()) {
				return fmt.Errorf("icon %d is outside sheet %d", iconID, sheet)
			}

			icon := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
			draw.Draw(icon, icon.Bounds(), sheetImage, sourcePoint, draw.Src)
			if err := writePNG(IconPath(targetDir, iconID), icon); err != nil {
				return fmt.Errorf("write spell icon %d: %w", iconID, err)
			}
		}
	}
	return nil
}

func IconPath(iconDir string, iconID int) string {
	return filepath.Join(iconDir, fmt.Sprintf("spell_%d.png", iconID))
}

func SetIconPath(iconDir, setName string, iconID int) string {
	if setName == "" {
		return IconPath(iconDir, iconID)
	}
	return IconPath(filepath.Join(iconDir, filepath.Base(setName)), iconID)
}

func discoverIconSets(source Source, spells []catalog.Spell) ([]iconSet, error) {
	uiDir := filepath.Join(source.GameDir, "uifiles")
	entries, err := os.ReadDir(uiDir)
	if err != nil {
		return nil, fmt.Errorf("read EverQuest UI directory: %w", err)
	}
	required := requiredSheets(spells)
	defaultFiles, err := directoryFiles(source.SheetDir)
	if err != nil {
		return nil, fmt.Errorf("read default UI icon set: %w", err)
	}
	defaultSheets := make(map[int]string, len(required))
	for _, sheet := range required {
		path, ok := defaultFiles[strings.ToLower(sheetFilename(sheet))]
		if !ok {
			return nil, fmt.Errorf("default UI is missing spell icon sheet %d", sheet)
		}
		defaultSheets[sheet] = path
	}

	candidates := []string{source.SheetDir}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(uiDir, entry.Name())
		if samePath(path, source.SheetDir) {
			continue
		}
		files, readErr := directoryFiles(path)
		if readErr != nil || !containsSpellSheet(files) {
			continue
		}
		candidates = append(candidates, path)
	}

	seen := make(map[string]bool)
	sets := make([]iconSet, 0, len(candidates))
	for _, directory := range candidates {
		files, err := directoryFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("read UI icon set %q: %w", filepath.Base(directory), err)
		}
		sheets := make(map[int]string, len(required))
		for _, sheet := range required {
			path, ok := files[strings.ToLower(sheetFilename(sheet))]
			if !ok {
				path = defaultSheets[sheet]
			}
			sheets[sheet] = path
		}
		fingerprint, err := iconSetFingerprint(required, sheets)
		if err != nil {
			return nil, fmt.Errorf("fingerprint UI icon set %q: %w", filepath.Base(directory), err)
		}
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		sets = append(sets, iconSet{name: filepath.Base(directory), sheets: sheets})
	}
	return sets, nil
}

func directoryFiles(directory string) (map[string]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			files[strings.ToLower(entry.Name())] = filepath.Join(directory, entry.Name())
		}
	}
	return files, nil
}

func containsSpellSheet(files map[string]string) bool {
	for name := range files {
		if strings.HasPrefix(name, "spells") && strings.HasSuffix(name, ".tga") {
			number := strings.TrimSuffix(strings.TrimPrefix(name, "spells"), ".tga")
			if len(number) == 2 {
				if _, err := strconv.Atoi(number); err == nil {
					return true
				}
			}
		}
	}
	return false
}

func iconSetFingerprint(required []int, sheets map[int]string) (string, error) {
	hash := sha256.New()
	for _, sheet := range required {
		if err := binary.Write(hash, binary.LittleEndian, int32(sheet)); err != nil {
			return "", err
		}
		file, err := os.Open(sheets[sheet])
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func samePath(one, two string) bool {
	return strings.EqualFold(filepath.Clean(one), filepath.Clean(two))
}

func requiredSheets(spells []catalog.Spell) []int {
	seen := make(map[int]bool)
	for _, spell := range spells {
		seen[spell.IconID/iconsPerSheet+1] = true
	}
	sheets := make([]int, 0, len(seen))
	for sheet := range seen {
		sheets = append(sheets, sheet)
	}
	sort.Ints(sheets)
	return sheets
}

func sheetPath(sheetDir string, sheet int) string {
	return filepath.Join(sheetDir, sheetFilename(sheet))
}

func sheetFilename(sheet int) string {
	return fmt.Sprintf("Spells%02d.tga", sheet)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func uniqueInts(values []int) []int {
	seen := make(map[int]bool)
	result := make([]int, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func writePNG(path string, icon image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	encodeErr := png.Encode(writer, icon)
	flushErr := writer.Flush()
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

func decodeTGA(r io.Reader) (image.Image, error) {
	var header [18]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if header[1] != 0 {
		return nil, fmt.Errorf("color-mapped TGA images are unsupported")
	}
	imageType := header[2]
	if imageType != 2 && imageType != 10 {
		return nil, fmt.Errorf("unsupported TGA image type %d", imageType)
	}
	width := int(binary.LittleEndian.Uint16(header[12:14]))
	height := int(binary.LittleEndian.Uint16(header[14:16]))
	depth := int(header[16])
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil, fmt.Errorf("invalid TGA dimensions %dx%d", width, height)
	}
	if depth != 24 && depth != 32 {
		return nil, fmt.Errorf("unsupported TGA pixel depth %d", depth)
	}
	if _, err := io.CopyN(io.Discard, r, int64(header[0])); err != nil {
		return nil, err
	}

	bytesPerPixel := depth / 8
	target := image.NewNRGBA(image.Rect(0, 0, width, height))
	pixelIndex := 0
	setPixel := func(pixel []byte) {
		x := pixelIndex % width
		y := pixelIndex / width
		if header[17]&0x10 != 0 {
			x = width - 1 - x
		}
		if header[17]&0x20 == 0 {
			y = height - 1 - y
		}
		offset := target.PixOffset(x, y)
		target.Pix[offset] = pixel[2]
		target.Pix[offset+1] = pixel[1]
		target.Pix[offset+2] = pixel[0]
		target.Pix[offset+3] = 255
		if bytesPerPixel == 4 {
			target.Pix[offset+3] = pixel[3]
		}
		pixelIndex++
	}

	pixel := make([]byte, bytesPerPixel)
	if imageType == 2 {
		for pixelIndex < width*height {
			if _, err := io.ReadFull(r, pixel); err != nil {
				return nil, err
			}
			setPixel(pixel)
		}
		return target, nil
	}

	for pixelIndex < width*height {
		var packet [1]byte
		if _, err := io.ReadFull(r, packet[:]); err != nil {
			return nil, err
		}
		count := int(packet[0]&0x7f) + 1
		if pixelIndex+count > width*height {
			return nil, fmt.Errorf("TGA packet exceeds image dimensions")
		}
		if packet[0]&0x80 != 0 {
			if _, err := io.ReadFull(r, pixel); err != nil {
				return nil, err
			}
			for range count {
				setPixel(pixel)
			}
			continue
		}
		for range count {
			if _, err := io.ReadFull(r, pixel); err != nil {
				return nil, err
			}
			setPixel(pixel)
		}
	}
	return target, nil
}
