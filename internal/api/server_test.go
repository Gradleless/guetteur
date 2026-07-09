package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gradleless/guetteur/internal/api"
	"github.com/gradleless/guetteur/internal/config"
	"github.com/gradleless/guetteur/internal/db"
	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

func newTestServer(t *testing.T) (*api.Server, *dbgen.Queries) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	q := dbgen.New(conn)
	srv := api.New(&config.Config{}, q, nil, nil, nil, nil, api.SchedulerHooks{})
	return srv, q
}

func newTestServerWithHooks(t *testing.T, hooks api.SchedulerHooks) (*api.Server, *dbgen.Queries) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	q := dbgen.New(conn)
	srv := api.New(&config.Config{}, q, nil, nil, nil, nil, hooks)
	return srv, q
}

func do(t *testing.T, srv *api.Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	srv.Router.ServeHTTP(rr, req)
	return rr
}

func mustUpsert(t *testing.T, q *dbgen.Queries, id int64, romaji string) {
	t.Helper()
	if err := q.UpsertSeries(context.Background(), dbgen.UpsertSeriesParams{
		ID:                  id,
		TitleRomaji:         romaji,
		AliasesJson:         "[]",
		Status:              "RELEASING",
		PreferredGroupsJson: "[]",
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertSeries: %v", err)
	}
}

func TestHandleHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/health", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("health ok = %v, want true", resp["ok"])
	}
}

func TestHandleListSeries_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/series", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var arr []any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr))
	}
}

func TestHandleListSeries_WithData(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 1, "Frieren")
	mustUpsert(t, q, 2, "Attack on Titan")

	rr := do(t, srv, "GET", "/api/series", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 2 {
		t.Errorf("expected 2 series, got %d", len(arr))
	}
}

func TestHandleListSeries_FollowStateFilter(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 1, "Frieren")
	mustUpsert(t, q, 2, "Bleach")
	q.UpdateSeriesFollowState(context.Background(), dbgen.UpdateSeriesFollowStateParams{ID: 1, FollowState: 1})

	rr := do(t, srv, "GET", "/api/series?follow_state=active", nil)
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("expected 1 active series, got %d", len(arr))
	}
	if len(arr) == 1 && arr[0]["title_romaji"] != "Frieren" {
		t.Errorf("expected Frieren, got %v", arr[0]["title_romaji"])
	}
}

func TestHandleListSeries_NoNextAiring(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 1, "Anime")

	rr := do(t, srv, "GET", "/api/series", nil)
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 series, got %d", len(arr))
	}
	if arr[0]["next_airing"] != nil {
		t.Errorf("next_airing should be null when no schedule, got %v", arr[0]["next_airing"])
	}
}

func TestHandleGetSeries_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/series/9999", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandleGetSeries_BadID(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/series/notanumber", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandleGetSeries_Found(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 5, "Jujutsu Kaisen")

	rr := do(t, srv, "GET", "/api/series/5", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	seriesMap, ok := resp["series"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'series' key, got: %v", resp)
	}
	if seriesMap["title_romaji"] != "Jujutsu Kaisen" {
		t.Errorf("title_romaji = %v, want Jujutsu Kaisen", seriesMap["title_romaji"])
	}

	if sched, ok := resp["airing_schedule"]; !ok || sched == nil {
		t.Errorf("airing_schedule should be present (empty array), got %v", sched)
	}
}

func TestHandleGetSeries_NoNextAiring(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 7, "Danmachi")

	rr := do(t, srv, "GET", "/api/series/7", nil)
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	seriesMap := resp["series"].(map[string]any)
	if seriesMap["next_airing"] != nil {
		t.Errorf("next_airing should be null when no schedule, got %v", seriesMap["next_airing"])
	}
}

func TestHandleFollow(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 10, "Naruto")

	rr := do(t, srv, "POST", "/api/series/10/follow", nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	got, _ := q.GetSeriesByID(context.Background(), 10)
	if got.FollowState != 1 {
		t.Errorf("FollowState = %d, want 1 after follow", got.FollowState)
	}
}

func TestHandleIgnore(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 11, "Bleach")
	q.UpdateSeriesFollowState(context.Background(), dbgen.UpdateSeriesFollowStateParams{ID: 11, FollowState: 1})

	rr := do(t, srv, "POST", "/api/series/11/ignore", nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	got, _ := q.GetSeriesByID(context.Background(), 11)
	if got.FollowState != 0 {
		t.Errorf("FollowState = %d, want 0 after ignore", got.FollowState)
	}
}

func TestHandleArchive(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 12, "One Piece")

	rr := do(t, srv, "POST", "/api/series/12/archive", nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	got, _ := q.GetSeriesByID(context.Background(), 12)
	if got.FollowState != 2 {
		t.Errorf("FollowState = %d, want 2 after archive", got.FollowState)
	}
}

