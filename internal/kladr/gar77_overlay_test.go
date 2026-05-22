package kladr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeGAR77Overlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.json")
	jsonData := `{
  "regions": [
    {
      "code": "gar77-1",
      "name": "Москва, муниципальный округ Арбат",
      "socr": "вн.тер.г.",
      "streets": [
        {
          "name": "Арбат",
          "type": "ул",
          "houses": ["1", "10б", "  ", "1"]
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(jsonData), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	idx := &AddressIndex{Cities: map[string]*City{}}
	if err := idx.MergeGAR77Overlay(path); err != nil {
		t.Fatalf("merge overlay: %v", err)
	}

	city := idx.Cities["gar77-1"]
	if city == nil {
		t.Fatalf("overlay city not loaded")
	}
	if city.Name != "Москва, муниципальный округ Арбат" {
		t.Fatalf("unexpected city name: %q", city.Name)
	}
	if len(city.Streets) != 1 {
		t.Fatalf("expected 1 street, got %d", len(city.Streets))
	}
	st := city.Streets[0]
	if st.DisplayName != "Арбат ул" {
		t.Fatalf("unexpected display name: %q", st.DisplayName)
	}
	if _, ok := st.Houses["1"]; !ok {
		t.Fatalf("expected house 1")
	}
	if _, ok := st.Houses["10б"]; !ok {
		t.Fatalf("expected house 10б")
	}
	if got := idx.prefixStreetCount("gar77-1"); got != 1 {
		t.Fatalf("unexpected prefix street count: %d", got)
	}
}
