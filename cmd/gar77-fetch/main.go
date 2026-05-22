package main

import (
	"compress/flate"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	fiasDownloadInfoURL = "https://fias.nalog.ru/WebServices/Public/GetAllDownloadFileInfo"
	defaultRegions      = "77,78"
	defaultTailSize     = 1 << 20
)

type downloadInfo struct {
	GarXMLFullURL string `json:"GarXMLFullURL"`
}

type remoteZip struct {
	url          string
	contentSize  int64
	httpClient   *http.Client
	entriesByKey map[string]zipEntry
}

type zipEntry struct {
	Name              string
	Method            uint16
	CompressedSize    uint64
	UncompressedSize  uint64
	LocalHeaderOffset uint64
}

type addrObject struct {
	ObjectID string
	Name     string
	TypeName string
	Level    string
	IsActual string
	IsActive string
}

type houseObject struct {
	ObjectID string
	HouseNum string
	AddNum1  string
	AddNum2  string
	IsActual string
	IsActive string
}

type hierarchyItem struct {
	ObjectID string
	IsActive string
	Path     string
}

type streetMeta struct {
	ID   int64
	Name string
	Type string
}

type regionMeta struct {
	ID   int64
	Name string
	Socr string
}

type overlayData struct {
	Regions []overlayRegion `json:"regions"`
}

type overlayRegion struct {
	Code    string          `json:"code"`
	Name    string          `json:"name"`
	Socr    string          `json:"socr"`
	Streets []overlayStreet `json:"streets"`
}

type overlayStreet struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Houses []string `json:"houses"`
}

func main() {
	var (
		zipURL     = flag.String("zip-url", "", "GAR ZIP URL (default: latest official from FIAS service)")
		regionsArg = flag.String("regions", defaultRegions, "comma-separated region codes inside gar_xml.zip, for example: 77,78")
		outPath    = flag.String("out", "data/gar_overlay.json", "output JSON path")
	)
	flag.Parse()
	regionCodes := splitRegionCodes(*regionsArg)
	if len(regionCodes) == 0 {
		log.Fatalf("at least one region code is required")
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	url := strings.TrimSpace(*zipURL)
	if url == "" {
		var err error
		url, err = latestGarURL(client)
		if err != nil {
			log.Fatalf("detect latest GAR URL: %v", err)
		}
	}

	log.Printf("GAR ZIP URL: %s", url)
	log.Printf("Regions: %s", strings.Join(regionCodes, ", "))

	z, err := newRemoteZip(client, url)
	if err != nil {
		log.Fatalf("open remote zip: %v", err)
	}

	overlay := overlayData{Regions: make([]overlayRegion, 0, 512)}
	for _, regionCode := range regionCodes {
		required := map[string]string{
			"addr":         findRegionEntry(z, regionCode, "AS_ADDR_OBJ_"),
			"houses":       findRegionEntry(z, regionCode, "AS_HOUSES_"),
			"admHierarchy": findRegionEntry(z, regionCode, "AS_ADM_HIERARCHY_"),
			"munHierarchy": findRegionEntry(z, regionCode, "AS_MUN_HIERARCHY_"),
		}
		for key, entryName := range required {
			if entryName == "" {
				log.Printf("region %s skipped: required entry not found for %s", regionCode, key)
				required = nil
				break
			}
			log.Printf("region %s entry %-9s: %s", regionCode, key, entryName)
		}
		if required == nil {
			continue
		}

		regionOverlay, err := buildOverlay(
			z,
			regionCode,
			required["addr"],
			required["houses"],
			required["admHierarchy"],
			required["munHierarchy"],
		)
		if err != nil {
			log.Fatalf("build overlay for region %s: %v", regionCode, err)
		}
		overlay.Regions = append(overlay.Regions, regionOverlay.Regions...)
	}
	if len(overlay.Regions) == 0 {
		log.Fatalf("no regions were exported")
	}

	if err := os.MkdirAll(dirOf(*outPath), 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	f, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("create output file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(overlay); err != nil {
		log.Fatalf("write output json: %v", err)
	}

	regionCount := len(overlay.Regions)
	streetCount := 0
	for _, r := range overlay.Regions {
		streetCount += len(r.Streets)
	}
	log.Printf("done: %d regions, %d streets -> %s", regionCount, streetCount, *outPath)
}

func latestGarURL(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fiasDownloadInfoURL, nil)
	if err == nil {
		resp, reqErr := client.Do(req)
		if reqErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode/100 == 2 {
				var infos []downloadInfo
				if decErr := json.NewDecoder(resp.Body).Decode(&infos); decErr == nil {
					for _, info := range infos {
						if strings.TrimSpace(info.GarXMLFullURL) != "" {
							return info.GarXMLFullURL, nil
						}
					}
				}
			}
		}
	}
	return fallbackLatestGarURL(client)
}

