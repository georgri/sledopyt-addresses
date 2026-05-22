package bot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"strings"
	"time"

	"github.com/georgri/sledopyt-addresses/internal/formula"
	"github.com/georgri/sledopyt-addresses/internal/kladr"
	"github.com/georgri/sledopyt-addresses/internal/storage"
)

const (
	maxCitySuggestions = 50
	pageSize           = 50
	maxTelegramMessage = 3800
)

type Bot struct {
	tg    *tgClient
	index *kladr.AddressIndex
	store *storage.JSONStore

	sessionMu sync.RWMutex
	sessions  map[int64]*userSession
}

type userSession struct {
	Results []kladr.Match
	Offset  int
}

func (s *userSession) reset() {
	s.Results = nil
	s.Offset = 0
}

func New(token string, index *kladr.AddressIndex, store *storage.JSONStore) *Bot {
	return &Bot{
		tg:    newTGClient(token),
		index: index,
		store: store,
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
			if upd.Message.Chat.ID == 0 || strings.TrimSpace(upd.Message.Text) == "" {
				continue
			}
			if err := b.handleMessage(ctx, upd.Message); err != nil {
				log.Printf("handle message: %v", err)
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg tgMessage) error {
	text := strings.TrimSpace(msg.Text)

	switch {
	case text == "/start":
		return b.onStart(ctx, msg.Chat.ID)
	case text == "/cities":
		return b.onCities(ctx, msg.Chat.ID)
	case strings.HasPrefix(strings.ToLower(text), "/city "):
		return b.onCity(ctx, msg)
	case strings.HasPrefix(strings.ToLower(text), "/cityname "):
		return b.onCityName(ctx, msg)
	case strings.EqualFold(text, "/more"):
		return b.onMore(ctx, msg)
	case text == "/help":
		return b.onHelp(ctx, msg.Chat.ID)
	default:
		if _, ok := b.store.Get(msg.From.ID); !ok {
			return b.onCityNameRaw(ctx, msg.Chat.ID, text)
		}
		return b.onFormula(ctx, msg)
	}
}

func (b *Bot) onStart(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Привет! Я бот Sledopyt Addresses.\n\n1) Найди город: /cityname <часть названия>, например /cityname москва\n2) Выбери город: /city <код>\n3) Отправь формулу дома, например: 1 + 3*x2 - 4*x5 + \"б\"\n\nКоманды: /cities, /cityname <текст>, /city <код>, /more, /help")
}

func (b *Bot) onHelp(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Формула поддерживает:\n- целые числа\n- переменные x1, x2, x3 ...\n- операции + и -\n- умножение коэффициента на xN (например 4*x2)\n- суффикс буквой: + \"б\"\n\nПример: 1 + 3*x2 - 4*x5 + \"б\"\n\nДля выбора города: /cityname <часть названия>")
}

func (b *Bot) onCities(ctx context.Context, chatID int64) error {
	return b.sendText(ctx, chatID, "Городов очень много, поэтому используй поиск:\n/cityname <часть названия>\n\nПример:\n/cityname москва\n\nЯ предложу до 50 вариантов с кодами для /city <код>.")
}

func (b *Bot) onCityName(ctx context.Context, msg tgMessage) error {
	query := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/cityname"))
	if query == "" {
		return b.sendText(ctx, msg.Chat.ID, "Формат: /cityname <часть названия города>")
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
	sb.WriteString("Выбери код командой /city <код>\n\n")
	for _, c := range results {
		sb.WriteString(fmt.Sprintf("%s — %s\n", c.Code11, c.Name))
	}
	return b.sendText(ctx, chatID, sb.String())
}

func (b *Bot) onCity(ctx context.Context, msg tgMessage) error {
	args := strings.Fields(msg.Text)
	if len(args) != 2 {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Формат: /city <код>. Код берется из /cities")
	}
	cityCode := strings.TrimSpace(args[1])
	city := b.index.Cities[cityCode]
	if city == nil {
		return b.sendText(ctx, msg.Chat.ID, "Город с таким кодом не найден. Используй /cityname <название>")
	}
	if err := b.store.Set(msg.From.ID, cityCode); err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Не удалось сохранить выбор города")
	}
	b.clearSession(msg.From.ID)
	return b.sendText(ctx, msg.Chat.ID, fmt.Sprintf("Город выбран: %s (%s).\nПоиск будет идти по городу и всем его дочерним кодам.\nТеперь отправь формулу дома.", city.Name, cityCode))
}

func (b *Bot) onFormula(ctx context.Context, msg tgMessage) error {
	cityCode, ok := b.store.Get(msg.From.ID)
	if !ok {
		return b.sendText(ctx, msg.Chat.ID, "Сначала выбери город: /cityname <название> и /city <код>")
	}
	city := b.index.Cities[cityCode]
	if city == nil {
		return b.sendText(ctx, msg.Chat.ID, "Ранее выбранный город не найден. Выбери снова через /cityname")
	}

	parsed, err := formula.Parse(msg.Text)
	if err != nil {
		return b.sendText(ctx, msg.Chat.ID, "Некорректная формула: "+err.Error())
	}

	if err := b.sendText(ctx, msg.Chat.ID, "Принято. Нормализованная формула: "+parsed.Normalized+"\nИщу совпадения..."); err != nil {
		return err
	}

	results := b.index.Find(cityCode, parsed, 0)
	if len(results) == 0 {
		b.clearSession(msg.From.ID)
		return b.sendText(ctx, msg.Chat.ID, "Совпадений не найдено.")
	}

	b.setResults(msg.From.ID, results)
	return b.sendPage(ctx, msg.Chat.ID, msg.From.ID, parsed.Normalized, true)
}

func (b *Bot) onMore(ctx context.Context, msg tgMessage) error {
	return b.sendPage(ctx, msg.Chat.ID, msg.From.ID, "", false)
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
		sb.WriteString(fmt.Sprintf("- %s, дом %s\n", r.Street, strings.ToUpper(r.House)))
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
