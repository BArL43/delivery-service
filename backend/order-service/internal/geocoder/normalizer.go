package geocoder

import (
	"regexp"
	"strings"
)

var normalizeRegex = regexp.MustCompile(`\s+`)

func NormalizeAddress(addres string) string {
	addr := strings.ToLower(addres)

	addr = strings.ReplaceAll(addr, ",", " ")
	addr = strings.ReplaceAll(addr, ".", " ")

	replacement := map[string]string{
		"улица":    "ул",
		"проспект": "пр",
		"переулок": "пер",
		"шоссе":    "ш",
		"площадь":  "пл",
		"квартал":  "кв",
		"дом":      "д",
		"корпус":   "к",
		"строение": "стр",
		"город":    "г",
		"посёлок":  "пос",
		"деревня":  "дер",
	}

	for old, new := range replacement {
		addr = strings.ReplaceAll(addr, old, new)
	}

	addr = normalizeRegex.ReplaceAllString(addr, " ")

	return strings.TrimSpace(addr)
}

func GenerateCacheKey(NormalizedAddress string) string {
	cleanKey := strings.ReplaceAll(NormalizedAddress, " ", "_")
	return "gocode:forward" + cleanKey
}
