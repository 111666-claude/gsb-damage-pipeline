package damagepipe

import (
	"strconv"
	"strings"
)

func strconvFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func parseLast(entry string) float64 {
	_, text, found := strings.Cut(entry, "=")
	if !found {
		return -1
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return -1
	}
	return value
}