func fallbackLatestGarURL(client *http.Client) (string, error) {
	now := time.Now()
	for dayShift := 1; dayShift <= 14; dayShift++ {
		d := now.AddDate(0, 0, -dayShift)
		candidate := fmt.Sprintf("https://fias-file.nalog.ru/downloads/%04d.%02d.%02d/gar_xml.zip", d.Year(), d.Month(), d.Day())
		req, err := http.NewRequest(http.MethodHead, candidate, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return candidate, nil
		}
	}
	return "", errors.New("GAR XML URL not found in metadata or date fallback")
}

func newRemoteZip(client *http.Client, url string) (*remoteZip, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HEAD status: %s", resp.Status)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes") {
		return nil, errors.New("server does not support byte ranges")
	}
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil || size <= 0 {
		return nil, fmt.Errorf("bad content-length: %q", resp.Header.Get("Content-Length"))
	}

	z := &remoteZip{
		url:         url,
		contentSize: size,
		httpClient:  client,
	}
	if err := z.loadCentralDirectory(); err != nil {
		return nil, err
	}
	return z, nil
}

func (z *remoteZip) loadCentralDirectory() error {
	tailSize := int64(defaultTailSize)
	if z.contentSize < tailSize {
		tailSize = z.contentSize
	}
	tailStart := z.contentSize - tailSize
	tail, err := z.getRange(tailStart, z.contentSize-1)
	if err != nil {
		return err
	}

	eocdPos := bytesLastIndex(tail, []byte{'P', 'K', 0x05, 0x06})
	if eocdPos < 0 || eocdPos+22 > len(tail) {
		return errors.New("EOCD not found in ZIP tail")
	}
	eocd := tail[eocdPos : eocdPos+22]
	totalEntries16 := u16le(eocd[10:12])
	cdSize32 := u32le(eocd[12:16])
	cdOff32 := u32le(eocd[16:20])

	var (
		cdSize uint64
		cdOff  uint64
	)
	isZip64 := totalEntries16 == 0xFFFF || cdSize32 == 0xFFFFFFFF || cdOff32 == 0xFFFFFFFF
	if !isZip64 {
		cdSize = uint64(cdSize32)
		cdOff = uint64(cdOff32)
	} else {
		locPos := bytesLastIndex(tail[:eocdPos], []byte{'P', 'K', 0x06, 0x07})
		if locPos < 0 || locPos+20 > len(tail) {
			return errors.New("ZIP64 locator not found")
		}
		zip64EOCDOff := u64le(tail[locPos+8 : locPos+16])
		z64, err := z.getRange(int64(zip64EOCDOff), int64(zip64EOCDOff)+255)
		if err != nil {
			return err
		}
		if len(z64) < 56 || string(z64[:4]) != "PK\x06\x06" {
			return errors.New("invalid ZIP64 EOCD header")
		}
		cdSize = u64le(z64[40:48])
		cdOff = u64le(z64[48:56])
	}

	if cdSize == 0 {
		return errors.New("empty central directory")
	}

	cd, err := z.getRange(int64(cdOff), int64(cdOff+cdSize-1))
	if err != nil {
		return err
	}

	entries := make(map[string]zipEntry, 2048)
	for p := 0; p+46 <= len(cd); {
		if string(cd[p:p+4]) != "PK\x01\x02" {
			break
		}
		method := u16le(cd[p+10 : p+12])
		csize32 := u32le(cd[p+20 : p+24])
		usize32 := u32le(cd[p+24 : p+28])
		fnl := int(u16le(cd[p+28 : p+30]))
		exl := int(u16le(cd[p+30 : p+32]))
		coml := int(u16le(cd[p+32 : p+34]))
		lhOff32 := u32le(cd[p+42 : p+46])
		if p+46+fnl+exl+coml > len(cd) {
			return errors.New("truncated central directory entry")
		}
		name := string(cd[p+46 : p+46+fnl])
		extra := cd[p+46+fnl : p+46+fnl+exl]

		entry := zipEntry{
			Name:              name,
			Method:            method,
			CompressedSize:    uint64(csize32),
			UncompressedSize:  uint64(usize32),
			LocalHeaderOffset: uint64(lhOff32),
		}

		if csize32 == 0xFFFFFFFF || usize32 == 0xFFFFFFFF || lhOff32 == 0xFFFFFFFF {
			if err := fillZip64Fields(extra, &entry, csize32 == 0xFFFFFFFF, usize32 == 0xFFFFFFFF, lhOff32 == 0xFFFFFFFF); err != nil {
				return fmt.Errorf("zip64 extra for %s: %w", name, err)
			}
		}

		entries[name] = entry
		p += 46 + fnl + exl + coml
	}
	z.entriesByKey = entries
	return nil
}

