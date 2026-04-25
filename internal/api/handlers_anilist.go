package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gradleless/guetteur/internal/anilist"
)

func (s *Server) handleAnilistSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q required")
		return
	}
	results, err := s.al.SearchMedia(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleAnilistImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AnilistID int64 `json:"anilist_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AnilistID == 0 {
		writeError(w, http.StatusBadRequest, "anilist_id required")
		return
	}

	m, err := s.al.MediaByID(r.Context(), body.AnilistID)
	if err != nil {
		writeError(w, http.StatusNotFound, "media not found on AniList")
		return
	}

	if err := s.importMedia(r.Context(), *m); err != nil {
		writeError(w, http.StatusInternalServerError, "import failed")
		return
	}

	se, err := s.q.GetSeriesByID(r.Context(), body.AnilistID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch after import failed")
		return
	}
	writeJSON(w, http.StatusOK, toSeriesResponse(se, nil))
}

type importMediaFunc func(ctx context.Context, m anilist.Media) error
