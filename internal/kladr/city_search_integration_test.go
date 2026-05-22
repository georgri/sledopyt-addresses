package kladr

import (
	"os"
	"testing"
)

func TestMoscowRootInTopCitynameResults(t *testing.T) {
	source := os.Getenv("KLADR_TEST_SOURCE")
	if source == "" {
		t.Skip("KLADR_TEST_SOURCE is not set")
	}

	idx, err := Load(source)
	if err != nil {
		t.Fatalf("load kladr: %v", err)
	}

	results := idx.FindCitiesByName("москва", 50)
	if len(results) == 0 {
		t.Fatalf("no results for query")
	}

	const wanted = "77000000000"
	for _, r := range results {
		if r.Code11 == wanted {
			if r.Path == "" {
				t.Fatalf("expected non-empty hierarchy path for %s", wanted)
			}
			return
		}
	}
	t.Fatalf("root moscow code %s not found in top 50", wanted)
}

func TestCitySearchDoesNotReturnRuralTypes(t *testing.T) {
	source := os.Getenv("KLADR_TEST_SOURCE")
	if source == "" {
		t.Skip("KLADR_TEST_SOURCE is not set")
	}

	idx, err := Load(source)
	if err != nil {
		t.Fatalf("load kladr: %v", err)
	}

	results := idx.FindCitiesByName("москва", 100)
	for _, r := range results {
		city := idx.Cities[r.Code11]
		if city == nil {
			continue
		}
		if !isSearchableCityType(city.Socr) {
			t.Fatalf("found non-city type in result: %s (%s)", city.Name, city.Socr)
		}
	}
}
 