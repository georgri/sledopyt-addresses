package kladr

import "sort"

type AddressIndex struct {
	Cities              map[string]*City
	prefixStreetCounts map[string]int
}

type City struct {
	Code11  string
	Name    string
	Socr    string
	Streets []*Street
}

type Street struct {
	Code17       string
	DisplayName  string
	LetterValues []int
	Houses       map[string]struct{}
}

type CityMeta struct {
	Code11 string `json:"code"`
	Name   string `json:"name"`
}

func (a *AddressIndex) CityList() []CityMeta {
	out := make([]CityMeta, 0, len(a.Cities))
	for _, c := range a.Cities {
		out = append(out, CityMeta{Code11: c.Code11, Name: c.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func CodePrefix(code string) string {
	i := len(code)
	for i > 0 && code[i-1] == '0' {
		i--
	}
	if i == 0 {
		return code
	}
	return code[:i]
}

func buildPrefixStreetCounts(cities map[string]*City) map[string]int {
	counts := make(map[string]int, len(cities)*2)
	for code, city := range cities {
		direct := len(city.Streets)
		if direct == 0 {
			continue
		}
		for i := 1; i <= len(code); i++ {
			prefix := code[:i]
			counts[prefix] += direct
		}
	}
	return counts
}

func (a *AddressIndex) prefixStreetCount(code string) int {
	if a == nil || a.prefixStreetCounts == nil {
		return 0
	}
	return a.prefixStreetCounts[CodePrefix(code)]
}
