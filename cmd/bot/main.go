package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"food-kbju-bot/internal/ai"
	"food-kbju-bot/internal/bot"
	"food-kbju-bot/internal/config"
	"food-kbju-bot/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Error("timezone", "err", err, "tz", cfg.Timezone)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		logger.Error("mkdir data", "err", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DatabasePath, loc)
	if err != nil {
		logger.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx := context.Background()
	aiClient, err := ai.New(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		logger.Error("ai client", "err", err)
		os.Exit(1)
	}

	b, err := bot.New(cfg, aiClient, st, logger)
	if err != nil {
		logger.Error("bot", "err", err)
		os.Exit(1)
	}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-ch
		logger.Info("shutting down", "signal", sig.String())
		b.Stop()
	}()

	b.Start()
}
