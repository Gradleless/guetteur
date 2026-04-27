package scheduler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/gradleless/guetteur/internal/anilist"
	"github.com/gradleless/guetteur/internal/config"
	dbgen "github.com/gradleless/guetteur/internal/db/generated"
	"github.com/gradleless/guetteur/internal/events"
	appi18n "github.com/gradleless/guetteur/internal/i18n"
	"github.com/gradleless/guetteur/internal/notifier"
	"github.com/gradleless/guetteur/internal/nyaa"
	torrentclient "github.com/gradleless/guetteur/internal/torrent"
	"github.com/gradleless/guetteur/internal/tpl"
)

type Scheduler struct {
	cron     *cron.Cron
	q        *dbgen.Queries
	anilist  *anilist.Client
	cfg      *config.Config
	tc       *torrentclient.Client
	notifier *notifier.Notifier
	bus      *events.Bus
	ctx      context.Context
	loc      *appi18n.Localizer

	// downloadMu serialises the concurrency-limit check + slot reservation so
	// two goroutines can't both see "0 active" and both start a download.
	downloadMu sync.Mutex
}

func New(cfg *config.Config, q *dbgen.Queries, al *anilist.Client, tc *torrentclient.Client, n *notifier.Notifier, bus *events.Bus) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		q:        q,
		anilist:  al,
		cfg:      cfg,
		tc:       tc,
		notifier: n,
		bus:      bus,
		loc:      appi18n.New(cfg.Locale),
	}
}

func (s *Scheduler) ImportMedia(ctx context.Context, m anilist.Media) error {
	return s.upsertMedia(ctx, m)
}

func (s *Scheduler) TriggerTmdbLookup(seriesID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		series, err := s.q.GetSeriesByID(ctx, seriesID)
		if err != nil {
			slog.Warn("tmdb trigger: series not found", "series_id", seriesID, "err", err)
			return
		}
		if series.TmdbID.Valid && series.TmdbID.Int64 != 0 {
			slog.Debug("tmdb trigger: already set, skipping", "series_id", seriesID, "tmdb_id", series.TmdbID.Int64)
			return
		}
		result := lookupTmdbID(ctx, seriesID)
		if !result.Valid {
			return
		}
		if err := s.q.UpdateSeriesTmdbID(ctx, dbgen.UpdateSeriesTmdbIDParams{
			TmdbID: result,
			ID:     seriesID,
		}); err != nil {
			slog.Error("tmdb trigger: update series", "series_id", seriesID, "err", err)
		}
	}()
}

func (s *Scheduler) TriggerPollForSeries(seriesID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		series, err := s.q.GetSeriesByID(ctx, seriesID)
		if err != nil {
			slog.Warn("trigger poll: series not found", "series_id", seriesID, "err", err)
			return
		}

		if !series.TmdbID.Valid || series.TmdbID.Int64 == 0 {
			if result := lookupTmdbID(ctx, seriesID); result.Valid {
				if err := s.q.UpdateSeriesTmdbID(ctx, dbgen.UpdateSeriesTmdbIDParams{
					TmdbID: result,
					ID:     seriesID,
				}); err != nil {
					slog.Error("trigger poll: update tmdb_id", "series_id", seriesID, "err", err)
				}
				series.TmdbID = result
			}
		}
		slog.Info("trigger poll: following series immediately", "series_id", seriesID, "title", series.TitleRomaji)
		s.pollSeriesNyaa(ctx, series)
	}()
}

func (s *Scheduler) Start(ctx context.Context) error {
	anilistSpec := fmt.Sprintf("@every %s", s.cfg.AnilistRefresh.Duration)
	if _, err := s.cron.AddFunc(anilistSpec, func() {
		s.refreshAniList(context.Background())
	}); err != nil {
		return fmt.Errorf("register anilist refresh job: %w", err)
	}

	nyaaSpec := fmt.Sprintf("@every %s", s.cfg.PollInterval.Duration)
	if _, err := s.cron.AddFunc(nyaaSpec, func() {
		s.pollNyaa(context.Background())
	}); err != nil {
		return fmt.Errorf("register nyaa poll job: %w", err)
	}

	if _, err := s.cron.AddFunc("@every 5s", func() {
		s.flushProgressToDB(context.Background())
	}); err != nil {
		return fmt.Errorf("register progress flush job: %w", err)
	}

	if _, err := s.cron.AddFunc("@every 30s", func() {
		s.refreshVpnIP(context.Background())
	}); err != nil {
		return fmt.Errorf("register vpn ip job: %w", err)
	}

	s.ctx = ctx
	s.cron.Start()
	slog.Info("scheduler started",
		"anilist_refresh", s.cfg.AnilistRefresh.Duration,
		"poll_interval", s.cfg.PollInterval.Duration,
	)

	go s.refreshAniList(ctx)
	go s.resumeDownloading(ctx)
	go s.refreshVpnIP(ctx)

	go func() {
		<-ctx.Done()
		s.cron.Stop()
		slog.Info("scheduler stopped")
	}()

	return nil
}

