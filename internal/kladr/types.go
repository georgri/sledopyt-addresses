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

func (a *AddressIndex) CityPath(code string) string {
	city := a.Cities[code]
	if city == nil {
		return code
	}

	parts := make([]string, 0, 4)
	for _, c := range ancestorCodes(code) {
		if parent := a.Cities[c]; parent != nil {
			parts = append(parts, parent.Name)
		}
	}
	parts = append(parts, city.Name)
	if len(parts) == 0 {
		return city.Name
	}
	// Dedupe repeating labels in malformed chains.
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) == 0 {
		return city.Name
	}
	res := out[0]
	for i := 1; i < len(out); i++ {
		res += " -> " + out[i]
	}
	return res
}

func ancestorCodes(code string) []string {
	if len(code) < 11 {
		return nil
	}
	var out []string
	// SS RRR GGG PPP (11 digits total)
	if code[8:11] != "000" {
		out = append(out, code[:8]+"000")
	}
	if code[5:8] != "000" {
		out = append(out, code[:5]+"000000")
	}
	if code[2:5] != "000" {
		out = append(out, code[:2]+"000000000")
	}
	// reverse so path goes root->leaf
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
