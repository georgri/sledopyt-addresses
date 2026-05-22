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
			return
		}
	}
	t.Fatalf("root moscow code %s not found in top 50", wanted)
}