func (s *Scheduler) refreshAniList(ctx context.Context) {
	now := time.Now()
	for offset := 0; offset <= 1; offset++ {
		t := now.AddDate(0, offset*3, 0)
		season, year := anilist.SeasonOf(t)
		slog.Info("anilist refresh", "season", season, "year", year)

		medias, err := s.anilist.SeasonAll(ctx, season, year)
		if err != nil {
			slog.Error("anilist season fetch", "season", season, "year", year, "err", err)
			continue
		}

		for _, m := range medias {
			if err := s.upsertMedia(ctx, m); err != nil {
				slog.Error("upsert media", "series_id", m.ID, "err", err)
			}
		}
		slog.Info("anilist refresh done", "season", season, "year", year, "count", len(medias))
	}
}

func (s *Scheduler) upsertMedia(ctx context.Context, m anilist.Media) error {
	aliasesJSON, _ := json.Marshal(m.Synonyms)
	genresJSON, _ := json.Marshal(m.Genres)

	type charEntry struct {
		Name string `json:"name"`
		VA   string `json:"va"`
	}
	chars := make([]charEntry, 0, len(m.Characters.Edges))
	for _, n := range m.Characters.Edges {
		va := ""
		if len(n.VoiceActors) > 0 {
			va = n.VoiceActors[0].Name.Full
		}
		chars = append(chars, charEntry{Name: n.Node.Name.Full, VA: va})
	}
	charsJSON, _ := json.Marshal(chars)

	type relEntry struct {
		Type  string `json:"type"`
		Title string `json:"title"`
		ID    int64  `json:"id"`
	}
	rels := make([]relEntry, 0, len(m.Relations.Edges))
	for _, e := range m.Relations.Edges {
		t := e.Node.Title.Romaji
		if e.Node.Title.English != "" {
			t = e.Node.Title.English
		}
		rels = append(rels, relEntry{Type: e.RelationType, Title: t, ID: e.Node.ID})
	}
	relsJSON, _ := json.Marshal(rels)

	studio := ""
	if len(m.Studios.Nodes) > 0 {
		studio = m.Studios.Nodes[0].Name
	}

	params := dbgen.UpsertSeriesParams{
		ID:                  m.ID,
		TitleRomaji:         m.Title.Romaji,
		TitleEnglish:        sql.NullString{String: m.Title.English, Valid: m.Title.English != ""},
		TitleNative:         sql.NullString{String: m.Title.Native, Valid: m.Title.Native != ""},
		AliasesJson:         string(aliasesJSON),
		SeasonNumber:        sql.NullInt64{Int64: 1, Valid: true},
		Status:              m.Status,
		Source:              sql.NullString{String: m.Source, Valid: m.Source != ""},
		PreferredGroupsJson: "[]",
		CoverUrl:            sql.NullString{String: m.CoverImage.Large, Valid: m.CoverImage.Large != ""},
		AnilistUrl:          sql.NullString{String: m.SiteURL, Valid: m.SiteURL != ""},
		Description:         sql.NullString{String: m.Description, Valid: m.Description != ""},
		Studio:              sql.NullString{String: studio, Valid: studio != ""},
		GenresJson:          string(genresJSON),
		CharactersJson:      string(charsJSON),
		RelationsJson:       string(relsJSON),
		UpdatedAt:           time.Now().UTC(),
	}
	if m.Episodes != nil {
		params.TotalEpisodes = sql.NullInt64{Int64: int64(*m.Episodes), Valid: true}
	}
	if m.MeanScore != nil {
		params.ScoreAnilist = sql.NullFloat64{Float64: float64(*m.MeanScore) / 10.0, Valid: true}
	}
	if m.Season != nil {
		params.SeasonName = sql.NullString{String: *m.Season, Valid: true}
	}
	if m.SeasonYear != nil {
		params.SeasonYear = sql.NullInt64{Int64: int64(*m.SeasonYear), Valid: true}
	}

	if err := s.q.UpsertSeries(ctx, params); err != nil {
		return fmt.Errorf("upsert series %d: %w", m.ID, err)
	}

	if m.Status == "RELEASING" || m.Status == "NOT_YET_RELEASED" {
		if err := s.q.DeleteAiringScheduleForSeries(ctx, m.ID); err != nil {
			return fmt.Errorf("delete schedule %d: %w", m.ID, err)
		}
		for _, node := range m.AiringSchedule.Nodes {
			p := dbgen.UpsertAiringScheduleParams{
				SeriesID: m.ID,
				Episode:  int64(node.Episode),
				AiringAt: time.Unix(node.AiringAt, 0).UTC(),
			}
			if err := s.q.UpsertAiringSchedule(ctx, p); err != nil {
				slog.Error("upsert airing node", "series_id", m.ID, "episode", node.Episode, "err", err)
			}
		}
	}

	return nil
}

