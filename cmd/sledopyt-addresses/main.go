package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/georgri/sledopyt-addresses/internal/bot"
	"github.com/georgri/sledopyt-addresses/internal/config"
	"github.com/georgri/sledopyt-addresses/internal/formula"
	"github.com/georgri/sledopyt-addresses/internal/kladr"
	"github.com/georgri/sledopyt-addresses/internal/storage"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("starting sledopyt addresses bot")
	index, err := kladr.Load(cfg.KLADRSourcePath)
	if err != nil {
		log.Fatalf("load kladr: %v", err)
	}
	if cfg.GARDataPath != "" {
		if err := index.MergeGAROverlay(cfg.GARDataPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Printf("GAR overlay is not available yet (%s), continuing with KLADR only", cfg.GARDataPath)
			} else {
				log.Fatalf("load GAR overlay: %v", err)
			}
		} else {
			log.Printf("merged GAR overlay from %s", cfg.GARDataPath)
		}
	}
	log.Printf("loaded %d cities", len(index.Cities))

	ranOneShot, err := runOneShot(index)
	if err != nil {
		log.Fatalf("one-shot: %v", err)
	}
	if ranOneShot {
		return
	}

	stateStore, err := storage.NewJSONStore(cfg.UserStatePath)
	if err != nil {
		log.Fatalf("user state storage: %v", err)
	}

	telegramBot := bot.New(cfg.TelegramToken, index, stateStore)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := telegramBot.Run(ctx); err != nil {
		log.Fatalf("run bot: %v", err)
	}
}

func runOneShot(index *kladr.AddressIndex) (bool, error) {
	cityCode := os.Getenv("ONESHOT_CITY_CODE")
	formulaRaw := os.Getenv("ONESHOT_FORMULA")
	if cityCode == "" || formulaRaw == "" {
		return false, nil
	}

	parsed, err := formula.Parse(formulaRaw)
	if err != nil {
		return false, err
	}

	results := index.Find(cityCode, parsed, 30)
	if v := os.Getenv("ONESHOT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			results = index.Find(cityCode, parsed, n)
		}
	}
	fmt.Printf("one-shot city=%s formula=%s matches=%d\n", cityCode, parsed.Normalized, len(results))
	for _, r := range results {
		fmt.Printf("%s, дом %s\n", r.Street, r.House)
	}
	return true, nil
}
