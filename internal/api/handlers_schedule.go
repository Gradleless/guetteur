package api

import (
	"net/http"
	"time"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

type scheduleEntry struct {
	SeriesID    int64   `json:"series_id"`
	Title       string  `json:"title"`
	Episode     int64   `json:"episode"`
	AiringAt    string  `json:"airing_at"`
	CoverURL    *string `json:"cover_url"`
	FollowState int64   `json:"follow_state"`
}

func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	from := time.Now().UTC().Truncate(24 * time.Hour)
	to := from.Add(7 * 24 * time.Hour)

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t.UTC()
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t.UTC()
		}
	}

	rows, err := s.q.ListScheduleInWindow(ctx, dbgen.ListScheduleInWindowParams{
		AiringAt:   from,
		AiringAt_2: to,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	out := make([]scheduleEntry, 0, len(rows))
	for _, row := range rows {
		title := row.TitleRomaji
		if row.TitleEnglish.Valid && row.TitleEnglish.String != "" {
			title = row.TitleEnglish.String
		}
		e := scheduleEntry{
			SeriesID:    row.SeriesID,
			Title:       title,
			Episode:     row.Episode,
			AiringAt:    row.AiringAt.UTC().Format(time.RFC3339),
			FollowState: row.FollowState,
		}
		if row.CoverUrl.Valid {
			e.CoverURL = &row.CoverUrl.String
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
