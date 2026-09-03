package ach

import (
	"strings"

	achievments "github.com/uija/eqdps/internal/achievements"
)

func (m *Module) FilterData() {
	search := strings.ToLower(strings.TrimSpace(m.filter.Text()))
	filtered := &achievments.Export{
		Categories: make([]achievments.Category, 0, len(m.data.Categories)),
	}

	for _, category := range m.data.Categories {
		filteredCategory := achievments.Category{
			Name:          category.Name,
			Subcategories: make([]achievments.Subcategory, 0, len(category.Subcategories)),
		}
		for _, subcategory := range category.Subcategories {
			filteredSubcategory := achievments.Subcategory{
				Name:         subcategory.Name,
				Achievements: make([]achievments.Achievement, 0, len(subcategory.Achievements)),
			}
			for _, achievement := range subcategory.Achievements {
				if achievementMatches(achievement, search) {
					filteredSubcategory.Achievements = append(filteredSubcategory.Achievements, achievement)
				}
			}
			if len(filteredSubcategory.Achievements) > 0 {
				filteredCategory.Subcategories = append(filteredCategory.Subcategories, filteredSubcategory)
			}
		}
		if len(filteredCategory.Subcategories) > 0 {
			filtered.Categories = append(filtered.Categories, filteredCategory)
		}
	}

	m.filtered_data = filtered
}

func achievementMatches(achievement achievments.Achievement, search string) bool {
	if search == "" || strings.Contains(strings.ToLower(achievement.Name), search) {
		return true
	}
	for _, objective := range achievement.Objectives {
		if strings.Contains(strings.ToLower(objective.Text), search) {
			return true
		}
	}
	return false
}