func (s *Scheduler) pollNyaa(ctx context.Context) {
	slog.Info("nyaa poll started")

	followed, err := s.q.ListFollowedSeries(ctx)
	if err != nil {
		slog.Error("nyaa poll: list followed series", "err", err)
		return
	}

	for _, series := range followed {
		s.pollSeriesNyaa(ctx, series)
	}

	slog.Info("nyaa poll done", "series_count", len(followed))
}

func (s *Scheduler) pollSeriesNyaa(ctx context.Context, series dbgen.Series) {
	seriesTitles := []string{series.TitleRomaji}
	if series.TitleEnglish.Valid && series.TitleEnglish.String != "" {
		seriesTitles = append(seriesTitles, series.TitleEnglish.String)
	}
	if series.TitleNative.Valid && series.TitleNative.String != "" {
		seriesTitles = append(seriesTitles, series.TitleNative.String)
	}
	var aliases []string
	if err := json.Unmarshal([]byte(series.AliasesJson), &aliases); err == nil {
		seriesTitles = append(seriesTitles, aliases...)
	}

	var preferredGroups []string
	if err := json.Unmarshal([]byte(series.PreferredGroupsJson), &preferredGroups); err != nil {
		preferredGroups = nil
	}

	existing, err := s.q.ListReleasesBySeriesID(ctx, sql.NullInt64{Int64: series.ID, Valid: true})
	if err != nil {
		slog.Error("nyaa poll: list releases", "series_id", series.ID, "err", err)
		return
	}
	existingHashes := make(map[string]string, len(existing))
	existingEpisodes := make(map[int]string, len(existing))
	for _, r := range existing {
		existingHashes[r.InfoHash] = r.Status
		if r.Episode.Valid {
			ep := int(r.Episode.Int64)
			switch r.Status {
			case "queued", "downloading", "completed":
				existingEpisodes[ep] = r.Status
			}
		}
	}

	searchGroups := preferredGroups
	if len(searchGroups) == 0 {
		searchGroups = s.cfg.DefaultGroups
	}

	var langExtra []string
	if len(s.cfg.PreferredLanguage) > 0 {
		langExtra = []string{s.cfg.PreferredLanguage[0]}
	}

	searchTitles := []string{series.TitleRomaji}
	if series.TitleEnglish.Valid && series.TitleEnglish.String != "" {
		searchTitles = []string{series.TitleEnglish.String, series.TitleRomaji}
	}

qualityLoop:
	for _, quality := range s.cfg.QualityPriority {
		for _, group := range searchGroups {
			for _, searchTitle := range searchTitles {
				rssURL := nyaa.BuildRSSURL(searchTitle, quality, group, langExtra...)
				releases, err := nyaa.Fetch(ctx, rssURL)
				if err != nil {
					slog.Error("nyaa fetch", "series_id", series.ID, "quality", quality, "group", group, "err", err)
					continue
				}
				slog.Info("nyaa fetch done", "series_id", series.ID, "search_title", searchTitle, "quality", quality, "group", group, "results", len(releases))

				matched := 0
				for _, rel := range releases {
					result := nyaa.Filter(nyaa.FilterInput{
						Release:            rel,
						SeriesID:           series.ID,
						SeriesTitles:       seriesTitles,
						PreferredGroups:    preferredGroups,
						QualityPriority:    s.cfg.QualityPriority,
						DefaultGroups:      s.cfg.DefaultGroups,
						PreferredLanguage:  s.cfg.PreferredLanguage,
						MinSeeders:         s.cfg.MinSeeders,
						FuzzyThreshold:     s.cfg.Match.FuzzyThreshold,
						ExistingInfoHashes: existingHashes,
						ExistingEpisodes:   existingEpisodes,
					})
					if !result.Match {
						slog.Info("nyaa release skipped", "series_id", series.ID, "title", rel.Title, "reason", result.Reason)
						continue
					}

					if s.cfg.ReplaceWithBatch && result.Parsed.EpisodeEnd != nil && result.Parsed.Episode != nil {
						s.supersedeSingleEpisodes(ctx, series.ID, existing, *result.Parsed.Episode, *result.Parsed.EpisodeEnd)
					}

					if err := s.insertRelease(ctx, series.ID, rel, result); err != nil {
						slog.Error("insert release", "series_id", series.ID, "info_hash", rel.InfoHash, "err", err)
						continue
					}
					// Keep in-memory maps up to date so subsequent releases in the same
					// RSS batch are correctly deduplicated without a round-trip to the DB.
					existingHashes[rel.InfoHash] = "queued"
					if result.Parsed.Episode != nil {
						existingEpisodes[*result.Parsed.Episode] = "queued"
					}
					matched++
					slog.Info("release queued",
						"series_id", series.ID,
						"info_hash", rel.InfoHash,
						"episode", result.Parsed.Episode,
						"group", result.Parsed.Group,
						"resolution", result.Parsed.Resolution,
					)

					if s.cfg.Notifications.OnEpisodeDetected {
						if dbRel, err := s.q.GetReleaseByInfoHash(ctx, rel.InfoHash); err == nil {
							ep := fmtEpisode(dbRel, series)
							res := ""
							if dbRel.Resolution.Valid {
								res = fmt.Sprintf(" [%s]", dbRel.Resolution.String)
							}
							s.notifier.Notify(notifier.Event{
								Title:   seriesTitle(series),
								Message: strings.TrimSpace(ep + res),
							})
						}
					}

					storageDir := s.computeStorageDir(series, result)
					if err := s.startDownload(ctx, rel.InfoHash, rel.Magnet, storageDir, series); err != nil {
						slog.Error("start download", "series_id", series.ID, "info_hash", rel.InfoHash, "err", err)
					}
				}

				if matched > 0 {
					break qualityLoop
				}
			}
		}
	}
}

