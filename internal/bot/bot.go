package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/georgri/sledopyt-addresses/internal/formula"
	"github.com/georgri/sledopyt-addresses/internal/kladr"
	"github.com/georgri/sledopyt-addresses/internal/storage"
)

const (
	maxCitiesShown  = 200
	maxResultsShown = 200
)

type Bot struct {
	tg    *tgClient
	index *kladr.AddressIndex
	store *storage.JSONStore
}

func New(token string, index *kladr.AddressIndex, store *storage.JSONStore) *Bot {
	return &Bot{
		tg:    newTGClient(token),
		index: index,
		store: store,
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
	case text == "/help":
		return b.onHelp(ctx, msg.Chat.ID)
	default:
		return b.onFormula(ctx, msg)
	}
}

func (b *Bot) onStart(ctx context.Context, chatID int64) error {
	return b.tg.sendMessage(ctx, chatID, "Привет! Я бот Sledopyt Addresses.\n\n1) Выбери город командой /cities и /city <код>\n2) Отправь формулу дома, например: 1 + 3*x2 - 4*x5 + \"б\"\n\nКоманды: /cities, /city <код>, /help")
}

func (b *Bot) onHelp(ctx context.Context, chatID int64) error {
	return b.tg.sendMessage(ctx, chatID, "Формула поддерживает:\n- целые числа\n- переменные x1, x2, x3 ...\n- операции + и -\n- умножение коэффициента на xN (например 4*x2)\n- суффикс буквой: + \"б\"\n\nПример: 1 + 3*x2 - 4*x5 + \"б\"")
}

func (b *Bot) onCities(ctx context.Context, chatID int64) error {
	cities := b.index.CityList()
	if len(cities) == 0 {
		return b.tg.sendMessage(ctx, chatID, "Список городов пуст. Проверь загрузку KLADR.")
	}
	var sb strings.Builder
	sb.WriteString("Доступные города (используй /city <код>):\n")
	limit := len(cities)
	if limit > maxCitiesShown {
		limit = maxCitiesShown
	}
	for i := 0; i < limit; i++ {
		sb.WriteString(fmt.Sprintf("%s — %s\n", cities[i].Code11, cities[i].Name))
	}
	if len(cities) > limit {
		sb.WriteString(fmt.Sprintf("... и еще %d городов.\n", len(cities)-limit))
	}
	return b.tg.sendMessage(ctx, chatID, sb.String())
}

func (b *Bot) onCity(ctx context.Context, msg tgMessage) error {
	args := strings.Fields(msg.Text)
	if len(args) != 2 {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Формат: /city <код>. Код берется из /cities")
	}
	cityCode := strings.TrimSpace(args[1])
	city := b.index.Cities[cityCode]
	if city == nil {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Город с таким кодом не найден. Проверь /cities")
	}
	if err := b.store.Set(msg.From.ID, cityCode); err != nil {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Не удалось сохранить выбор города")
	}
	return b.tg.sendMessage(ctx, msg.Chat.ID, fmt.Sprintf("Город выбран: %s (%s).\nТеперь отправь формулу дома.", city.Name, cityCode))
}

func (b *Bot) onFormula(ctx context.Context, msg tgMessage) error {
	cityCode, ok := b.store.Get(msg.From.ID)
	if !ok {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Сначала выбери город: /cities и /city <код>")
	}
	city := b.index.Cities[cityCode]
	if city == nil {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Ранее выбранный город не найден. Выбери снова через /cities")
	}

	parsed, err := formula.Parse(msg.Text)
	if err != nil {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Некорректная формула: "+err.Error())
	}

	if err := b.tg.sendMessage(ctx, msg.Chat.ID, "Принято. Нормализованная формула: "+parsed.Normalized+"\nИщу совпадения..."); err != nil {
		return err
	}

	results := b.index.Find(cityCode, parsed, maxResultsShown+1)
	if len(results) == 0 {
		return b.tg.sendMessage(ctx, msg.Chat.ID, "Совпадений не найдено.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Найдено совпадений: %d", len(results)))
	truncated := false
	if len(results) > maxResultsShown {
		results = results[:maxResultsShown]
		truncated = true
	}
	sb.WriteString("\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- %s, дом %s\n", r.Street, strings.ToUpper(r.House)))
	}
	if truncated {
		sb.WriteString("Показаны первые результаты, уточни формулу для сужения.")
	}
	return b.tg.sendMessage(ctx, msg.Chat.ID, sb.String())
}
