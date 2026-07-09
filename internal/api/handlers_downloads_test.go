package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gradleless/guetteur/internal/api"
	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

func mustInsertRelease(t *testing.T, q *dbgen.Queries, infoHash, status string) {
	t.Helper()
	if err := q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash:   infoHash,
		RawTitle:   "[SubsPlease] Test - 01 (1080p).mkv",
		Magnet:     "magnet:?xt=urn:btih:" + infoHash,
		DetectedAt: time.Now().UTC(),
		Status:     status,
	}); err != nil {
		t.Fatalf("InsertRelease: %v", err)
	}
}

func TestDeleteDownload_NotFound(t *testing.T) {
	srv, _ := newTestServerWithHooks(t, api.SchedulerHooks{
		DeleteRelease: func(ctx context.Context, infoHash string) error { return nil },
	})
	rr := do(t, srv, http.MethodDelete, "/api/downloads/deadbeef", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

func TestDeleteDownload_CallsHook(t *testing.T) {
	var got string
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{
		DeleteRelease: func(ctx context.Context, infoHash string) error {
			got = infoHash
			return nil
		},
	})
	mustInsertRelease(t, q, "aabbccdd", "completed")

	rr := do(t, srv, http.MethodDelete, "/api/downloads/aabbccdd", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204 (%s)", rr.Code, rr.Body.String())
	}
	if got != "aabbccdd" {
		t.Errorf("hook called with %q, want aabbccdd", got)
	}
}

func TestDeleteDownload_AlreadyDeleted_NoHookCall(t *testing.T) {
	called := false
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{
		DeleteRelease: func(ctx context.Context, infoHash string) error {
			called = true
			return nil
		},
	})
	mustInsertRelease(t, q, "aabbccdd", "deleted")

	rr := do(t, srv, http.MethodDelete, "/api/downloads/aabbccdd", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", rr.Code)
	}
	if called {
		t.Error("hook must not run for an already-deleted release")
	}
}

func TestRedownload_ActiveRelease_Conflict(t *testing.T) {
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{
		RedownloadRelease: func(ctx context.Context, infoHash string) error { return nil },
	})
	mustInsertRelease(t, q, "aabbccdd", "downloading")

	rr := do(t, srv, http.MethodPost, "/api/downloads/aabbccdd/redownload", nil)
	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want 409", rr.Code)
	}
}

func TestRedownload_DeletedRelease(t *testing.T) {
	var got string
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{
		RedownloadRelease: func(ctx context.Context, infoHash string) error {
			got = infoHash
			return nil
		},
	})
	mustInsertRelease(t, q, "aabbccdd", "deleted")

	rr := do(t, srv, http.MethodPost, "/api/downloads/aabbccdd/redownload", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204 (%s)", rr.Code, rr.Body.String())
	}
	if got != "aabbccdd" {
		t.Errorf("hook called with %q, want aabbccdd", got)
	}
}

func TestDeleteSeriesData(t *testing.T) {
	var got int64
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{
		DeleteSeriesData: func(ctx context.Context, seriesID int64) error {
			got = seriesID
			return nil
		},
	})
	mustUpsert(t, q, 42, "Frieren")

	rr := do(t, srv, http.MethodDelete, "/api/series/42/data", nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204 (%s)", rr.Code, rr.Body.String())
	}
	if got != 42 {
		t.Errorf("hook called with %d, want 42", got)
	}

	rr = do(t, srv, http.MethodDelete, "/api/series/999/data", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown series: got %d, want 404", rr.Code)
	}
}

func TestListDownloads_DeletedFilter(t *testing.T) {
	srv, q := newTestServerWithHooks(t, api.SchedulerHooks{})
	mustInsertRelease(t, q, "aabbccdd", "deleted")
	mustInsertRelease(t, q, "eeff0011", "completed")

	rr := do(t, srv, http.MethodGet, "/api/downloads?status=deleted", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	var out []struct {
		InfoHash string `json:"info_hash"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 || out[0].InfoHash != "aabbccdd" || out[0].Status != "deleted" {
		t.Errorf("unexpected result: %+v", out)
	}
}
