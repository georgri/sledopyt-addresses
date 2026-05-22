package kladr

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type gar77Overlay struct {
	Regions []gar77Region `json:"regions"`
}

type gar77Region struct {
	Code    string        `json:"code"`
	Name    string        `json:"name"`
	Socr    string        `json:"socr"`
	Streets []gar77Street `json:"streets"`
}

type gar77Street struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Houses []string `json:"houses"`
}

func (a *AddressIndex) MergeGAROverlay(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read GAR overlay %s: %w", path, err)
	}
	var overlay gar77Overlay
	if err := json.Unmarshal(data, &overlay); err != nil {
		return fmt.Errorf("parse GAR overlay %s: %w", path, err)
	}
	if len(overlay.Regions) == 0 {
		return nil
	}
	if a.Cities == nil {
		a.Cities = make(map[string]*City)
	}

	for _, region := range overlay.Regions {
		code := strings.TrimSpace(region.Code)
		if code == "" {
			continue
		}
		name := strings.TrimSpace(region.Name)
		if name == "" {
			name = code
		}
		city := &City{
			Code11: code,
			Name:   name,
			Socr:   strings.TrimSpace(region.Socr),
		}
		for i, st := range region.Streets {
			stName := strings.TrimSpace(st.Name)
			if stName == "" {
				continue
			}
			stType := strings.TrimSpace(st.Type)
			display := stName
			if stType != "" {
				display = strings.TrimSpace(stName + " " + stType)
			}
			houses := make(map[string]struct{}, len(st.Houses))
			for _, h := range st.Houses {
				h = strings.ToLower(strings.TrimSpace(h))
				if h == "" {
					continue
				}
				houses[h] = struct{}{}
			}
			if len(houses) == 0 {
				continue
			}
			city.Streets = append(city.Streets, &Street{
				Code17:       fmt.Sprintf("%s:%06d", code, i+1),
				DisplayName:  display,
				LetterValues: streetLetterValues(stName),
				Houses:       houses,
			})
		}
		if len(city.Streets) == 0 {
			continue
		}
		a.Cities[code] = city
	}
	a.prefixStreetCounts = buildPrefixStreetCounts(a.Cities)
	return nil
}

func (a *AddressIndex) MergeGAR77Overlay(path string) error {
	return a.MergeGAROverlay(path)
}
