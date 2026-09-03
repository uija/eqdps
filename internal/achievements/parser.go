package achievments

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gioui.org/widget"
)

// Export is the achievement tree contained in an EverQuest achievement
// export file.
type Export struct {
	Created    time.Time  `json:"time_created"`
	Categories []Category `json:"categories"`
}

type Category struct {
	Name          string        `json:"name"`
	Subcategories []Subcategory `json:"subcategories"`
}

type Subcategory struct {
	Name         string        `json:"name"`
	Achievements []Achievement `json:"achievements"`
}

type Achievement struct {
	Name       string           `json:"name"`
	Complete   bool             `json:"complete"`
	Objectives []Objective      `json:"objectives"`
	Open       bool             `json:"-"`
	Click      widget.Clickable `json:"-"`
}

type Objective struct {
	Text     string    `json:"text"`
	Complete bool      `json:"complete"`
	Progress *Progress `json:"progress,omitempty"`
}

type Progress struct {
	Current  int `json:"current"`
	Required int `json:"required"`
}

// Parse reads an EverQuest achievement export and returns its contents as a
// category, subcategory, achievement and objective tree.
func Parse(path string) (*Export, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open achievement export: %w", err)
	}
	defer file.Close()

	result := &Export{Categories: make([]Category, 0), Created: time.Now()}
	categoryIndex := -1
	subcategoryIndex := -1
	achievementIndex := -1

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 1:
			categoryName, subcategoryName, ok := strings.Cut(fields[0], ": ")
			if !ok || strings.TrimSpace(categoryName) == "" || strings.TrimSpace(subcategoryName) == "" {
				return nil, fmt.Errorf("parse achievement export line %d: invalid category %q", lineNumber, line)
			}

			categoryIndex = findCategory(result.Categories, categoryName)
			if categoryIndex < 0 {
				result.Categories = append(result.Categories, Category{Name: categoryName})
				categoryIndex = len(result.Categories) - 1
			}
			category := &result.Categories[categoryIndex]
			subcategoryIndex = findSubcategory(category.Subcategories, subcategoryName)
			if subcategoryIndex < 0 {
				category.Subcategories = append(category.Subcategories, Subcategory{Name: subcategoryName})
				subcategoryIndex = len(category.Subcategories) - 1
			}
			achievementIndex = -1

		case 2:
			if categoryIndex < 0 || subcategoryIndex < 0 {
				return nil, fmt.Errorf("parse achievement export line %d: achievement has no category", lineNumber)
			}
			complete, err := parseStatus(fields[0])
			if err != nil {
				return nil, fmt.Errorf("parse achievement export line %d: %w", lineNumber, err)
			}
			if strings.TrimSpace(fields[1]) == "" {
				return nil, fmt.Errorf("parse achievement export line %d: achievement name is empty", lineNumber)
			}

			subcategory := &result.Categories[categoryIndex].Subcategories[subcategoryIndex]
			subcategory.Achievements = append(subcategory.Achievements, Achievement{
				Name:     fields[1],
				Complete: complete,
			})
			achievementIndex = len(subcategory.Achievements) - 1

		case 3, 4:
			if categoryIndex < 0 || subcategoryIndex < 0 || achievementIndex < 0 {
				return nil, fmt.Errorf("parse achievement export line %d: objective has no achievement", lineNumber)
			}
			complete, err := parseStatus(fields[0])
			if err != nil {
				return nil, fmt.Errorf("parse achievement export line %d: %w", lineNumber, err)
			}
			if fields[1] != "" || strings.TrimSpace(fields[2]) == "" {
				return nil, fmt.Errorf("parse achievement export line %d: invalid objective", lineNumber)
			}

			objective := Objective{Text: fields[2], Complete: complete}
			if len(fields) == 4 {
				progress, err := parseProgress(fields[3])
				if err != nil {
					return nil, fmt.Errorf("parse achievement export line %d: %w", lineNumber, err)
				}
				objective.Progress = progress
			}

			achievement := &result.Categories[categoryIndex].Subcategories[subcategoryIndex].Achievements[achievementIndex]
			achievement.Objectives = append(achievement.Objectives, objective)

		default:
			return nil, fmt.Errorf("parse achievement export line %d: unexpected field count %d", lineNumber, len(fields))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read achievement export: %w", err)
	}
	return result, nil
}

func parseStatus(value string) (bool, error) {
	switch value {
	case "C":
		return true, nil
	case "I":
		return false, nil
	default:
		return false, fmt.Errorf("invalid completion status %q", value)
	}
}

func parseProgress(value string) (*Progress, error) {
	currentText, requiredText, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return nil, fmt.Errorf("invalid progress %q", value)
	}
	current, err := strconv.Atoi(currentText)
	if err != nil || current < 0 {
		return nil, fmt.Errorf("invalid current progress %q", currentText)
	}
	required, err := strconv.Atoi(requiredText)
	if err != nil || required < 1 {
		return nil, fmt.Errorf("invalid required progress %q", requiredText)
	}
	return &Progress{Current: current, Required: required}, nil
}

func findCategory(categories []Category, name string) int {
	for index := range categories {
		if categories[index].Name == name {
			return index
		}
	}
	return -1
}

func findSubcategory(subcategories []Subcategory, name string) int {
	for index := range subcategories {
		if subcategories[index].Name == name {
			return index
		}
	}
	return -1
}