func (s *Scheduler) computeStorageDir(series dbgen.Series, result nyaa.FilterResult) string {
	season := 1
	if series.SeasonNumber.Valid {
		season = int(series.SeasonNumber.Int64)
	}
	episode := 0
	if result.Parsed.Episode != nil {
		episode = *result.Parsed.Episode
	}
	episodeEnd := 0
	if result.Parsed.EpisodeEnd != nil {
		episodeEnd = *result.Parsed.EpisodeEnd
	}

	data := tpl.Data{
		TitleEnglish: series.TitleEnglish.String,
		TitleRomaji:  series.TitleRomaji,
		Season:       season,
		Episode:      episode,
		EpisodeEnd:   episodeEnd,
		Group:        result.Parsed.Group,
		Resolution:   result.Parsed.Resolution,
		TmdbID:       series.TmdbID.Int64,
	}

	slog.Info("compute storage dir",
		"series_id", series.ID,
		"tmdb_id", data.TmdbID,
		"tmdb_tag_present", data.TmdbID > 0,
	)

	expanded, err := tpl.Expand(s.cfg.MediaFilenameTemplate, data)
	if err != nil {
		slog.Warn("compute storage dir: template expand failed, using fallback",
			"series_id", series.ID, "err", err)
		expanded = series.TitleRomaji
	}

	dir := filepath.Dir(filepath.FromSlash(tpl.SanitizePath(expanded)))
	return filepath.Join(s.cfg.Env.MediaDir, dir)
}

func (s *Scheduler) supersedeSingleEpisodes(ctx context.Context, seriesID int64, releases []dbgen.Release, epStart, epEnd int) {
	for _, r := range releases {
		if !r.Episode.Valid || r.EpisodeEnd.Valid {
			continue
		}
		ep := int(r.Episode.Int64)
		if ep < epStart || ep > epEnd {
			continue
		}
		if r.Status == "superseded" || r.Status == "skipped" {
			continue
		}
		if err := s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "superseded",
			ErrorMsg: sql.NullString{},
			InfoHash: r.InfoHash,
		}); err != nil {
			slog.Error("supersede release", "info_hash", r.InfoHash, "series_id", seriesID, "err", err)
		} else {
			slog.Info("release superseded by batch", "info_hash", r.InfoHash, "episode", ep, "series_id", seriesID)
		}
	}
}

func newStreamToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Scheduler) insertRelease(ctx context.Context, seriesID int64, rel nyaa.RawRelease, result nyaa.FilterResult) error {
	p := dbgen.InsertReleaseParams{
		InfoHash:    rel.InfoHash,
		SeriesID:    sql.NullInt64{Int64: seriesID, Valid: true},
		RawTitle:    rel.Title,
		Magnet:      rel.Magnet,
		NyaaUrl:     sql.NullString{String: rel.NyaaURL, Valid: rel.NyaaURL != ""},
		Seeders:     sql.NullInt64{Int64: int64(rel.Seeders), Valid: true},
		Leechers:    sql.NullInt64{Int64: int64(rel.Leechers), Valid: true},
		DetectedAt:  time.Now().UTC(),
		Status:      "queued",
		GroupName:   sql.NullString{String: result.Parsed.Group, Valid: result.Parsed.Group != ""},
		Resolution:  sql.NullString{String: result.Parsed.Resolution, Valid: result.Parsed.Resolution != ""},
		StreamToken: sql.NullString{String: newStreamToken(), Valid: true},
	}
	if result.Parsed.Episode != nil {
		p.Episode = sql.NullInt64{Int64: int64(*result.Parsed.Episode), Valid: true}
	}
	if result.Parsed.EpisodeEnd != nil {
		p.EpisodeEnd = sql.NullInt64{Int64: int64(*result.Parsed.EpisodeEnd), Valid: true}
	}

	if err := s.q.InsertRelease(ctx, p); err != nil {
		return fmt.Errorf("insert release %s: %w", rel.InfoHash, err)
	}
	s.bus.Publish(events.Event{
		Type: "release_detected",
		Payload: map[string]any{
			"info_hash":  rel.InfoHash,
			"raw_title":  rel.Title,
			"series_id":  seriesID,
			"episode":    result.Parsed.Episode,
			"resolution": result.Parsed.Resolution,
			"group":      result.Parsed.Group,
		},
	})
	return nil
}

func (s *Scheduler) startDownload(ctx context.Context, infoHash, magnet, storageDir string, series dbgen.Series) error {
	if err := s.tc.CheckDiskSpace(); err != nil {
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "failed",
			ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
			InfoHash: infoHash,
		})
		if s.cfg.Notifications.OnError {
			s.notifier.Notify(notifier.Event{
				Title:   s.loc.T("disk_space_error"),
				Message: err.Error(),
			})
		}
		return fmt.Errorf("disk space: %w", err)
	}

	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return fmt.Errorf("create storage dir %s: %w", storageDir, err)
	}

	// Save path now so drainDownloadQueue can find this release if it is deferred.
	_ = s.q.UpdateReleaseDownloadPath(ctx, dbgen.UpdateReleaseDownloadPathParams{
		DownloadPath: sql.NullString{String: storageDir, Valid: true},
		InfoHash:     infoHash,
	})

	// Atomically check the concurrency cap and reserve a slot by marking the
	// release "downloading" — all under downloadMu so two concurrent goroutines
	// can't both see "0 active" and both proceed to launch.
	if max := s.cfg.Torrent.MaxConcurrentDownloads; max > 0 {
		s.downloadMu.Lock()
		downloading, _ := s.q.ListReleasesByStatus(ctx, "downloading")
		if len(downloading) >= max {
			s.downloadMu.Unlock()
			slog.Info("download deferred: concurrency limit", "info_hash", infoHash, "active", len(downloading), "limit", max)
			return nil
		}
		// Claim the slot now, before releasing the lock.
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "downloading",
			ErrorMsg: sql.NullString{},
			InfoHash: infoHash,
		})
		s.downloadMu.Unlock()
	}

	return s.launchDownload(ctx, infoHash, magnet, storageDir, series)
}

// launchDownload adds the torrent to the client, marks it as downloading, sends
// the start notification and begins watching progress. Call only when a
// concurrency slot is available.
func (s *Scheduler) launchDownload(ctx context.Context, infoHash, magnet, storageDir string, series dbgen.Series) error {
	if err := s.tc.Add(ctx, infoHash, magnet, storageDir); err != nil {
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "failed",
			ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
			InfoHash: infoHash,
		})
		return fmt.Errorf("add torrent: %w", err)
	}

	if err := s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		Status:   "downloading",
		ErrorMsg: sql.NullString{},
		InfoHash: infoHash,
	}); err != nil {
		return fmt.Errorf("update status to downloading: %w", err)
	}

	if s.cfg.Notifications.OnDownloadStart {
		rel, _ := s.q.GetReleaseByInfoHash(ctx, infoHash)
		streamLine := ""
		if rel.StreamToken.Valid && rel.StreamToken.String != "" {
			streamLine = s.loc.Tf("stream_url_line", map[string]any{"Token": rel.StreamToken.String})
		}
		s.notifier.Notify(notifier.Event{
			Title:   seriesTitle(series),
			Message: fmtNotifBody(rel, series, s.loc.T("download_start"), streamLine),
		})
	}

	slog.Info("download started", "info_hash", infoHash, "storage_dir", storageDir)
	s.watchDownload(s.ctx, infoHash)
	return nil
}

