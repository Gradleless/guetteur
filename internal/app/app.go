package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"

	"github.com/gradleless/guetteur/internal/anilist"
	"github.com/gradleless/guetteur/internal/api"
	"github.com/gradleless/guetteur/internal/config"
	"github.com/gradleless/guetteur/internal/events"
	"github.com/gradleless/guetteur/internal/notifier"
	"github.com/gradleless/guetteur/internal/scheduler"
	torrentclient "github.com/gradleless/guetteur/internal/torrent"
	"github.com/gradleless/guetteur/internal/ui"
)

type App struct {
	cfg       *config.Config
	db        *sql.DB
	queries   *dbgen.Queries
	anilist   *anilist.Client
	tc        *torrentclient.Client
	scheduler *scheduler.Scheduler
	server    *api.Server
}

func New(cfg *config.Config, db *sql.DB) (*App, error) {
	q := dbgen.New(db)
	al := anilist.New()

	tc, err := torrentclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create torrent client: %w", err)
	}

	discord, ntfy := cfg.Env.DiscordWebhook, cfg.Env.NtfyTopic
	if kv, err := q.GetKV(context.Background(), "settings.discord_webhook"); err == nil && kv.V != "" {
		discord = kv.V
	}
	if kv, err := q.GetKV(context.Background(), "settings.ntfy_topic"); err == nil && kv.V != "" {
		ntfy = kv.V
	}
	n := notifier.New(discord, ntfy)

	bus := events.New()
	sched := scheduler.New(cfg, q, al, tc, n, bus)
	srv := api.New(cfg, q, tc, al, n, bus, api.SchedulerHooks{
		ImportMedia:              sched.ImportMedia,
		TriggerSeriesPoll:        sched.TriggerPollForSeries,
		DeleteRelease:            sched.DeleteRelease,
		RedownloadRelease:        sched.RedownloadRelease,
		DeleteSeriesReleases:     sched.DeleteSeriesReleases,
		RedownloadSeriesReleases: sched.RedownloadSeriesReleases,
		DeleteSeriesData:         sched.DeleteSeriesData,
	})

	return &App{
		cfg:       cfg,
		db:        db,
		queries:   q,
		anilist:   al,
		tc:        tc,
		scheduler: sched,
		server:    srv,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", a.server.Router)
	mux.Handle("/stream/", a.server.Router)
	mux.Handle("/", ui.Handler())

	addr := ":" + a.cfg.Env.APIPort
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	slog.Info("HTTP server listening", "addr", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := httpServer.Shutdown(shutdownCtx)
		a.tc.Close()
		return shutdownErr
	case err := <-errCh:
		a.tc.Close()
		return err
	}
}
