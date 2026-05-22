package kladr

import (
	"sort"
	"strings"
	"unicode"
)

type CitySearchResult struct {
	CityMeta
	StreetCount int
}

func (a *AddressIndex) FindCitiesByName(query string, limit int) []CitySearchResult {
	q := normalizeForSearch(query)
	if q == "" {
		return nil
	}
	tokens := strings.Fields(q)

	results := make([]CitySearchResult, 0, 64)
	for _, city := range a.Cities {
		if !isSearchableCityType(city.Socr) {
			continue
		}
		n := normalizeForSearch(city.Name)
		if !containsAllSubstrings(n, tokens) {
			continue
		}
		results = append(results, CitySearchResult{
			CityMeta: CityMeta{
				Code11: city.Code11,
				Name:   city.Name,
			},
			StreetCount: a.prefixStreetCount(city.Code11),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].StreetCount != results[j].StreetCount {
			return results[i].StreetCount > results[j].StreetCount
		}
		return results[i].Name < results[j].Name
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func normalizeForSearch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func containsAllSubstrings(haystack string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

func isSearchableCityType(socr string) bool {
	s := strings.ToLower(strings.TrimSpace(socr))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")

	switch s {
	case "г", "город",
		"рн", "район",
		"окр", "округ",
		"ао",
		"мкр", "микрорайон",
		"тер", "территория",
		"внтерг", "внутригородскаятерритория":
		return true
	default:
		return false
	}
}
