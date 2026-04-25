package api

import (
	"database/sql"
	"net/http"
	"time"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

type releaseResponse struct {
	InfoHash    string  `json:"info_hash"`
	SeriesID    *int64  `json:"series_id"`
	SeriesTitle *string `json:"series_title"`
	CoverURL    *string `json:"cover_url"`
	RawTitle    string  `json:"raw_title"`
	Episode     *int64  `json:"episode"`
	EpisodeEnd  *int64  `json:"episode_end"`
	GroupName   *string `json:"group_name"`
	Resolution  *string `json:"resolution"`
	NyaaURL     *string `json:"nyaa_url"`
	Seeders     *int64  `json:"seeders"`
	SizeBytes   *int64  `json:"size_bytes"`
	Status      string  `json:"status"`
	Progress    float64 `json:"progress"`
	SpeedBps    int64   `json:"speed_bps"`
	StreamURL   string  `json:"stream_url"`
	DetectedAt  string  `json:"detected_at"`
}

func toReleaseResponse(r dbgen.Release) releaseResponse {
	streamPath := "/stream/" + r.InfoHash
	if r.StreamToken.Valid && r.StreamToken.String != "" {
		streamPath = "/stream/" + r.StreamToken.String
	}
	out := releaseResponse{
		InfoHash:   r.InfoHash,
		RawTitle:   r.RawTitle,
		Status:     r.Status,
		Progress:   r.Progress,
		StreamURL:  streamPath,
		DetectedAt: r.DetectedAt.UTC().Format(time.RFC3339),
	}
	if r.SeriesID.Valid {
		out.SeriesID = &r.SeriesID.Int64
	}
	if r.Episode.Valid {
		out.Episode = &r.Episode.Int64
	}
	if r.EpisodeEnd.Valid {
		out.EpisodeEnd = &r.EpisodeEnd.Int64
	}
	if r.GroupName.Valid {
		out.GroupName = &r.GroupName.String
	}
	if r.Resolution.Valid {
		out.Resolution = &r.Resolution.String
	}
	if r.NyaaUrl.Valid {
		out.NyaaURL = &r.NyaaUrl.String
	}
	if r.Seeders.Valid {
		out.Seeders = &r.Seeders.Int64
	}
	if r.SizeBytes.Valid {
		out.SizeBytes = &r.SizeBytes.Int64
	}
	return out
}

func (s *Server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	statusParam := r.URL.Query().Get("status")
	if statusParam == "" {
		statusParam = "active"
	}

	var statuses []string
	switch statusParam {
	case "active":
		statuses = []string{"queued", "downloading"}
	case "completed":
		statuses = []string{"completed"}
	case "failed":
		statuses = []string{"failed"}
	case "all":
		statuses = []string{"queued", "downloading", "completed", "failed", "skipped", "superseded"}
	default:
		writeError(w, http.StatusBadRequest, "status must be active|completed|failed|all")
		return
	}

	ctx := r.Context()
	var all []dbgen.ListReleasesWithSeriesByStatusRow
	for _, st := range statuses {
		rows, err := s.q.ListReleasesWithSeriesByStatus(ctx, st)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list downloads")
			return
		}
		all = append(all, rows...)
	}

	out := make([]releaseResponse, 0, len(all))
	for _, row := range all {
		rel := dbgen.Release{
			InfoHash:     row.InfoHash,
			SeriesID:     row.SeriesID,
			RawTitle:     row.RawTitle,
			Episode:      row.Episode,
			EpisodeEnd:   row.EpisodeEnd,
			GroupName:    row.GroupName,
			Resolution:   row.Resolution,
			Magnet:       row.Magnet,
			NyaaUrl:      row.NyaaUrl,
			Seeders:      row.Seeders,
			Leechers:     row.Leechers,
			SizeBytes:    row.SizeBytes,
			DetectedAt:   row.DetectedAt,
			Status:       row.Status,
			DownloadPath: row.DownloadPath,
			MediaPath:    row.MediaPath,
			Progress:     row.Progress,
			ErrorMsg:     row.ErrorMsg,
			StreamToken:  row.StreamToken,
		}
		rr := toReleaseResponse(rel)
		if row.Status == "downloading" && s.tc != nil {
			rr.SpeedBps = s.tc.SpeedBps(row.InfoHash)
		}
		if row.TitleEnglish.Valid && row.TitleEnglish.String != "" {
			rr.SeriesTitle = &row.TitleEnglish.String
		} else if row.TitleRomaji.Valid {
			rr.SeriesTitle = &row.TitleRomaji.String
		}
		if row.CoverUrl.Valid {
			rr.CoverURL = &row.CoverUrl.String
		}
		out = append(out, rr)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	panic("TODO: phase 3 — handleDeleteDownload")
}

func (s *Server) handleRetryDownload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	infoHash := r.PathValue("infoHash")
	if infoHash == "" {
		writeError(w, http.StatusBadRequest, "missing infoHash")
		return
	}
	rel, err := s.q.GetReleaseByInfoHash(ctx, infoHash)
	if err != nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	if rel.Status != "failed" {
		writeError(w, http.StatusConflict, "release is not in failed state")
		return
	}
	if err := s.q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		InfoHash: infoHash,
		Status:   "queued",
		ErrorMsg: sql.NullString{},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "retry failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
