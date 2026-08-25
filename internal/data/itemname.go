package data

import (
	"regexp"
	"strconv"
	"strings"
)

var itemUpgradeSuffixRE = regexp.MustCompile(` \+[0-9]+$`)

func NormalizeItemName(value string) (int, string) {
	value = strings.TrimSpace(value)
	prefix, name, found := strings.Cut(value, " ")
	if !found {
		return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
	}

	quantity := 1
	if !strings.EqualFold(prefix, "a") && !strings.EqualFold(prefix, "an") {
		parsed, err := strconv.Atoi(prefix)
		if err != nil || parsed < 1 {
			return 1, itemUpgradeSuffixRE.ReplaceAllString(value, "")
		}
		quantity = parsed
	}

	name = itemUpgradeSuffixRE.ReplaceAllString(strings.TrimSpace(name), "")
	return quantity, name
}
