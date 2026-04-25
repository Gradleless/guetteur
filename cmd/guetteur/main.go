package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gradleless/guetteur/internal/app"
	"github.com/gradleless/guetteur/internal/config"
	"github.com/gradleless/guetteur/internal/db"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(ctx, cfg.Env.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	a, err := app.New(cfg, database)
	if err != nil {
		slog.Error("init app", "err", err)
		os.Exit(1)
	}

	slog.Info("guetteur ready", "port", cfg.Env.APIPort)

	if err := a.Run(ctx); err != nil {
		slog.Error("run", "err", err)
		os.Exit(1)
	}
}
