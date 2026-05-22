package kladr

import (
	"sort"
	"strings"
	"unicode"
)

type CitySearchResult struct {
	CityMeta
	Score int
}

func (a *AddressIndex) FindCitiesByName(query string, limit int) []CitySearchResult {
	q := normalizeForSearch(query)
	if q == "" {
		return nil
	}

	results := make([]CitySearchResult, 0, 64)
	for _, city := range a.Cities {
		n := normalizeForSearch(city.Name)
		score, ok := cityScore(q, n)
		if !ok {
			continue
		}
		results = append(results, CitySearchResult{
			CityMeta: CityMeta{
				Code11: city.Code11,
				Name:   city.Name,
			},
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score < results[j].Score
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

func cityScore(query, name string) (int, bool) {
	switch {
	case name == query:
		return 0, true
	case strings.HasPrefix(name, query):
		return 1, true
	case strings.Contains(name, query):
		return 2, true
	case isSubsequence(query, name):
		return 3, true
	default:
		return 0, false
	}
}

func isSubsequence(needle, haystack string) bool {
	rn := []rune(needle)
	rh := []rune(haystack)
	j := 0
	for i := 0; i < len(rh) && j < len(rn); i++ {
		if rh[i] == rn[j] {
			j++
		}
	}
	return j == len(rn)
}
