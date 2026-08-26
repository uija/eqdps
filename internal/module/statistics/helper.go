package statistics

import (
	"strconv"
	"strings"
)

func FormatMoney(copper int64) string {
	parts := make([]string, 0, 4)
	values := []struct {
		amount int64
		suffix string
	}{
		{amount: copper / 1000, suffix: "P"},
		{amount: copper % 1000 / 100, suffix: "G"},
		{amount: copper % 100 / 10, suffix: "S"},
		{amount: copper % 10, suffix: "C"},
	}
	for _, value := range values {
		if value.amount != 0 {
			parts = append(parts, strconv.FormatInt(value.amount, 10)+value.suffix)
		}
	}
	return strings.Join(parts, " ")
}
