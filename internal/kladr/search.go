package kladr

import (
	"sort"
	"strings"

	"github.com/georgri/sledopyt-addresses/internal/formula"
)

type Match struct {
	City   string
	Street string
	House  string
}

func (a *AddressIndex) Find(cityCode string, f formula.Parsed, limit int) []Match {
	cities := a.citiesWithDescendants(cityCode)
	if len(cities) == 0 {
		return nil
	}

	results := make([]Match, 0, 128)
	seen := make(map[string]struct{}, 1024)
	for _, city := range cities {
		for _, street := range city.Streets {
			house, ok := f.Eval(street.LetterValues)
			if !ok {
				continue
			}
			house = strings.ToLower(house)
			if _, found := street.Houses[house]; found {
				key := strings.ToLower(city.Name) + "|" + strings.ToLower(street.DisplayName) + "|" + house
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				results = append(results, Match{
					City:   city.Name,
					Street: street.DisplayName,
					House:  house,
				})
			}
			if limit > 0 && len(results) >= limit {
				break
			}
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].City != results[j].City {
			return results[i].City < results[j].City
		}
		if results[i].Street == results[j].Street {
			return results[i].House < results[j].House
		}
		return results[i].Street < results[j].Street
	})
	return results
}

func (a *AddressIndex) citiesWithDescendants(cityCode string) []*City {
	prefix := CodePrefix(cityCode)
	if prefix == "" {
		return nil
	}

	out := make([]*City, 0, 64)
	for code, city := range a.Cities {
		if strings.HasPrefix(code, prefix) {
			if city.Socr != "" && !isSearchableCityType(city.Socr) {
				continue
			}
			out = append(out, city)
		}
	}
	return out
}
