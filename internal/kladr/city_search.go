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

	results := make([]CitySearchResult, 0, 64)
	for _, city := range a.Cities {
		n := normalizeForSearch(city.Name)
		if !strings.Contains(n, q) {
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
