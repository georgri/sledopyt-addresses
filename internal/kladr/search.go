package kladr

import (
	"sort"
	"strings"

	"github.com/georgri/sledopyt-addresses/internal/formula"
)

type Match struct {
	Street string
	House  string
}

func (a *AddressIndex) Find(cityCode string, f formula.Parsed, limit int) []Match {
	city := a.Cities[cityCode]
	if city == nil {
		return nil
	}

	results := make([]Match, 0, 128)
	for _, street := range city.Streets {
		house, ok := f.Eval(street.LetterValues)
		if !ok {
			continue
		}
		house = strings.ToLower(house)
		if _, found := street.Houses[house]; found {
			results = append(results, Match{
				Street: street.DisplayName,
				House:  house,
			})
		}
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Street == results[j].Street {
			return results[i].House < results[j].House
		}
		return results[i].Street < results[j].Street
	})
	return results
}
