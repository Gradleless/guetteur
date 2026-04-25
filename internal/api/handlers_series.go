package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

type charJSON struct {
	Name string `json:"name"`
	VA   string `json:"va"`
}

type relJSON struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	ID    int64  `json:"id"`
}

type seriesStatsResp struct {
	DLCount    int64   `json:"dl_count"`
	TotalBytes int64   `json:"total_bytes"`
	FirstDL    *string `json:"first_dl"`
	LastDL     *string `json:"last_dl"`
}

type seriesResponse struct {
	ID            int64           `json:"id"`
	TitleRomaji   string          `json:"title_romaji"`
	TitleEnglish  *string         `json:"title_english"`
	TitleNative   *string         `json:"title_native"`
	Status        string          `json:"status"`
	FollowState   int64           `json:"follow_state"`
	CoverURL      *string         `json:"cover_url"`
	TotalEpisodes *int64          `json:"total_episodes"`
	AnilistURL    *string         `json:"anilist_url"`
	NextAiring    *nextAiringResp `json:"next_airing"`

	Description     *string    `json:"description"`
	Studio          *string    `json:"studio"`
	ScoreAnilist    *float64   `json:"score_anilist"`
	Genres          []string   `json:"genres"`
	Source          *string    `json:"source"`
	SeasonYear      *int64     `json:"season_year"`
	SeasonName      *string    `json:"season_name"`
	SeasonFormatted *string    `json:"season_formatted"`
	Characters      []charJSON `json:"characters"`
	Relations       []relJSON  `json:"relations"`

	PreferredGroups []string `json:"preferred_groups"`
	Aliases         []string `json:"aliases"`

	DownloadedEpisodes *int64 `json:"downloaded_episodes"`
}

type nextAiringResp struct {
	Episode  int64  `json:"episode"`
	AiringAt string `json:"airing_at"`
}

type seriesDetailResponse struct {
	Series         seriesResponse  `json:"series"`
	RecentReleases []any           `json:"recent_releases"`
	AiringSchedule []airingSlot    `json:"airing_schedule"`
	Stats          seriesStatsResp `json:"stats"`
}

type airingSlot struct {
	Episode  int64  `json:"episode"`
	AiringAt string `json:"airing_at"`
}

var seasonFR = map[string]string{
	"WINTER": "Hiver",
	"SPRING": "Printemps",
	"SUMMER": "Été",
	"FALL":   "Automne",
}

func toSeriesResponse(s dbgen.Series, next *dbgen.AiringSchedule) seriesResponse {
	r := seriesResponse{
		ID:              s.ID,
		TitleRomaji:     s.TitleRomaji,
		Status:          s.Status,
		FollowState:     s.FollowState,
		Genres:          []string{},
		Characters:      []charJSON{},
		Relations:       []relJSON{},
		PreferredGroups: []string{},
		Aliases:         []string{},
	}
	if s.TitleEnglish.Valid {
		r.TitleEnglish = &s.TitleEnglish.String
	}
	if s.TitleNative.Valid {
		r.TitleNative = &s.TitleNative.String
	}
	if s.CoverUrl.Valid {
		r.CoverURL = &s.CoverUrl.String
	}
	if s.TotalEpisodes.Valid {
		r.TotalEpisodes = &s.TotalEpisodes.Int64
	}
	if s.AnilistUrl.Valid {
		r.AnilistURL = &s.AnilistUrl.String
	}
	if s.Description.Valid {
		r.Description = &s.Description.String
	}
	if s.Studio.Valid {
		r.Studio = &s.Studio.String
	}
	if s.ScoreAnilist.Valid {
		r.ScoreAnilist = &s.ScoreAnilist.Float64
	}
	if s.Source.Valid {
		r.Source = &s.Source.String
	}
	if s.SeasonYear.Valid {
		r.SeasonYear = &s.SeasonYear.Int64
	}
	if s.SeasonName.Valid {
		r.SeasonName = &s.SeasonName.String
		if s.SeasonYear.Valid {
			fr := seasonFR[s.SeasonName.String]
			if fr == "" {
				fr = s.SeasonName.String
			}
			formatted := fr + " " + strconv.FormatInt(s.SeasonYear.Int64, 10)
			r.SeasonFormatted = &formatted
		}
	}

	_ = json.Unmarshal([]byte(s.GenresJson), &r.Genres)
	_ = json.Unmarshal([]byte(s.CharactersJson), &r.Characters)
	_ = json.Unmarshal([]byte(s.RelationsJson), &r.Relations)
	_ = json.Unmarshal([]byte(s.PreferredGroupsJson), &r.PreferredGroups)
	_ = json.Unmarshal([]byte(s.AliasesJson), &r.Aliases)

	if next != nil {
		r.NextAiring = &nextAiringResp{
			Episode:  next.Episode,
			AiringAt: next.AiringAt.UTC().Format(time.RFC3339),
		}
	}
	return r
}

