package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/georgri/sledopyt-addresses/internal/formula"
	"github.com/georgri/sledopyt-addresses/internal/geo"
	"github.com/georgri/sledopyt-addresses/internal/kladr"
	"github.com/georgri/sledopyt-addresses/internal/storage"
)

const (
	maxCitySuggestions = 50
	pageSize           = 50
	maxTelegramMessage = 3800
	maxMapURLLength    = 3500
)

type Bot struct {
	tg       *tgClient
	index    *kladr.AddressIndex
	store    *storage.JSONStore
	geocoder geo.Geocoder

	sessionMu sync.RWMutex
	sessions  map[int64]*userSession
}

type userSession struct {
	Results        []kladr.Match
	Offset         int
	HasLocation    bool
	LocationLat    float64
	LocationLon    float64
	DistanceSorted bool
}

func (s *userSession) reset() {
	s.Results = nil
	s.Offset = 0
	s.DistanceSorted = false
}

func New(token string, index *kladr.AddressIndex, store *storage.JSONStore, geocoder geo.Geocoder) *Bot {
	return &Bot{
		tg:       newTGClient(token),
		index:    index,
		store:    store,
		geocoder: geocoder,
		sessions: make(map[int64]*userSession),
	}
}

func (b *Bot) Run(ctx context.Context) error {
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, err := b.tg.getUpdates(ctx, offset)
		if err != nil {
			log.Printf("get updates: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			if upd.Message.Chat.ID == 0 {
				continue
			}
			if strings.TrimSpace(upd.Message.Text) == "" && upd.Message.Location == nil {
				continue
			}
			if err := b.handleMessage(ctx, upd.Message); err != nil {
				log.Printf("handle message: %v", err)
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg tgMessage) error {
	if msg.Location != nil {
		return b.onLocation(ctx, msg)
	}

	text := strings.TrimSpace(msg.Text)

	switch {
	case text == "/start":
		return b.onStart(ctx, msg.Chat.ID)
	case text == "/cities":
		return b.onCities(ctx, msg.Chat.ID)
	case strings.HasPrefix(strings.ToLower(text), "/city "):
		return b.onCity(ctx, msg)
	case text == "/citylist":
		return b.onCityList(ctx, msg.Chat.ID, msg.From.ID)
	case text == "/cityclear":
		return b.onCityClear(ctx, msg.Chat.ID, msg.From.ID)
	case strings.HasPrefix(strings.ToLower(text), "/cityname "):
		return b.onCityName(ctx, msg)
	case strings.EqualFold(text, "/more"):
		return b.onMore(ctx, msg)
	case text == "/help":
		return b.onHelp(ctx, msg.Chat.ID)
	case strings.EqualFold(text, "/loc"):
		return b.onLocationRequest(ctx, msg.Chat.ID, 0)
	case strings.EqualFold(text, "/sortdistance"):
		return b.onSortDistance(ctx, msg)
	default:
		if _, ok := b.store.Get(msg.From.ID); !ok {
			return b.onCityNameRaw(ctx, msg.Chat.ID, text)
		}
		return b.onFormula(ctx, msg)
	}
}

func (b *Bot) onStart(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Привет! Я бот Sledopyt Addresses.\n\n1) Найди город: /cityname <часть названия>, например /cityname москва\n2) Добавь 1+ города в поиск: /city <код>\n3) Проверь выбранные: /citylist\n4) Отправь формулу дома, например: 1 + 3*x2 - 4*x5 + \"б\"\n5) Для сортировки по расстоянию: /loc, затем /sortdistance\n\nКоманды: /cities, /cityname <текст>, /city <код>, /citylist, /cityclear, /more, /loc, /sortdistance, /help")
}

func (b *Bot) onHelp(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Формула поддерживает:\n- целые числа\n- переменные x1, x2, x3 ...\n- операции + и -\n- умножение коэффициента на xN (например 4*x2)\n- суффикс буквой: + \"б\"\n\nПример: 1 + 3*x2 - 4*x5 + \"б\"\n\nГорода:\n- поиск: /cityname <подстрока>\n- добавить в выбор: /city <код>\n- список выбора: /citylist\n- очистить выбор: /cityclear\n\nРасстояния:\n- отправить геопозицию: /loc\n- отсортировать текущую выдачу по расстоянию: /sortdistance")
}