// drainDownloadQueue starts queued releases up to the configured concurrency
// cap. Called periodically and on download completion.
func (s *Scheduler) drainDownloadQueue(ctx context.Context) {
	max := s.cfg.Torrent.MaxConcurrentDownloads
	if max <= 0 {
		return
	}

	downloading, err := s.q.ListReleasesByStatus(ctx, "downloading")
	if err != nil {
		slog.Error("drain queue: list downloading", "err", err)
		return
	}
	slots := max - len(downloading)
	if slots <= 0 {
		return
	}

	queued, err := s.q.ListReleasesByStatus(ctx, "queued")
	if err != nil {
		slog.Error("drain queue: list queued", "err", err)
		return
	}

	filled := 0
	for _, r := range queued {
		if filled >= slots {
			break
		}
		if !r.DownloadPath.Valid || r.DownloadPath.String == "" {
			continue
		}

		// Reserve slot under mutex before releasing it for the slow tc.Add.
		s.downloadMu.Lock()
		active, _ := s.q.ListReleasesByStatus(ctx, "downloading")
		if len(active) >= max {
			s.downloadMu.Unlock()
			break
		}
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "downloading",
			ErrorMsg: sql.NullString{},
			InfoHash: r.InfoHash,
		})
		s.downloadMu.Unlock()

		var series dbgen.Series
		if r.SeriesID.Valid {
			series, _ = s.q.GetSeriesByID(ctx, r.SeriesID.Int64)
		}
		if err := s.launchDownload(ctx, r.InfoHash, r.Magnet, r.DownloadPath.String, series); err != nil {
			slog.Error("drain queue: launch download", "info_hash", r.InfoHash, "err", err)
			continue
		}
		filled++
	}
	if filled > 0 {
		slog.Info("drain queue: started deferred downloads", "count", filled)
	}
}
func (s *Scheduler) resumeDownloading(ctx context.Context) {
	releases, err := s.q.ListReleasesByStatus(ctx, "downloading")
	if err != nil {
		slog.Error("resume: list downloading releases", "err", err)
		return
	}

	max := s.cfg.Torrent.MaxConcurrentDownloads
	resumed := 0
	for _, r := range releases {
		if max > 0 && resumed >= max {
			// Re-queue excess releases so drainDownloadQueue picks them up.
			_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
				Status:   "queued",
				ErrorMsg: sql.NullString{},
				InfoHash: r.InfoHash,
			})
			slog.Info("resume: demoted to queued (over concurrency cap)", "info_hash", r.InfoHash)
			continue
		}
		storageDir := r.DownloadPath.String
		if storageDir == "" {
			slog.Warn("resume: no storage dir recorded, skipping", "info_hash", r.InfoHash)
			continue
		}
		if err := s.tc.Add(ctx, r.InfoHash, r.Magnet, storageDir); err != nil {
			slog.Error("resume: re-add torrent", "info_hash", r.InfoHash, "err", err)
			continue
		}
		s.watchDownload(s.ctx, r.InfoHash)
		resumed++
	}

	if resumed > 0 {
		slog.Info("resumed in-flight downloads", "count", resumed)
	}
}

func (s *Scheduler) watchDownload(ctx context.Context, infoHash string) {
	s.tc.Watch(ctx, infoHash, func(ih string, p float64, bytesCompleted, speedBps int64, seeders int) {
		s.bus.Publish(events.Event{
			Type: "download_progress",
			Payload: map[string]any{
				"info_hash":       ih,
				"progress":        p,
				"bytes_completed": bytesCompleted,
				"speed_bps":       speedBps,
				"seeders":         seeders,
			},
		})

		if p >= 1.0 {
			bgCtx := context.Background()
			r, err := s.q.GetReleaseByInfoHash(bgCtx, ih)
			if err != nil {
				slog.Error("watch: get release on complete", "info_hash", ih, "err", err)
				return
			}

			_ = s.q.UpdateReleaseProgress(bgCtx, dbgen.UpdateReleaseProgressParams{
				Progress: p,
				InfoHash: ih,
			})
			s.onComplete(bgCtx, r)
		}
	})
}

func (s *Scheduler) flushProgressToDB(ctx context.Context) {
	releases, err := s.q.ListReleasesByStatus(ctx, "downloading")
	if err != nil {
		slog.Error("flush progress: list releases", "err", err)
		return
	}
	for _, r := range releases {
		p := s.tc.Progress(r.InfoHash)
		if p < 0 {
			continue
		}
		if err := s.q.UpdateReleaseProgress(ctx, dbgen.UpdateReleaseProgressParams{
			Progress: p,
			InfoHash: r.InfoHash,
		}); err != nil {
			slog.Error("flush progress", "info_hash", r.InfoHash, "err", err)
		}
	}
	s.drainDownloadQueue(ctx)
}

