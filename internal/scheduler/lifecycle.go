package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"
	"github.com/gradleless/guetteur/internal/events"
)

// DeleteRelease stops an in-flight download if any, removes the files it wrote
// under the media folder and marks the release "deleted". A deleted episode is
// never re-downloaded by the poll; use RedownloadRelease to bring it back.
func (s *Scheduler) DeleteRelease(ctx context.Context, infoHash string) error {
	r, err := s.q.GetReleaseByInfoHash(ctx, infoHash)
	if err != nil {
		return fmt.Errorf("get release %s: %w", infoHash, err)
	}

	// Collect paths before dropping: FilePaths only works on tracked torrents.
	paths := s.tc.FilePaths(r.InfoHash)
	s.tc.Drop(r.InfoHash)
	s.tc.PruneCompletionDB(r.InfoHash)
	if r.MediaPath.Valid && r.MediaPath.String != "" {
		paths = append(paths, r.MediaPath.String)
	}
	s.removeFiles(paths)

	if err := s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		Status:   "deleted",
		ErrorMsg: sql.NullString{},
		InfoHash: r.InfoHash,
	}); err != nil {
		return fmt.Errorf("mark release deleted: %w", err)
	}

	s.bus.Publish(events.Event{
		Type:    "download_status_changed",
		Payload: map[string]any{"info_hash": r.InfoHash, "status": "deleted"},
	})
	slog.Info("release deleted", "info_hash", r.InfoHash, "files", len(paths))

	// Deleting an active download frees a concurrency slot.
	s.drainDownloadQueue(ctx)
	return nil
}

// RedownloadRelease resets a deleted/failed/completed release to "queued" and
// restarts the download from its stored magnet. The torrent add (metadata
// fetch can take up to 90s) runs in the background; progress is reported over
// the event bus like any other download.
func (s *Scheduler) RedownloadRelease(ctx context.Context, infoHash string) error {
	r, err := s.q.GetReleaseByInfoHash(ctx, infoHash)
	if err != nil {
		return fmt.Errorf("get release %s: %w", infoHash, err)
	}

	var series dbgen.Series
	if r.SeriesID.Valid {
		if series, err = s.q.GetSeriesByID(ctx, r.SeriesID.Int64); err != nil {
			return fmt.Errorf("get series %d: %w", r.SeriesID.Int64, err)
		}
	}

	storageDir := r.DownloadPath.String
	if storageDir == "" {
		storageDir = s.storageDirForRelease(series, r)
	}

	// Clear any stale torrent entry so tc.Add doesn't short-circuit on
	// "already tracked".
	s.tc.Drop(r.InfoHash)

	if err := s.q.UpdateReleaseForRedownload(ctx, r.InfoHash); err != nil {
		return fmt.Errorf("reset release for redownload: %w", err)
	}
	s.bus.Publish(events.Event{
		Type:    "download_status_changed",
		Payload: map[string]any{"info_hash": r.InfoHash, "status": "queued"},
	})
	slog.Info("release requeued for redownload", "info_hash", r.InfoHash, "storage_dir", storageDir)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.startDownload(bgCtx, r.InfoHash, r.Magnet, storageDir, series); err != nil {
			slog.Error("redownload: start download", "info_hash", r.InfoHash, "err", err)
		}
	}()
	return nil
}

// DeleteSeriesReleases deletes every downloaded/active episode of a series
// (files removed, releases marked "deleted") while keeping the follow and the
// release history. Season-level counterpart of DeleteRelease.
func (s *Scheduler) DeleteSeriesReleases(ctx context.Context, seriesID int64) error {
	releases, err := s.q.ListReleasesBySeriesID(ctx, sql.NullInt64{Int64: seriesID, Valid: true})
	if err != nil {
		return fmt.Errorf("list releases for series %d: %w", seriesID, err)
	}
	for _, r := range releases {
		if r.Status == "deleted" {
			continue
		}
		if err := s.DeleteRelease(ctx, r.InfoHash); err != nil {
			slog.Error("delete series releases", "series_id", seriesID, "info_hash", r.InfoHash, "err", err)
		}
	}
	return nil
}