func (b *Bot) onCities(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Городов очень много, поэтому используй поиск по подстроке:\n/cityname <часть названия>\n\nПример:\n/cityname москва\n\nЯ предложу до 50 вариантов с кодами для /city <код>.")
}

func (b *Bot) onCityName(ctx context.Context, msg tgMessage) error {
	query := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/cityname"))
	if query == "" {
		return b.sendText(ctx, msg.Chat.ID, "Формат: /cityname <подстрока названия города>")
	}
	return b.onCityNameRaw(ctx, msg.Chat.ID, query)
}

func (b *Bot) onCityNameRaw(ctx context.Context, chatID int64, query string) error {
	results := b.index.FindCitiesByName(query, maxCitySuggestions)
	if len(results) == 0 {
		return b.sendText(ctx, chatID, "Ничего не найдено. Попробуй другой фрагмент названия.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Найдено городов: %d (показаны первые %d)\n", len(results), len(results)))
	sb.WriteString("Отсортировано от более крупных к меньшим.\n")
	sb.WriteString("Выбери код командой /city <код> или нажми кнопку ниже.\n\n")
	commands := make([]string, 0, len(results))
	for _, c := range results {
		sb.WriteString(fmt.Sprintf("%s — %s\n  путь: %s\n", c.Code11, c.Name, c.Path))
		commands = append(commands, "/city "+c.Code11)
	}
	text := sb.String()
	if len(text) > maxTelegramMessage {
		if err := b.sendText(ctx, chatID, text); err != nil {
			return err
		}
		return b.tg.sendMessageWithKeyboard(ctx, chatID, "Кнопки выбора города:", commands)
	}
	return b.tg.sendMessageWithKeyboard(ctx, chatID, text, commands)
}

func (b *Bot) onCity(ctx context.Context, msg tgMessage) error {
	raw := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/city"))
	codes := parseCodes(raw)
	if len(codes) == 0 {
		return b.sendText(ctx, msg.Chat.ID, "Формат: /city <код> [код2 ...]")
	}

	current, _ := b.store.Get(msg.From.ID)
	merged := append([]string(nil), current...)
	added := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(merged))
	for _, c := range merged {
		seen[c] = struct{}{}
	}

	for _, cityCode := range codes {
		city := b.index.Cities[cityCode]
		if city == nil {
			return b.sendText(ctx, msg.Chat.ID, "Город с кодом "+cityCode+" не найден. Используй /cityname <название>")
		}
		if city.Socr != "" && !isAllowedSearchType(city.Socr) {
			return b.sendText(ctx, msg.Chat.ID, "Код "+cityCode+" относится не к городу/подрегиону. Выбери город через /cityname.")
		}
		if _, ok := seen[cityCode]; ok {
			continue
		}
		seen[cityCode] = struct{}{}
		merged = append(merged, cityCode)
		added = append(added, cityCode)
	}

	if len(added) == 0 {
		return b.sendText(ctx, msg.Chat.ID, "Эти города уже выбраны. /citylist покажет текущий список.")
	}

	if err := b.store.Set(msg.From.ID, merged); err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Не удалось сохранить выбор города")
	}
	b.clearSession(msg.From.ID)
	return b.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Добавлено городов: %d.\nТеперь выбрано: %d.\nПоиск будет идти по выбранным городам и их дочерним кодам.\nИспользуй /citylist для просмотра.", len(added), len(merged)))
}

func (b *Bot) onCityList(ctx context.Context, chatID, userID int64) error {
	codes, ok := b.store.Get(userID)
	if !ok || len(codes) == 0 {
		return b.sendText(ctx, chatID, "Список выбранных городов пуст. Используй /cityname и /city <код>.")
	}

	var sb strings.Builder
	sb.WriteString("Выбранные города:\n")
	for _, code := range codes {
		city := b.index.Cities[code]
		if city == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s — %s\n", code, city.Name))
	}
	return b.sendText(ctx, chatID, sb.String())
}