func (s *Scheduler) onComplete(ctx context.Context, r dbgen.Release) {
	slog.Info("release completed", "info_hash", r.InfoHash)
	s.tc.PruneCompletionDB(r.InfoHash)
	defer s.tc.Drop(r.InfoHash)

	var series dbgen.Series
	if r.SeriesID.Valid {
		var err error
		series, err = s.q.GetSeriesByID(ctx, r.SeriesID.Int64)
		if err != nil {
			slog.Error("on complete: get series", "info_hash", r.InfoHash, "err", err)
		}
	}

	currentPath, ok := s.tc.LargestFilePath(r.InfoHash)
	if !ok {
		slog.Warn("on complete: file path unavailable, marking completed without rename", "info_hash", r.InfoHash)
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "completed",
			ErrorMsg: sql.NullString{},
			InfoHash: r.InfoHash,
		})
		s.bus.Publish(events.Event{
			Type:    "download_status_changed",
			Payload: map[string]any{"info_hash": r.InfoHash, "status": "completed"},
		})
		s.sendCompletionNotification(series, r)
		return
	}

	ext := strings.TrimPrefix(filepath.Ext(currentPath), ".")
	// Guard against .mkv.part or similar: unwrap .part to get the real extension.
	if ext == "part" {
		if inner := strings.TrimPrefix(filepath.Ext(strings.TrimSuffix(currentPath, ".part")), "."); inner != "" {
			ext = inner
		}
	}
	season := 1
	if series.SeasonNumber.Valid {
		season = int(series.SeasonNumber.Int64)
	}
	episode := 0
	if r.Episode.Valid {
		episode = int(r.Episode.Int64)
	}
	episodeEnd := 0
	if r.EpisodeEnd.Valid {
		episodeEnd = int(r.EpisodeEnd.Int64)
	}

	data := tpl.Data{
		TitleEnglish: series.TitleEnglish.String,
		TitleRomaji:  series.TitleRomaji,
		Season:       season,
		Episode:      episode,
		EpisodeEnd:   episodeEnd,
		Group:        r.GroupName.String,
		Resolution:   r.Resolution.String,
		Ext:          ext,
		TmdbID:       series.TmdbID.Int64,
	}

	expanded, err := tpl.Expand(s.cfg.MediaFilenameTemplate, data)
	if err != nil {
		slog.Error("on complete: expand template", "info_hash", r.InfoHash, "err", err)
		_ = s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "completed",
			ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
			InfoHash: r.InfoHash,
		})
		s.sendCompletionNotification(series, r)
		return
	}

	finalName := filepath.Base(filepath.FromSlash(tpl.SanitizePath(expanded)))
	finalPath := filepath.Join(filepath.Dir(currentPath), finalName)

	if currentPath != finalPath {
		if err := os.Rename(currentPath, finalPath); err != nil {
			// If source is gone but destination exists, a previous onComplete
			// call already renamed the file but failed to update the DB.
			// Treat it as success so the media_path is recorded correctly.
			if _, statErr := os.Stat(finalPath); statErr == nil {
				slog.Info("on complete: file already at final path", "path", finalPath)
			} else {
				slog.Error("on complete: rename file", "src", currentPath, "dst", finalPath, "err", err)
				finalPath = currentPath
			}
		} else {
			slog.Info("on complete: renamed file", "src", currentPath, "dst", finalPath)
		}
	}

	if err := s.q.UpdateReleaseMediaPath(ctx, dbgen.UpdateReleaseMediaPathParams{
		MediaPath: sql.NullString{String: finalPath, Valid: true},
		InfoHash:  r.InfoHash,
	}); err != nil {
		slog.Error("on complete: update media path", "info_hash", r.InfoHash, "err", err)
	}

	s.bus.Publish(events.Event{
		Type:    "download_status_changed",
		Payload: map[string]any{"info_hash": r.InfoHash, "status": "completed"},
	})

	s.sendCompletionNotification(series, r)

	if r.SeriesID.Valid {
		s.checkAutoArchive(ctx, series)
	}

	// A slot just freed up — start the next queued download if any.
	s.drainDownloadQueue(ctx)
}

func seriesTitle(series dbgen.Series) string {
	if series.TitleEnglish.Valid && series.TitleEnglish.String != "" {
		return series.TitleEnglish.String
	}
	return series.TitleRomaji
}

