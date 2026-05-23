package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	nominatimURL          = "https://nominatim.openstreetmap.org/search"
	defaultRequestsPerSec = 1.0
)

type Coordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type cacheEntry struct {
	Found bool        `json:"found"`
	Coord Coordinates `json:"coord"`
}

type Geocoder interface {
	GeocodeAddress(ctx context.Context, address string) (Coordinates, bool, error)
	RatePerSecond() float64
}

type NominatimClient struct {
	httpClient *http.Client
	userAgent  string
	email      string
	rate       float64

	mu        sync.RWMutex
	cachePath string
	cache     map[string]cacheEntry
	limiter   <-chan time.Time
}

func NewNominatimClient(cachePath, userAgent, email string) (*NominatimClient, error) {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "sledopyt-addresses/1.0 (https://github.com/georgri/sledopyt-addresses)"
	}
	c := &NominatimClient{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		userAgent:  userAgent,
		email:      strings.TrimSpace(email),
		rate:       defaultRequestsPerSec,
		cachePath:  cachePath,
		cache:      map[string]cacheEntry{},
		limiter:    time.Tick(time.Second),
	}
	if err := c.loadCache(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *NominatimClient) RatePerSecond() float64 {
	return c.rate
}

func (c *NominatimClient) GeocodeAddress(ctx context.Context, address string) (Coordinates, bool, error) {
	key := normalizeKey(address)
	if key == "" {
		return Coordinates{}, false, nil
	}

	if coord, ok, found := c.getFromCache(key); found {
		return coord, ok, nil
	}

	select {
	case <-ctx.Done():
		return Coordinates{}, false, ctx.Err()
	case <-c.limiter:
	}

	reqURL, err := c.makeURL(address)
	if err != nil {
		return Coordinates{}, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Coordinates{}, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Coordinates{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Coordinates{}, false, fmt.Errorf("nominatim status: %s", resp.Status)
	}

	var rows []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return Coordinates{}, false, fmt.Errorf("decode nominatim response: %w", err)
	}
	if len(rows) == 0 {
		c.setCacheEntry(key, cacheEntry{Found: false})
		return Coordinates{}, false, nil
	}

	lat, err := strconv.ParseFloat(rows[0].Lat, 64)
	if err != nil {
		return Coordinates{}, false, fmt.Errorf("parse lat: %w", err)
	}
	lon, err := strconv.ParseFloat(rows[0].Lon, 64)
	if err != nil {
		return Coordinates{}, false, fmt.Errorf("parse lon: %w", err)
	}
	coord := Coordinates{Lat: lat, Lon: lon}
	c.setCacheEntry(key, cacheEntry{Found: true, Coord: coord})
	return coord, true, nil
}

func (c *NominatimClient) makeURL(address string) (string, error) {
	q := url.Values{}
	q.Set("q", address)
	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("addressdetails", "0")
	if c.email != "" {
		q.Set("email", c.email)
	}
	u, err := url.Parse(nominatimURL)
	if err != nil {
		return "", err
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *NominatimClient) getFromCache(key string) (Coordinates, bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.cache[key]
	if !ok {
		return Coordinates{}, false, false
	}
	return e.Coord, e.Found, true
}

func (c *NominatimClient) setCacheEntry(key string, e cacheEntry) {
	c.mu.Lock()
	c.cache[key] = e
	_ = c.saveLocked()
	c.mu.Unlock()
}

func (c *NominatimClient) loadCache() error {
	if strings.TrimSpace(c.cachePath) == "" {
		return nil
	}
	b, err := os.ReadFile(c.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read geocode cache: %w", err)
	}
	var parsed map[string]cacheEntry
	if err := json.Unmarshal(b, &parsed); err != nil {
		return fmt.Errorf("parse geocode cache: %w", err)
	}
	c.cache = parsed
	return nil
}

func (c *NominatimClient) saveLocked() error {
	if strings.TrimSpace(c.cachePath) == "" {
		return nil
	}
	if err := os.MkdirAll(dirOf(c.cachePath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.cachePath, b, 0o644)
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
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

func HaversineKm(a, b Coordinates) float64 {
	const earthRadiusKm = 6371.0
	lat1 := a.Lat * math.Pi / 180.0
	lat2 := b.Lat * math.Pi / 180.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180.0
	dLon := (b.Lon - a.Lon) * math.Pi / 180.0

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(h))
}