func (b *Bot) onCityClear(ctx context.Context, chatID, userID int64) error {
	if err := b.store.Set(userID, nil); err != nil {
		return b.sendText(ctx, chatID, "Не удалось очистить список городов.")
	}
	b.clearSession(userID)
	return b.sendText(ctx, chatID, "Список выбранных городов очищен.")
}

func (b *Bot) onFormula(ctx context.Context, msg tgMessage) error {
	cityCodes, ok := b.store.Get(msg.From.ID)
	if !ok || len(cityCodes) == 0 {
		return b.sendText(ctx, msg.Chat.ID, "Сначала выбери хотя бы один город: /cityname <название> и /city <код>")
	}

	parsed, err := formula.Parse(msg.Text)
	if err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Некорректная формула: "+err.Error())
	}

	if err := b.sendText(ctx, msg.Chat.ID, "Принято. Нормализованная формула: "+parsed.Normalized+"\nИщу совпадения..."); err != nil {
		return err
	}

	results := b.findAcrossSelectedCities(cityCodes, parsed)
	if len(results) == 0 {
		b.clearSession(msg.From.ID)
		return b.sendText(ctx, msg.Chat.ID, "Совпадений не найдено.")
	}

	b.setResults(msg.From.ID, results)
	if err := b.sendPage(ctx, msg.Chat.ID, msg.From.ID, parsed.Normalized, true); err != nil {
		return err
	}
	return b.onLocationRequest(ctx, msg.Chat.ID, len(results))
}

func (b *Bot) findAcrossSelectedCities(cityCodes []string, parsed formula.Parsed) []kladr.Match {
	merged := make([]kladr.Match, 0, 256)
	seen := make(map[string]struct{}, 2048)
	for _, code := range cityCodes {
		matches := b.index.Find(code, parsed, 0)
		for _, m := range matches {
			key := strings.ToLower(m.City) + "|" + strings.ToLower(m.Street) + "|" + strings.ToLower(m.House)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, m)
		}
	}
	return merged
}

func (b *Bot) onMore(ctx context.Context, msg tgMessage) error {
	return b.sendPage(ctx, msg.Chat.ID, msg.From.ID, "", false)
}

func (b *Bot) onLocation(ctx context.Context, msg tgMessage) error {
	if msg.Location == nil {
		return nil
	}
	b.setLocation(msg.From.ID, msg.Location.Latitude, msg.Location.Longitude)

	results, mapLink, mapPoints, err := b.sortResultsByDistance(ctx, msg.From.ID, msg.Location.Latitude, msg.Location.Longitude)
	if err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Геопозиция сохранена, но отсортировать по расстоянию не удалось: "+err.Error())
	}
	if results == 0 {
		return b.sendText(ctx, msg.Chat.ID, "Геопозиция сохранена. Теперь отправь формулу — результаты будут доступны для сортировки по расстоянию.")
	}
	if err := b.sendText(ctx, msg.Chat.ID, "Готово: результаты автоматически отсортированы по расстоянию."); err != nil {
		return err
	}
	if mapLink != "" {
		if err := b.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Карта ближайших точек (%d):\n%s", mapPoints, mapLink)); err != nil {
			return err
		}
	}
	return b.sendPage(ctx, msg.Chat.ID, msg.From.ID, "", true)
}

func (b *Bot) onLocationRequest(ctx context.Context, chatID int64, matches int) error {
	if b.geocoder == nil {
		return nil
	}
	rate := b.geocoder.RatePerSecond()
	if rate <= 0 {
		rate = 1
	}
	estimate := int(math.Ceil(float64(matches) / rate))
	if estimate < 1 {
		estimate = 1
	}
	text := fmt.Sprintf("Хочешь отсортировать найденные адреса по расстоянию от текущего местоположения?\nОценка времени: около %d сек. (геокодирование через OpenStreetMap)\nНажми кнопку ниже, чтобы отправить геопозицию. После получения геопозиции сортировка начнётся автоматически.", estimate)
	return b.tg.sendLocationRequest(ctx, chatID, text, "📍 Отправить текущую геопозицию")
}

