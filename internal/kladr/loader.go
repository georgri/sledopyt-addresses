package kladr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	godbf "github.com/LindsayBradford/go-dbf"
	"github.com/bodgit/sevenzip"
)

var (
	reSplitHouses = regexp.MustCompile(`[,\s;/]+`)
	reRange       = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

func Load(sourcePath string) (*AddressIndex, error) {
	dbfDir, cleanup, err := ensureDBFDirectory(sourcePath)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	cityNames, err := loadCityNames(filepath.Join(dbfDir, "KLADR.DBF"))
	if err != nil {
		return nil, err
	}

	cities, streetByCode, err := loadStreets(filepath.Join(dbfDir, "STREET.DBF"), cityNames)
	if err != nil {
		return nil, err
	}

	if err := loadHouses(filepath.Join(dbfDir, "DOMA.DBF"), streetByCode); err != nil {
		return nil, err
	}

	filtered := make(map[string]*City, len(cities))
	for code, city := range cities {
		if len(city.Streets) == 0 {
			continue
		}
		filtered[code] = city
	}

	return &AddressIndex{
		Cities:             filtered,
		prefixStreetCounts: buildPrefixStreetCounts(filtered),
	}, nil
}

func ensureDBFDirectory(sourcePath string) (string, func(), error) {
	stat, err := os.Stat(sourcePath)
	if err != nil {
		return "", nil, fmt.Errorf("stat KLADR source path: %w", err)
	}
	if stat.IsDir() {
		return sourcePath, nil, nil
	}

	if !strings.HasSuffix(strings.ToLower(sourcePath), ".7z") {
		return "", nil, fmt.Errorf("KLADR_SOURCE_PATH must point to dir with DBF files or BASE.7z")
	}

	r, err := sevenzip.OpenReader(sourcePath)
	if err != nil {
		return "", nil, fmt.Errorf("open 7z: %w", err)
	}
	defer r.Close()

	tempDir, err := os.MkdirTemp("", "kladr-extract-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	needed := map[string]struct{}{
		"KLADR.DBF":  {},
		"STREET.DBF": {},
		"DOMA.DBF":   {},
	}

	for _, f := range r.File {
		name := strings.ToUpper(filepath.Base(f.Name))
		if _, ok := needed[name]; !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", nil, fmt.Errorf("open entry %s: %w", f.Name, err)
		}
		dstPath := filepath.Join(tempDir, name)
		dst, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			return "", nil, fmt.Errorf("create extracted file %s: %w", dstPath, err)
		}
		if _, err := io.Copy(dst, rc); err != nil {
			dst.Close()
			rc.Close()
			return "", nil, fmt.Errorf("extract %s: %w", f.Name, err)
		}
		dst.Close()
		rc.Close()
	}

	for file := range needed {
		if _, err := os.Stat(filepath.Join(tempDir, file)); err != nil {
			return "", nil, fmt.Errorf("missing %s in KLADR source", file)
		}
	}

	return tempDir, func() { _ = os.RemoveAll(tempDir) }, nil
}

func loadCityNames(kladrPath string) (map[string]string, error) {
	table, err := godbf.NewFromFile(kladrPath, "CP866")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", kladrPath, err)
	}

	out := make(map[string]string)
	for i := 0; i < table.NumberOfRecords(); i++ {
		code, err := table.FieldValueByName(i, "CODE")
		if err != nil {
			return nil, fmt.Errorf("read KLADR code: %w", err)
		}
		code = strings.TrimSpace(code)
		if len(code) < 11 {
			continue
		}
		if !isActualCode(code) {
			continue
		}

		name, err := table.FieldValueByName(i, "NAME")
		if err != nil {
			return nil, fmt.Errorf("read KLADR name: %w", err)
		}
		socr, err := table.FieldValueByName(i, "SOCR")
		if err != nil {
			return nil, fmt.Errorf("read KLADR socr: %w", err)
		}
		name = strings.TrimSpace(name)
		socr = strings.TrimSpace(socr)
		if name == "" {
			continue
		}

		// Keep only locality-level objects as candidate city labels.
		if code[5:11] == "000000" {
			continue
		}

		cityCode11 := code[:11]
		fullName := strings.TrimSpace(name + " " + socr)
		out[cityCode11] = fullName
	}

	return out, nil
}

func loadStreets(streetPath string, cityNames map[string]string) (map[string]*City, map[string]*Street, error) {
	table, err := godbf.NewFromFile(streetPath, "CP866")
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", streetPath, err)
	}

	cities := make(map[string]*City)
	streetByCode := make(map[string]*Street)

	for i := 0; i < table.NumberOfRecords(); i++ {
		code, err := table.FieldValueByName(i, "CODE")
		if err != nil {
			return nil, nil, fmt.Errorf("read STREET code: %w", err)
		}
		code = strings.TrimSpace(code)
		if len(code) < 17 {
			continue
		}
		if !isActualCode(code) {
			continue
		}
		cityCode11 := code[:11]
		streetCode17 := code[:17]

		name, err := table.FieldValueByName(i, "NAME")
		if err != nil {
			return nil, nil, fmt.Errorf("read STREET name: %w", err)
		}
		socr, err := table.FieldValueByName(i, "SOCR")
		if err != nil {
			return nil, nil, fmt.Errorf("read STREET socr: %w", err)
		}
		name = strings.TrimSpace(name)
		socr = strings.TrimSpace(socr)
		if name == "" {
			continue
		}
		display := strings.TrimSpace(name + " " + socr)

		city, ok := cities[cityCode11]
		if !ok {
			cityName := cityNames[cityCode11]
			if cityName == "" {
				cityName = cityCode11
			}
			city = &City{Code11: cityCode11, Name: cityName}
			cities[cityCode11] = city
		}

		street := &Street{
			Code17:       streetCode17,
			DisplayName:  display,
			LetterValues: streetLetterValues(name),
			Houses:       make(map[string]struct{}),
		}
		city.Streets = append(city.Streets, street)
		streetByCode[streetCode17] = street
	}

	return cities, streetByCode, nil
}

func loadHouses(domaPath string, streetByCode map[string]*Street) error {
	table, err := godbf.NewFromFile(domaPath, "CP866")
	if err != nil {
		return fmt.Errorf("open %s: %w", domaPath, err)
	}

	for i := 0; i < table.NumberOfRecords(); i++ {
		code, err := table.FieldValueByName(i, "CODE")
		if err != nil {
			return fmt.Errorf("read DOMA code: %w", err)
		}
		code = strings.TrimSpace(code)
		if len(code) < 17 {
			continue
		}
		streetCode := code[:17]
		street, ok := streetByCode[streetCode]
		if !ok {
			continue
		}

		name, err := table.FieldValueByName(i, "NAME")
		if err != nil {
			return fmt.Errorf("read DOMA name: %w", err)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		for _, h := range normalizeHouseList(name) {
			street.Houses[h] = struct{}{}
		}
	}
	return nil
}

func normalizeHouseList(raw string) []string {
	parts := reSplitHouses.Split(strings.ToLower(strings.TrimSpace(raw)), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		p = strings.Trim(p, ".")
		m := reRange.FindStringSubmatch(p)
		if len(m) == 3 {
			start, err1 := strconv.Atoi(m[1])
			end, err2 := strconv.Atoi(m[2])
			if err1 == nil && err2 == nil && start > 0 && end >= start && end-start <= 200 {
				for i := start; i <= end; i++ {
					out = append(out, strconv.Itoa(i))
				}
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

func isActualCode(code string) bool {
	if len(code) < 2 {
		return false
	}
	return code[len(code)-2:] == "00"
}