func (s *Server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var series []dbgen.Series
	var err error

	switch r.URL.Query().Get("follow_state") {
	case "active":
		series, err = s.q.ListSeriesByFollowState(ctx, 1)
	case "ignored":
		series, err = s.q.ListSeriesByFollowState(ctx, 0)
	case "archived":
		series, err = s.q.ListSeriesByFollowState(ctx, 2)
	default:
		series, err = s.q.ListAllSeries(ctx)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	// Batch-fetch next airings and dl counts — 2 queries total instead of 2×N.
	now := time.Now().UTC()
	nextAirings, _ := s.q.ListNextAiringAllSeries(ctx, now)
	nextMap := make(map[int64]dbgen.AiringSchedule, len(nextAirings))
	for _, a := range nextAirings {
		nextMap[a.SeriesID] = a
	}

	dlCounts, _ := s.q.ListSeriesDownloadCounts(ctx)
	dlMap := make(map[int64]int64, len(dlCounts))
	for _, d := range dlCounts {
		if d.SeriesID.Valid {
			dlMap[d.SeriesID.Int64] = d.DlCount
		}
	}

	out := make([]seriesResponse, 0, len(series))
	for _, se := range series {
		var nextPtr *dbgen.AiringSchedule
		if a, ok := nextMap[se.ID]; ok {
			nextPtr = &a
		}
		resp := toSeriesResponse(se, nextPtr)
		if count, ok := dlMap[se.ID]; ok {
			resp.DownloadedEpisodes = &count
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	se, err := s.q.GetSeriesByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "series not found")
		return
	}

	now := time.Now().UTC()
	next, err := s.q.GetNextAiringForSeries(ctx, dbgen.GetNextAiringForSeriesParams{
		SeriesID: id,
		AiringAt: now,
	})
	var nextPtr *dbgen.AiringSchedule
	if err == nil {
		nextPtr = &next
	}

	slots, _ := s.q.ListAiringScheduleForSeries(ctx, id)
	airingOut := make([]airingSlot, 0, len(slots))
	for _, sl := range slots {
		airingOut = append(airingOut, airingSlot{
			Episode:  sl.Episode,
			AiringAt: sl.AiringAt.UTC().Format(time.RFC3339),
		})
	}

	var statsOut seriesStatsResp
	stats, err := s.q.GetSeriesStats(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err == nil {
		statsOut.DLCount = stats.DlCount
		switch v := stats.TotalBytes.(type) {
		case int64:
			statsOut.TotalBytes = v
		case float64:
			statsOut.TotalBytes = int64(v)
		}
		if s, ok := stats.FirstDl.(string); ok && s != "" {
			statsOut.FirstDL = &s
		}
		if s, ok := stats.LastDl.(string); ok && s != "" {
			statsOut.LastDL = &s
		}
	}

	releases, _ := s.q.ListReleasesBySeriesID(ctx, sql.NullInt64{Int64: id, Valid: true})
	recentOut := make([]any, 0, len(releases))
	for _, rel := range releases {
		rr := toReleaseResponse(rel)
		if rel.Status == "downloading" && s.tc != nil {
			rr.SpeedBps = s.tc.SpeedBps(rel.InfoHash)
		}
		recentOut = append(recentOut, rr)
	}

	writeJSON(w, http.StatusOK, seriesDetailResponse{
		Series:         toSeriesResponse(se, nextPtr),
		RecentReleases: recentOut,
		AiringSchedule: airingOut,
		Stats:          statsOut,
	})
}

func (s *Server) setFollowState(w http.ResponseWriter, r *http.Request, state int64) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.q.UpdateSeriesFollowState(ctx, dbgen.UpdateSeriesFollowStateParams{
		ID:          id,
		FollowState: state,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFollow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	s.setFollowState(w, r, 1)

	if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && s.triggerSeriesPoll != nil {
		s.triggerSeriesPoll(id)
	}
}

func (s *Server) handleIgnore(w http.ResponseWriter, r *http.Request)  { s.setFollowState(w, r, 0) }
func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) { s.setFollowState(w, r, 2) }

type patchSeriesBody struct {
	PreferredGroups *[]string `json:"preferred_groups"`
	Aliases         *[]string `json:"aliases"`
	SeasonNumber    *int64    `json:"season_number"`
}

func (s *Server) handlePatchSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body patchSeriesBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	se, err := s.q.GetSeriesByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "series not found")
		return
	}

	groupsJSON := se.PreferredGroupsJson
	if body.PreferredGroups != nil {
		b, _ := json.Marshal(*body.PreferredGroups)
		groupsJSON = string(b)
	}
	aliasesJSON := se.AliasesJson
	if body.Aliases != nil {
		b, _ := json.Marshal(*body.Aliases)
		aliasesJSON = string(b)
	}
	seasonNum := se.SeasonNumber
	if body.SeasonNumber != nil {
		seasonNum = sql.NullInt64{Int64: *body.SeasonNumber, Valid: true}
	}

	if err := s.q.UpdateSeriesUserFields(ctx, dbgen.UpdateSeriesUserFieldsParams{
		ID:                  id,
		PreferredGroupsJson: groupsJSON,
		AliasesJson:         aliasesJSON,
		SeasonNumber:        seasonNum,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	updated, _ := s.q.GetSeriesByID(ctx, id)
	writeJSON(w, http.StatusOK, toSeriesResponse(updated, nil))
}