func (b *Bot) onSortDistance(ctx context.Context, msg tgMessage) error {
	if b.geocoder == nil {
		return b.sendText(ctx, msg.Chat.ID, "Сортировка по расстоянию сейчас недоступна.")
	}
	lat, lon, ok := b.getLocation(msg.From.ID)
	if !ok {
		return b.onLocationRequest(ctx, msg.Chat.ID, pageSize)
	}

	results, mapLink, mapPoints, err := b.sortResultsByDistance(ctx, msg.From.ID, lat, lon)
	if err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Не удалось отсортировать по расстоянию: "+err.Error())
	}
	if results == 0 {
		return b.sendText(ctx, msg.Chat.ID, "Нет активного поиска. Сначала отправь формулу.")
	}
	if err := b.sendText(ctx, msg.Chat.ID, "Готово: результаты отсортированы по расстоянию."); err != nil {
		return err
	}
	if mapLink != "" {
		if err := b.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Карта ближайших точек (%d):\n%s", mapPoints, mapLink)); err != nil {
			return err
		}
	}
	return b.sendPage(ctx, msg.Chat.ID, msg.From.ID, "", true)
}

func (b *Bot) sendPage(ctx context.Context, chatID, userID int64, normalized string, reset bool) error {
	page, _, to, pageNo, totalPages, total, err := b.takePage(userID, reset)
	if err != nil {
		return b.sendText(ctx, chatID, "Нет активного поиска. Сначала отправь формулу.")
	}
	if len(page) == 0 {
		return b.sendText(ctx, chatID, "Больше результатов нет.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Найдено совпадений: %d\n", total))
	if normalized != "" {
		sb.WriteString("Нормализованная формула: " + normalized + "\n")
	}
	sb.WriteString(fmt.Sprintf("Страница %d/%d:\n", pageNo, totalPages))
	for _, r := range page {
		if r.HasDistance {
			sb.WriteString(fmt.Sprintf("- %s: %s, дом %s (%.1f км)\n", r.City, r.Street, strings.ToUpper(r.House), r.DistanceKm))
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s, дом %s\n", r.City, r.Street, strings.ToUpper(r.House)))
	}
	if to < total {
		sb.WriteString("\nДля следующей страницы отправь /more")
	}
	return b.sendText(ctx, chatID, sb.String())
}

func (b *Bot) sendText(ctx context.Context, chatID int64, text string) error {
	chunks := splitMessage(text, maxTelegramMessage)
	for _, c := range chunks {
		if err := b.tg.sendMessage(ctx, chatID, c); err != nil {
			return err
		}
	}
	return nil
}

func splitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)/8+1)
	var cur strings.Builder
	for _, line := range lines {
		// +1 for newline separator.
		if cur.Len()+len(line)+1 > limit && cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func (b *Bot) setResults(userID int64, results []kladr.Match) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	s, ok := b.sessions[userID]
	if !ok {
		s = &userSession{}
		b.sessions[userID] = s
	}
	s.Results = results
	s.Offset = 0
	s.DistanceSorted = false
}

func (b *Bot) clearSession(userID int64) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	delete(b.sessions, userID)
}

func (b *Bot) takePage(userID int64, reset bool) (page []kladr.Match, from int, to int, pageNo int, totalPages int, total int, err error) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()

	session := b.sessions[userID]
	if session == nil || len(session.Results) == 0 {
		return nil, 0, 0, 0, 0, 0, fmt.Errorf("no session")
	}
	if reset {
		session.Offset = 0
	}
	if session.Offset >= len(session.Results) {
		return []kladr.Match{}, session.Offset, session.Offset, 0, 0, len(session.Results), nil
	}

	from = session.Offset
	to = from + pageSize
	if to > len(session.Results) {
		to = len(session.Results)
	}
	page = append([]kladr.Match(nil), session.Results[from:to]...)
	session.Offset = to

	total = len(session.Results)
	pageNo = from/pageSize + 1
	totalPages = (total + pageSize - 1) / pageSize
	return page, from, to, pageNo, totalPages, total, nil
}

func (b *Bot) setLocation(userID int64, lat, lon float64) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	s := b.sessions[userID]
	if s == nil {
		s = &userSession{}
		b.sessions[userID] = s
	}
	s.HasLocation = true
	s.LocationLat = lat
	s.LocationLon = lon
}

