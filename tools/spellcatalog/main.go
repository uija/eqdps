package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/uija/eqdps/internal/spellicon"
)

const (
	spellIDField        = 0
	spellNameField      = 1
	firstClassField     = 36
	spellGoneField      = 5
	newIconField        = 75
	requiredSpellFields = 76
)

var classNames = []string{
	"Warrior",
	"Cleric",
	"Paladin",
	"Ranger",
	"Shadowknight",
	"Druid",
	"Monk",
	"Bard",
	"Rogue",
	"Shaman",
	"Necromancer",
	"Wizard",
	"Magician",
	"Enchanter",
	"Beastlord",
	"Berserker",
}

type spell = spellicon.Spell

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "spellcatalog: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 3 {
		return errors.New("usage: spellcatalog <maxlevel> <pathtoeql> <targetpath>")
	}

	maxLevel, err := strconv.Atoi(args[0])
	if err != nil || maxLevel < 1 {
		return fmt.Errorf("maxlevel must be a positive integer")
	}

	fadeFile, err := os.Open(filepath.Join(args[1], "spells_us_str.txt"))
	if err != nil {
		return fmt.Errorf("open spells_us_str.txt: %w", err)
	}
	fades, err := readFadeMessages(fadeFile)
	closeErr := fadeFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close spells_us_str.txt: %w", closeErr)
	}

	spellFile, err := os.Open(filepath.Join(args[1], "spells_us.txt"))
	if err != nil {
		return fmt.Errorf("open spells_us.txt: %w", err)
	}
	spells, err := readSpells(spellFile, fades, maxLevel)
	closeErr = spellFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close spells_us.txt: %w", closeErr)
	}

	sort.Slice(spells, func(i, j int) bool {
		if spells[i].Name == spells[j].Name {
			return spells[i].FadeMessage < spells[j].FadeMessage
		}
		return spells[i].Name < spells[j].Name
	})

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(spells); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	if err := os.WriteFile(args[2], output.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", args[2], err)
	}

	fmt.Printf("Wrote %d spells to %s\n", len(spells), args[2])
	return nil
}

func readFadeMessages(r io.Reader) (map[string]string, error) {
	fades := make(map[string]string)
	err := scanLines(r, func(lineNumber int, line string) error {
		fields := strings.Split(line, "^")
		if len(fields) == 0 || strings.HasPrefix(fields[spellIDField], "#") {
			return nil
		}
		if len(fields) <= spellGoneField {
			return fmt.Errorf("spells_us_str.txt line %d has too few fields", lineNumber)
		}
		if fade := strings.TrimSpace(fields[spellGoneField]); fade != "" {
			fades[fields[spellIDField]] = fade
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fades, nil
}

func readSpells(r io.Reader, fades map[string]string, maxLevel int) ([]spell, error) {
	var spells []spell
	err := scanLines(r, func(lineNumber int, line string) error {
		fields := strings.Split(line, "^")
		if len(fields) < requiredSpellFields {
			return fmt.Errorf("spells_us.txt line %d has too few fields", lineNumber)
		}

		fade, ok := fades[fields[spellIDField]]
		if !ok {
			return nil
		}

		classes := make([]string, 0, len(classNames))
		for i, className := range classNames {
			levelText := strings.TrimSpace(fields[firstClassField+i])
			level, err := strconv.Atoi(levelText)
			if err != nil {
				return fmt.Errorf(
					"spells_us.txt line %d has invalid %s level %q",
					lineNumber,
					className,
					levelText,
				)
			}
			if level >= 1 && level <= maxLevel {
				classes = append(classes, className)
			}
		}
		if len(classes) == 0 {
			return nil
		}

		iconIDText := strings.TrimSpace(fields[newIconField])
		iconID, err := strconv.Atoi(iconIDText)
		if err != nil || iconID < 0 {
			return fmt.Errorf(
				"spells_us.txt line %d has invalid new_icon %q",
				lineNumber,
				iconIDText,
			)
		}

		name := fields[spellNameField]
		spells = append(spells, spell{
			Name:              name,
			FadeMessage:       fade,
			FadeMessageOthers: fadeMessageOthers(name),
			Classes:           classes,
			IconID:            iconID,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return spells, nil
}

func fadeMessageOthers(spellName string) string {
	return `^Your ` + regexp.QuoteMeta(spellName) + ` spell has worn off of .+\.$`
}

func scanLines(r io.Reader, visit func(lineNumber int, line string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if err := visit(lineNumber, line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	return nil
}