func fmtEpisode(r dbgen.Release, series dbgen.Series) string {
	season := int64(1)
	if series.SeasonNumber.Valid {
		season = series.SeasonNumber.Int64
	}
	if r.Episode.Valid {
		return fmt.Sprintf("S%02dE%02d", season, r.Episode.Int64)
	}
	return ""
}

func (s *Scheduler) sendCompletionNotification(series dbgen.Series, r dbgen.Release) {
	if !s.cfg.Notifications.OnDownloadComplete {
		return
	}
	s.notifier.Notify(notifier.Event{
		Title:   seriesTitle(series),
		Message: fmtNotifBody(r, series, s.loc.T("download_complete"), ""),
	})
}

// fmtNotifBody builds a consistent notification body:
//
//	S01E04 · SubsPlease · 1080p · <label>
//	<streamLine>   (empty = omitted)
func fmtNotifBody(r dbgen.Release, series dbgen.Series, label, streamLine string) string {
	ep := fmtEpisode(r, series)
	parts := []string{ep}
	if r.GroupName.Valid && r.GroupName.String != "" {
		parts = append(parts, r.GroupName.String)
	}
	if r.Resolution.Valid && r.Resolution.String != "" {
		parts = append(parts, r.Resolution.String)
	}
	parts = append(parts, label)
	line := strings.Join(parts, " · ")
	if streamLine != "" {
		line += "\n" + streamLine
	}
	return line
}

func lookupTmdbID(ctx context.Context, anilistID int64) sql.NullInt64 {
	url := fmt.Sprintf("https://arm.haglund.dev/api/v2/ids?source=anilist&id=%d", anilistID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return sql.NullInt64{}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Info("tmdb lookup: arm-server request failed", "anilist_id", anilistID, "err", err)
		return sql.NullInt64{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Info("tmdb lookup: arm-server returned non-200", "anilist_id", anilistID, "status", resp.StatusCode)
		return sql.NullInt64{}
	}

	var body struct {
		Themoviedb *int64 `json:"themoviedb"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err != nil {
		slog.Info("tmdb lookup: failed to decode arm-server response", "anilist_id", anilistID, "err", err)
		return sql.NullInt64{}
	}
	if body.Themoviedb == nil || *body.Themoviedb == 0 {
		slog.Info("tmdb lookup: no tmdb mapping found in arm-server", "anilist_id", anilistID)
		return sql.NullInt64{}
	}
	slog.Info("tmdb lookup: resolved", "anilist_id", anilistID, "tmdb_id", *body.Themoviedb)
	return sql.NullInt64{Int64: *body.Themoviedb, Valid: true}
}

func (s *Scheduler) checkAutoArchive(ctx context.Context, series dbgen.Series) {
	if series.Status != "FINISHED" {
		return
	}
	if !series.TotalEpisodes.Valid || series.TotalEpisodes.Int64 <= 0 {
		return
	}
	total := int(series.TotalEpisodes.Int64)

	releases, err := s.q.ListReleasesBySeriesID(ctx, sql.NullInt64{Int64: series.ID, Valid: true})
	if err != nil {
		slog.Error("auto-archive: list releases", "series_id", series.ID, "err", err)
		return
	}

	completed := make(map[int]bool)
	for _, r := range releases {
		if r.Status != "completed" || !r.Episode.Valid {
			continue
		}
		ep := int(r.Episode.Int64)
		end := ep
		if r.EpisodeEnd.Valid {
			end = int(r.EpisodeEnd.Int64)
		}
		for e := ep; e <= end; e++ {
			completed[e] = true
		}
	}

	for ep := 1; ep <= total; ep++ {
		if !completed[ep] {
			return
		}
	}

	if err := s.q.UpdateSeriesFollowState(ctx, dbgen.UpdateSeriesFollowStateParams{
		FollowState: 2,
		ID:          series.ID,
	}); err != nil {
		slog.Error("auto-archive: update follow state", "series_id", series.ID, "err", err)
		return
	}
	slog.Info("series auto-archived", "series_id", series.ID, "title", series.TitleRomaji)
}

func (s *Scheduler) refreshVpnIP(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8000/v1/publicip/ip", nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("vpn ip: gluetun unreachable", "err", err)
		return
	}
	defer resp.Body.Close()

	var body struct {
		PublicIP string `json:"public_ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.PublicIP == "" {
		slog.Debug("vpn ip: unexpected response", "err", err)
		return
	}

	if err := s.q.SetKV(ctx, dbgen.SetKVParams{K: "system.vpn_ip", V: body.PublicIP}); err != nil {
		slog.Error("vpn ip: set kv", "err", err)
	}
}