func TestHandleFollow_BadID(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "POST", "/api/series/abc/follow", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandlePatchSeries_PreferredGroups(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 20, "Chainsaw Man")

	body := map[string]any{"preferred_groups": []string{"SubsPlease", "Erai-raws"}}
	rr := do(t, srv, "PATCH", "/api/series/20", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	got, _ := q.GetSeriesByID(context.Background(), 20)
	if got.PreferredGroupsJson != `["SubsPlease","Erai-raws"]` {
		t.Errorf("PreferredGroupsJson = %q", got.PreferredGroupsJson)
	}
}

func TestHandlePatchSeries_Aliases(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 21, "Sono Bisque Doll")

	body := map[string]any{"aliases": []string{"My Dress-Up Darling"}}
	rr := do(t, srv, "PATCH", "/api/series/21", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	got, _ := q.GetSeriesByID(context.Background(), 21)
	if got.AliasesJson != `["My Dress-Up Darling"]` {
		t.Errorf("AliasesJson = %q", got.AliasesJson)
	}
}

func TestHandlePatchSeries_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	body := map[string]any{"preferred_groups": []string{"SubsPlease"}}
	rr := do(t, srv, "PATCH", "/api/series/9999", body)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandlePatchSeries_BadJSON(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 22, "Test")

	req := httptest.NewRequest("PATCH", "/api/series/22", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid JSON", rr.Code)
	}
}

func TestHandleListDownloads_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/downloads", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var arr []any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %d", len(arr))
	}
}

func TestHandleListDownloads_ActiveDefault(t *testing.T) {
	srv, q := newTestServer(t)
	q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash: "aa", RawTitle: "Test", Magnet: "m",
		DetectedAt: time.Now().UTC(), Status: "downloading",
	})
	q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash: "bb", RawTitle: "Test2", Magnet: "m2",
		DetectedAt: time.Now().UTC(), Status: "completed",
	})

	rr := do(t, srv, "GET", "/api/downloads", nil)
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("expected 1 active download, got %d", len(arr))
	}
	if arr[0]["info_hash"] != "aa" {
		t.Errorf("info_hash = %v, want aa", arr[0]["info_hash"])
	}
}

func TestHandleListDownloads_StreamURL(t *testing.T) {
	srv, q := newTestServer(t)
	q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash: "cc", RawTitle: "Stream Test", Magnet: "m",
		DetectedAt: time.Now().UTC(), Status: "downloading",
		StreamToken: sql.NullString{String: "abc123token", Valid: true},
	})

	rr := do(t, srv, "GET", "/api/downloads", nil)
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 result, got %d", len(arr))
	}
	if arr[0]["stream_url"] != "/stream/abc123token" {
		t.Errorf("stream_url = %v, want /stream/abc123token", arr[0]["stream_url"])
	}
}

func TestHandleListDownloads_StatusCompleted(t *testing.T) {
	srv, q := newTestServer(t)
	q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash: "dd", RawTitle: "Done", Magnet: "m",
		DetectedAt: time.Now().UTC(), Status: "completed",
	})

	rr := do(t, srv, "GET", "/api/downloads?status=completed", nil)
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("expected 1 completed, got %d", len(arr))
	}
}

func TestHandleListDownloads_StatusAll(t *testing.T) {
	srv, q := newTestServer(t)
	for _, status := range []string{"queued", "downloading", "completed", "failed"} {
		q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
			InfoHash:   status + "hash",
			RawTitle:   "T",
			Magnet:     "m",
			DetectedAt: time.Now().UTC(),
			Status:     status,
		})
	}

	rr := do(t, srv, "GET", "/api/downloads?status=all", nil)
	var arr []any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 4 {
		t.Errorf("expected 4 results for status=all, got %d", len(arr))
	}
}

func TestHandleListDownloads_InvalidStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/downloads?status=invalid", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid status param", rr.Code)
	}
}

func TestHandleSchedule_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/schedule", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var arr []any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected empty schedule, got %d items", len(arr))
	}
}

func TestHandleSchedule_WithData(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 99, "My Hero Academia")

	future := time.Now().UTC().Add(48 * time.Hour)
	q.UpsertAiringSchedule(context.Background(), dbgen.UpsertAiringScheduleParams{
		SeriesID: 99, Episode: 5, AiringAt: future,
	})

	from := time.Now().UTC().Format(time.RFC3339)
	to := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	rr := do(t, srv, "GET", "/api/schedule?from="+from+"&to="+to, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var arr []map[string]any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 1 {
		t.Errorf("expected 1 schedule entry, got %d", len(arr))
	}
	if len(arr) == 1 && arr[0]["title"] != "My Hero Academia" {
		t.Errorf("title = %v, want My Hero Academia", arr[0]["title"])
	}
}

func TestHandleSchedule_OutsideWindow(t *testing.T) {
	srv, q := newTestServer(t)
	mustUpsert(t, q, 100, "Distant Anime")

	distant := time.Now().UTC().Add(30 * 24 * time.Hour)
	q.UpsertAiringSchedule(context.Background(), dbgen.UpsertAiringScheduleParams{
		SeriesID: 100, Episode: 1, AiringAt: distant,
	})

	from := time.Now().UTC().Format(time.RFC3339)
	to := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	rr := do(t, srv, "GET", "/api/schedule?from="+from+"&to="+to, nil)
	var arr []any
	json.Unmarshal(rr.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Errorf("expected 0 entries for out-of-window airing, got %d", len(arr))
	}
}

func TestResponseContentType(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/health", nil)
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestErrorResponse_IsJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := do(t, srv, "GET", "/api/series/notanumber", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("error response is not valid JSON: %v (body: %s)", err, rr.Body.String())
	}
	if _, ok := resp["error"]; !ok {
		t.Errorf("error response missing 'error' key: %v", resp)
	}
}

var _ = sql.NullString{}