// RedownloadSeriesReleases requeues every deleted/failed/completed release of
// a series. Season-level counterpart of RedownloadRelease.
func (s *Scheduler) RedownloadSeriesReleases(ctx context.Context, seriesID int64) error {
	releases, err := s.q.ListReleasesBySeriesID(ctx, sql.NullInt64{Int64: seriesID, Valid: true})
	if err != nil {
		return fmt.Errorf("list releases for series %d: %w", seriesID, err)
	}
	for _, r := range releases {
		switch r.Status {
		case "deleted", "failed", "completed":
			if err := s.RedownloadRelease(ctx, r.InfoHash); err != nil {
				slog.Error("redownload series releases", "series_id", seriesID, "info_hash", r.InfoHash, "err", err)
			}
		}
	}
	return nil
}

// DeleteSeriesData purges everything guetteur produced for a series: active
// torrents, downloaded files (pruning directories left empty), and the release
// history. The series itself stays in the catalog but is unfollowed.
func (s *Scheduler) DeleteSeriesData(ctx context.Context, seriesID int64) error {
	sid := sql.NullInt64{Int64: seriesID, Valid: true}
	releases, err := s.q.ListReleasesBySeriesID(ctx, sid)
	if err != nil {
		return fmt.Errorf("list releases for series %d: %w", seriesID, err)
	}

	for _, r := range releases {
		paths := s.tc.FilePaths(r.InfoHash)
		s.tc.Drop(r.InfoHash)
		s.tc.PruneCompletionDB(r.InfoHash)
		if r.MediaPath.Valid && r.MediaPath.String != "" {
			paths = append(paths, r.MediaPath.String)
		}
		s.removeFiles(paths)
	}

	if err := s.q.DeleteReleasesBySeriesID(ctx, sid); err != nil {
		return fmt.Errorf("delete releases for series %d: %w", seriesID, err)
	}
	if err := s.q.UpdateSeriesFollowState(ctx, dbgen.UpdateSeriesFollowStateParams{
		FollowState: 0,
		ID:          seriesID,
	}); err != nil {
		return fmt.Errorf("unfollow series %d: %w", seriesID, err)
	}

	s.bus.Publish(events.Event{
		Type:    "series_updated",
		Payload: map[string]any{"series_id": seriesID},
	})
	s.bus.Publish(events.Event{
		Type:    "download_status_changed",
		Payload: map[string]any{"series_id": seriesID, "status": "deleted"},
	})
	slog.Info("series data deleted", "series_id", seriesID, "releases", len(releases))
	return nil
}

// scanMissingFiles marks completed releases whose media file disappeared from
// disk (removed manually, moved by another tool) as "deleted" so the UI shows
// the real state and the poll doesn't assume the episode is still present.
func (s *Scheduler) scanMissingFiles(ctx context.Context) {
	releases, err := s.q.ListReleasesByStatus(ctx, "completed")
	if err != nil {
		slog.Error("scan missing files: list completed", "err", err)
		return
	}

	for _, r := range releases {
		if !r.MediaPath.Valid || r.MediaPath.String == "" {
			continue
		}
		if _, err := os.Stat(r.MediaPath.String); err == nil || !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
			Status:   "deleted",
			ErrorMsg: sql.NullString{String: "file missing from disk", Valid: true},
			InfoHash: r.InfoHash,
		}); err != nil {
			slog.Error("scan missing files: mark deleted", "info_hash", r.InfoHash, "err", err)
			continue
		}
		slog.Info("media file missing, release marked deleted", "info_hash", r.InfoHash, "path", r.MediaPath.String)
		s.bus.Publish(events.Event{
			Type:    "download_status_changed",
			Payload: map[string]any{"info_hash": r.InfoHash, "status": "deleted"},
		})
	}
}

func (s *Scheduler) storageDirForRelease(series dbgen.Series, r dbgen.Release) string {
	episode, episodeEnd := 0, 0
	if r.Episode.Valid {
		episode = int(r.Episode.Int64)
	}
	if r.EpisodeEnd.Valid {
		episodeEnd = int(r.EpisodeEnd.Int64)
	}
	return s.storageDirFor(series, episode, episodeEnd, r.GroupName.String, r.Resolution.String)
}

// removeFiles deletes the given paths and prunes any directory left empty,
// stopping at the media root so shared parents (other series, other seasons)
// are never touched.
func (s *Scheduler) removeFiles(paths []string) {
	root := filepath.Clean(s.cfg.Env.MediaDir)
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("delete file", "path", p, "err", err)
			continue
		}
		pruneEmptyDirs(filepath.Dir(p), root)
	}
}

// pruneEmptyDirs removes dir and then its parents as long as they are empty,
// never removing root itself or anything outside it.
func pruneEmptyDirs(dir, root string) {
	dir = filepath.Clean(dir)
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
