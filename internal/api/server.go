package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/gradleless/guetteur/internal/anilist"
	"github.com/gradleless/guetteur/internal/config"
	dbgen "github.com/gradleless/guetteur/internal/db/generated"
	"github.com/gradleless/guetteur/internal/events"
	"github.com/gradleless/guetteur/internal/notifier"
	torrentclient "github.com/gradleless/guetteur/internal/torrent"
)

// SchedulerHooks are the scheduler operations the HTTP layer can trigger.
// Nil fields make the matching endpoints answer 503.
type SchedulerHooks struct {
	ImportMedia             func(ctx context.Context, m anilist.Media) error
	TriggerSeriesPoll       func(seriesID int64)
	DeleteRelease           func(ctx context.Context, infoHash string) error
	RedownloadRelease       func(ctx context.Context, infoHash string) error
	DeleteSeriesReleases    func(ctx context.Context, seriesID int64) error
	RedownloadSeriesReleases func(ctx context.Context, seriesID int64) error
	DeleteSeriesData        func(ctx context.Context, seriesID int64) error
}

type Server struct {
	Router   *chi.Mux
	q        *dbgen.Queries
	cfg      *config.Config
	tc       *torrentclient.Client
	al       *anilist.Client
	notifier *notifier.Notifier
	bus      *events.Bus
	sched    SchedulerHooks
	startAt  time.Time
}

func New(
	cfg *config.Config,
	q *dbgen.Queries,
	tc *torrentclient.Client,
	al *anilist.Client,
	n *notifier.Notifier,
	bus *events.Bus,
	sched SchedulerHooks,
) *Server {
	s := &Server{
		Router:   chi.NewRouter(),
		q:        q,
		cfg:      cfg,
		tc:       tc,
		al:       al,
		notifier: n,
		bus:      bus,
		sched:    sched,
		startAt:  time.Now(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := s.Router
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	streamLim := newIPLimiter()
	r.With(streamLim.middleware).Get("/stream/{token}", s.handleStream)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/events", s.handleEvents)

		r.Get("/series", s.handleListSeries)
		r.Get("/series/{id}", s.handleGetSeries)
		r.Post("/series/{id}/follow", s.handleFollow)
		r.Post("/series/{id}/ignore", s.handleIgnore)
		r.Post("/series/{id}/archive", s.handleArchive)
		r.Patch("/series/{id}", s.handlePatchSeries)
		r.Delete("/series/{id}/releases", s.handleDeleteSeriesReleases)
		r.Post("/series/{id}/redownload", s.handleRedownloadSeries)
		r.Delete("/series/{id}/data", s.handleDeleteSeriesData)

		r.Get("/schedule", s.handleSchedule)

		r.Get("/downloads", s.handleListDownloads)
		r.Delete("/downloads/{infoHash}", s.handleDeleteDownload)
		r.Post("/downloads/{infoHash}/redownload", s.handleRedownloadDownload)

		r.Get("/anilist/search", s.handleAnilistSearch)
		r.Post("/anilist/import", s.handleAnilistImport)

		r.Get("/settings", s.handleGetSettings)
		r.Patch("/settings", s.handlePatchSettings)
	})
}

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{limiters: make(map[string]*rate.Limiter)}
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.limiters[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(10, 20)
	l.limiters[ip] = lim
	return lim
}

func (l *ipLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.get(ip).Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	rel, err := s.q.GetReleaseByStreamToken(r.Context(), sql.NullString{String: token, Valid: true})
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Completed files are on disk — serve them directly so the torrent client
	// can be dropped after completion without breaking streaming.
	if rel.Status == "completed" && rel.MediaPath.Valid && rel.MediaPath.String != "" {
		f, err := os.Open(rel.MediaPath.String)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, rel.MediaPath.String, time.Time{}, f)
		return
	}

	reader, _, name, ok := s.tc.NewReader(rel.InfoHash)
	if !ok {
		http.NotFound(w, r)
		return
	}
	defer reader.Close()

	http.ServeContent(w, r, name, time.Time{}, reader)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {

		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