func fillZip64Fields(extra []byte, entry *zipEntry, needCSize, needUSize, needOffset bool) error {
	for p := 0; p+4 <= len(extra); {
		id := u16le(extra[p : p+2])
		l := int(u16le(extra[p+2 : p+4]))
		if p+4+l > len(extra) {
			return errors.New("zip64 extra truncated")
		}
		data := extra[p+4 : p+4+l]
		if id == 0x0001 {
			q := 0
			if needUSize {
				if q+8 > len(data) {
					return errors.New("missing usize")
				}
				entry.UncompressedSize = u64le(data[q : q+8])
				q += 8
			}
			if needCSize {
				if q+8 > len(data) {
					return errors.New("missing csize")
				}
				entry.CompressedSize = u64le(data[q : q+8])
				q += 8
			}
			if needOffset {
				if q+8 > len(data) {
					return errors.New("missing local header offset")
				}
				entry.LocalHeaderOffset = u64le(data[q : q+8])
				q += 8
			}
			return nil
		}
		p += 4 + l
	}
	return errors.New("zip64 extra record (0x0001) not found")
}

func (z *remoteZip) openEntry(entryName string) (io.ReadCloser, error) {
	e, ok := z.entriesByKey[entryName]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", entryName)
	}
	if e.Method != 8 {
		return nil, fmt.Errorf("unsupported method %d for %s", e.Method, entryName)
	}

	hdr, err := z.getRange(int64(e.LocalHeaderOffset), int64(e.LocalHeaderOffset)+4095)
	if err != nil {
		return nil, err
	}
	if len(hdr) < 30 || string(hdr[:4]) != "PK\x03\x04" {
		return nil, fmt.Errorf("bad local header for %s", entryName)
	}
	fnl := u16le(hdr[26:28])
	exl := u16le(hdr[28:30])
	dataOff := int64(e.LocalHeaderOffset) + 30 + int64(fnl) + int64(exl)
	dataEnd := dataOff + int64(e.CompressedSize) - 1
	if dataEnd < dataOff {
		return nil, fmt.Errorf("invalid compressed size for %s", entryName)
	}

	resp, err := z.getRangeResponse(dataOff, dataEnd)
	if err != nil {
		return nil, err
	}
	fr := flate.NewReader(resp.Body)
	return &readCloserChain{
		reader: fr,
		closers: []io.Closer{
			fr,
			resp.Body,
		},
	}, nil
}

type readCloserChain struct {
	reader  io.Reader
	closers []io.Closer
}