func (b *Bot) getLocation(userID int64) (float64, float64, bool) {
	b.sessionMu.RLock()
	defer b.sessionMu.RUnlock()
	s := b.sessions[userID]
	if s == nil || !s.HasLocation {
		return 0, 0, false
	}
	return s.LocationLat, s.LocationLon, true
}

func (b *Bot) sortResultsByDistance(ctx context.Context, userID int64, lat, lon float64) (int, string, int, error) {
	b.sessionMu.Lock()
	session := b.sessions[userID]
	if session == nil || len(session.Results) == 0 {
		b.sessionMu.Unlock()
		return 0, "", 0, nil
	}
	results := append([]kladr.Match(nil), session.Results...)
	b.sessionMu.Unlock()

	ref := geo.Coordinates{Lat: lat, Lon: lon}
	for i := range results {
		query := fmt.Sprintf("%s, %s, %s, Россия", results[i].City, results[i].Street, results[i].House)
		coord, ok, err := b.geocoder.GeocodeAddress(ctx, query)
		if err != nil {
			return 0, "", 0, err
		}
		if !ok {
			results[i].HasDistance = false
			results[i].HasCoordinates = false
			continue
		}
		results[i].HasDistance = true
		results[i].DistanceKm = geo.HaversineKm(ref, coord)
		results[i].HasCoordinates = true
		results[i].Lat = coord.Lat
		results[i].Lon = coord.Lon
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].HasDistance != results[j].HasDistance {
			return results[i].HasDistance
		}
		if results[i].HasDistance && results[j].HasDistance && results[i].DistanceKm != results[j].DistanceKm {
			return results[i].DistanceKm < results[j].DistanceKm
		}
		if results[i].City != results[j].City {
			return results[i].City < results[j].City
		}
		if results[i].Street != results[j].Street {
			return results[i].Street < results[j].Street
		}
		return results[i].House < results[j].House
	})

	b.sessionMu.Lock()
	session = b.sessions[userID]
	if session != nil {
		session.Results = results
		session.Offset = 0
		session.DistanceSorted = true
	}
	b.sessionMu.Unlock()

	mapLink, mapPoints := buildNearestMapLink(results)
	return len(results), mapLink, mapPoints, nil
}

func buildNearestMapLink(results []kladr.Match) (string, int) {
	features := make([]feature, 0, pageSize)
	bestFeatures := make([]feature, 0, pageSize)
	for _, r := range results {
		if !r.HasCoordinates {
			continue
		}
		features = append(features, feature{
			Type: "Feature",
			Geometry: featureGeom{
				Type:        "Point",
				Coordinates: []float64{roundCoord(r.Lon), roundCoord(r.Lat)},
			},
			Properties: map[string]any{
				"n": fmt.Sprintf("%s, %s, %s", r.City, r.Street, strings.ToUpper(r.House)),
			},
		})
		link := buildGeoJSONLink(features)
		if link == "" {
			break
		}
		if len(link) > maxMapURLLength {
			break
		}
		bestFeatures = append(bestFeatures[:0], features...)
	}
	if len(bestFeatures) == 0 {
		return "", 0
	}
	return buildGeoJSONLink(bestFeatures), len(bestFeatures)
}

func buildGeoJSONLink(features []feature) string {
	payload, err := json.Marshal(featureCollection{
		Type:     "FeatureCollection",
		Features: features,
	})
	if err != nil {
		return ""
	}
	encoded := url.QueryEscape("data:application/json," + string(payload))
	return "https://geojson.io/#data=" + encoded
}

type featureGeom struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type feature struct {
	Type       string         `json:"type"`
	Geometry   featureGeom    `json:"geometry"`
	Properties map[string]any `json:"properties,omitempty"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

func roundCoord(v float64) float64 {
	return math.Round(v*1e5) / 1e5
}

func parseCodes(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	fields := strings.Fields(raw)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func isAllowedSearchType(socr string) bool {
	s := strings.ToLower(strings.TrimSpace(socr))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	switch s {
	case "г", "город",
		"рн", "район",
		"окр", "округ",
		"ао",
		"мкр", "микрорайон",
		"тер", "территория",
		"внтерг", "внутригородскаятерритория":
		return true
	default:
		return false
	}
}