func (r *readCloserChain) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *readCloserChain) Close() error {
	var firstErr error
	for _, c := range r.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (z *remoteZip) getRange(start, end int64) ([]byte, error) {
	resp, err := z.getRangeResponse(start, end)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (z *remoteZip) getRangeResponse(start, end int64) (*http.Response, error) {
	if start < 0 || end < start {
		return nil, fmt.Errorf("invalid range %d-%d", start, end)
	}
	req, err := http.NewRequest(http.MethodGet, z.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := z.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("range status: %s", resp.Status)
	}
	return resp, nil
}

func findRegionEntry(z *remoteZip, regionCode, prefix string) string {
	p := regionCode + "/" + prefix
	candidates := make([]string, 0, 4)
	for name := range z.entriesByKey {
		if strings.HasPrefix(name, p) &&
			!strings.Contains(name, "_PARAMS_") &&
			!strings.Contains(name, "_DIVISION_") &&
			strings.HasSuffix(strings.ToUpper(name), ".XML") {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[len(candidates)-1]
}

func splitRegionCodes(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) == 1 {
			p = "0" + p
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func buildOverlay(z *remoteZip, regionCode, addrEntry, housesEntry, admHierarchyEntry, munHierarchyEntry string) (overlayData, error) {
	log.Printf("Parsing %s", addrEntry)
	streets, regions, err := parseAddrObjects(z, addrEntry)
	if err != nil {
		return overlayData{}, err
	}
	log.Printf("Active streets: %d, candidate regions: %d", len(streets), len(regions))

	log.Printf("Parsing %s (pass 1)", housesEntry)
	activeHouseIDs, err := collectActiveHouseIDs(z, housesEntry)
	if err != nil {
		return overlayData{}, err
	}
	log.Printf("Active houses: %d", len(activeHouseIDs))

	log.Printf("Parsing %s (streets -> regions)", munHierarchyEntry)
	streetToRegion, _, err := parseHierarchy(z, munHierarchyEntry, streets, regions, map[int64]struct{}{})
	if err != nil {
		return overlayData{}, err
	}
	log.Printf("Parsing %s (houses -> streets)", admHierarchyEntry)
	_, houseToStreet, err := parseHierarchy(z, admHierarchyEntry, streets, map[int64]regionMeta{}, activeHouseIDs)
	if err != nil {
		return overlayData{}, err
	}
	log.Printf("Mapped streets->regions: %d, houses->streets: %d", len(streetToRegion), len(houseToStreet))

	log.Printf("Parsing %s (pass 2)", housesEntry)
	streetHouses, err := collectStreetHouses(z, housesEntry, houseToStreet)
	if err != nil {
		return overlayData{}, err
	}
	log.Printf("Streets with houses: %d", len(streetHouses))

	regionStreets := make(map[int64]map[int64]streetMeta)
	for sid, rid := range streetToRegion {
		sm, okStreet := streets[sid]
		if !okStreet {
			continue
		}
		if _, ok := streetHouses[sid]; !ok {
			continue
		}
		if _, ok := regions[rid]; !ok {
			continue
		}
		m := regionStreets[rid]
		if m == nil {
			m = make(map[int64]streetMeta)
			regionStreets[rid] = m
		}
		m[sid] = sm
	}

	regionIDs := make([]int64, 0, len(regionStreets))
	for rid := range regionStreets {
		regionIDs = append(regionIDs, rid)
	}
	sort.Slice(regionIDs, func(i, j int) bool {
		return regions[regionIDs[i]].Name < regions[regionIDs[j]].Name
	})

	out := overlayData{Regions: make([]overlayRegion, 0, len(regionIDs))}
	for _, rid := range regionIDs {
		rm := regions[rid]
		streetsMap := regionStreets[rid]
		streetIDs := make([]int64, 0, len(streetsMap))
		for sid := range streetsMap {
			streetIDs = append(streetIDs, sid)
		}
		sort.Slice(streetIDs, func(i, j int) bool {
			a := streetsMap[streetIDs[i]]
			b := streetsMap[streetIDs[j]]
			if a.Name == b.Name {
				return a.Type < b.Type
			}
			return a.Name < b.Name
		})

		or := overlayRegion{
			Code:    fmt.Sprintf("gar%s-%d", regionCode, rid),
			Name:    rm.Name,
			Socr:    rm.Socr,
			Streets: make([]overlayStreet, 0, len(streetIDs)),
		}
		for _, sid := range streetIDs {
			sm := streetsMap[sid]
			houseSet := streetHouses[sid]
			houses := make([]string, 0, len(houseSet))
			for h := range houseSet {
				houses = append(houses, h)
			}
			sort.Strings(houses)
			or.Streets = append(or.Streets, overlayStreet{
				Name:   sm.Name,
				Type:   sm.Type,
				Houses: houses,
			})
		}
		out.Regions = append(out.Regions, or)
	}
	return out, nil
}

func parseAddrObjects(z *remoteZip, entryName string) (map[int64]streetMeta, map[int64]regionMeta, error) {
	rc, err := z.openEntry(entryName)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()

	streets := make(map[int64]streetMeta, 8192)
	regions := make(map[int64]regionMeta, 256)

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "OBJECT" {
			continue
		}
		obj := addrObject{
			ObjectID: attr(se.Attr, "OBJECTID"),
			Name:     attr(se.Attr, "NAME"),
			TypeName: attr(se.Attr, "TYPENAME"),
			Level:    attr(se.Attr, "LEVEL"),
			IsActual: attr(se.Attr, "ISACTUAL"),
			IsActive: attr(se.Attr, "ISACTIVE"),
		}
		if obj.IsActive != "1" || obj.IsActual != "1" {
			continue
		}
		id, ok := parseInt64(obj.ObjectID)
		if !ok {
			continue
		}
		level, _ := strconv.Atoi(strings.TrimSpace(obj.Level))
		name := strings.TrimSpace(obj.Name)
		typ := strings.TrimSpace(obj.TypeName)
		if name == "" {
			continue
		}

		if level == 8 {
			streets[id] = streetMeta{ID: id, Name: name, Type: typ}
		}
		if isMoscowAdministrativeRegion(level, name, typ) {
			full := strings.TrimSpace(name)
			if typ != "" && !containsFold(full, typ) {
				full = strings.TrimSpace(full + " " + typ)
			}
			if !containsFold(full, "москва") {
				full = "Москва, " + full
			}
			socr := typ
			if socr == "" {
				socr = "вн.тер.г."
			}
			regions[id] = regionMeta{ID: id, Name: full, Socr: socr}
		}
	}
	return streets, regions, nil
}

func collectActiveHouseIDs(z *remoteZip, entryName string) (map[int64]struct{}, error) {
	rc, err := z.openEntry(entryName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	out := make(map[int64]struct{}, 2_000_000)
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "HOUSE" {
			continue
		}
		if attr(se.Attr, "ISACTIVE") != "1" || attr(se.Attr, "ISACTUAL") != "1" {
			continue
		}
		id, ok := parseInt64(attr(se.Attr, "OBJECTID"))
		if !ok {
			continue
		}
		out[id] = struct{}{}
	}
	return out, nil
}

func parseHierarchy(
	z *remoteZip,
	entryName string,
	streets map[int64]streetMeta,
	regions map[int64]regionMeta,
	activeHouseIDs map[int64]struct{},
) (map[int64]int64, map[int64]int64, error) {
	rc, err := z.openEntry(entryName)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()

	streetIDs := make(map[int64]struct{}, len(streets))
	for id := range streets {
		streetIDs[id] = struct{}{}
	}
	regionIDs := make(map[int64]struct{}, len(regions))
	for id := range regions {
		regionIDs[id] = struct{}{}
	}

	streetToRegion := make(map[int64]int64, len(streets))
	houseToStreet := make(map[int64]int64, len(activeHouseIDs))

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "ITEM" {
			continue
		}
		item := hierarchyItem{
			ObjectID: attr(se.Attr, "OBJECTID"),
			IsActive: attr(se.Attr, "ISACTIVE"),
			Path:     attr(se.Attr, "PATH"),
		}
		if item.IsActive != "1" {
			continue
		}
		objID, ok := parseInt64(item.ObjectID)
		if !ok {
			continue
		}
		pathIDs := parsePathIDs(item.Path)
		if len(pathIDs) == 0 {
			continue
		}

		if _, ok := streetIDs[objID]; ok {
			if rid := lastIDInSet(pathIDs, regionIDs); rid != 0 {
				streetToRegion[objID] = rid
			}
			continue
		}
		if _, ok := activeHouseIDs[objID]; ok {
			if sid := lastIDInSet(pathIDs, streetIDs); sid != 0 {
				houseToStreet[objID] = sid
			}
		}
	}
	return streetToRegion, houseToStreet, nil
}

func collectStreetHouses(z *remoteZip, entryName string, houseToStreet map[int64]int64) (map[int64]map[string]struct{}, error) {
	rc, err := z.openEntry(entryName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	out := make(map[int64]map[string]struct{}, 8192)
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "HOUSE" {
			continue
		}
		house := houseObject{
			ObjectID: attr(se.Attr, "OBJECTID"),
			HouseNum: attr(se.Attr, "HOUSENUM"),
			AddNum1:  attr(se.Attr, "ADDNUM1"),
			AddNum2:  attr(se.Attr, "ADDNUM2"),
			IsActual: attr(se.Attr, "ISACTUAL"),
			IsActive: attr(se.Attr, "ISACTIVE"),
		}
		if house.IsActive != "1" || house.IsActual != "1" {
			continue
		}
		oid, ok := parseInt64(house.ObjectID)
		if !ok {
			continue
		}
		sid, ok := houseToStreet[oid]
		if !ok || sid == 0 {
			continue
		}
		m := out[sid]
		if m == nil {
			m = make(map[string]struct{}, 16)
			out[sid] = m
		}
		for _, h := range normalizeHouseVariants(house.HouseNum, house.AddNum1, house.AddNum2) {
			m[h] = struct{}{}
		}
	}
	return out, nil
}

func normalizeHouseVariants(houseNum, add1, add2 string) []string {
	base := normalizeHouseToken(houseNum)
	if base == "" {
		return nil
	}
	out := []string{base}
	a1 := normalizeHouseToken(add1)
	if a1 != "" && isLetterSuffix(a1) {
		out = append(out, base+a1)
	}
	a2 := normalizeHouseToken(add2)
	if a1 != "" && a2 != "" && isLetterSuffix(a2) {
		out = append(out, base+a1+a2)
	}
	return out
}

func normalizeHouseToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

func isLetterSuffix(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'а' && r <= 'я') || r == 'ё' {
			continue
		}
		return false
	}
	return true
}

func parsePathIDs(path string) []int64 {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil || v == 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}

func lastIDInSet(ids []int64, target map[int64]struct{}) int64 {
	for i := len(ids) - 1; i >= 0; i-- {
		if _, ok := target[ids[i]]; ok {
			return ids[i]
		}
	}
	return 0
}

func parseInt64(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func isMoscowAdministrativeRegion(level int, name, typ string) bool {
	if level != 3 {
		return false
	}
	n := strings.ToLower(strings.TrimSpace(name))
	t := normalizeType(typ)
	if strings.HasPrefix(n, "муниципальный округ ") {
		return true
	}
	switch t {
	case "рн", "район",
		"окр", "округ",
		"ао", "административныйокруг",
		"внтерг", "внутригородскаятерритория":
		return true
	default:
		return false
	}
}

func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := []string{".", " ", "-", "_"}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r, "")
	}
	return s
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func attr(attrs []xml.Attr, key string) string {
	for _, a := range attrs {
		if a.Name.Local == key {
			return a.Value
		}
	}
	return ""
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return "/"
	}
	return path[:i]
}

func bytesLastIndex(b []byte, sub []byte) int {
	if len(sub) == 0 || len(b) < len(sub) {
		return -1
	}
	for i := len(b) - len(sub); i >= 0; i-- {
		if string(b[i:i+len(sub)]) == string(sub) {
			return i
		}
	}
	return -1
}

func u16le(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

func u32le(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func u64le(b []byte) uint64 {
	return uint64(b[0]) |
		uint64(b[1])<<8 |
		uint64(b[2])<<16 |
		uint64(b[3])<<24 |
		uint64(b[4])<<32 |
		uint64(b[5])<<40 |
		uint64(b[6])<<48 |
		uint64(b[7])<<56
}
