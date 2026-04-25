package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/gradleless/guetteur/internal/db"
	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

func testDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, dbgen.New(conn)
}

func TestOpen_CreatesSchema(t *testing.T) {
	conn, _ := testDB(t)
	ctx := context.Background()

	for _, table := range []string{"series", "releases", "airing_schedule", "kv"} {
		var name string
		err := conn.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migrations: %v", table, err)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotent.db")

	conn1, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	conn1.Close()

	conn2, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("second open (migrations must be idempotent): %v", err)
	}
	conn2.Close()
}

func upsertSeries(t *testing.T, q *dbgen.Queries, id int64, romaji string) {
	t.Helper()
	if err := q.UpsertSeries(context.Background(), dbgen.UpsertSeriesParams{
		ID:                  id,
		TitleRomaji:         romaji,
		AliasesJson:         "[]",
		Status:              "RELEASING",
		PreferredGroupsJson: "[]",
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertSeries(%d, %q): %v", id, romaji, err)
	}
}

func TestSeriesUpsertAndGet(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	if err := q.UpsertSeries(ctx, dbgen.UpsertSeriesParams{
		ID:                  42,
		TitleRomaji:         "Frieren",
		TitleEnglish:        sql.NullString{String: "Frieren: Beyond Journey's End", Valid: true},
		AliasesJson:         `["Sousou no Frieren"]`,
		Status:              "FINISHED",
		PreferredGroupsJson: "[]",
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertSeries: %v", err)
	}

	got, err := q.GetSeriesByID(ctx, 42)
	if err != nil {
		t.Fatalf("GetSeriesByID: %v", err)
	}
	if got.TitleRomaji != "Frieren" {
		t.Errorf("TitleRomaji = %q, want Frieren", got.TitleRomaji)
	}
	if !got.TitleEnglish.Valid || got.TitleEnglish.String != "Frieren: Beyond Journey's End" {
		t.Errorf("TitleEnglish = %v", got.TitleEnglish)
	}

	if got.FollowState != 0 {
		t.Errorf("default FollowState = %d, want 0", got.FollowState)
	}
}

func TestSeriesUpsert_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	upsertSeries(t, q, 1, "Original Title")

	if err := q.UpsertSeries(ctx, dbgen.UpsertSeriesParams{
		ID:                  1,
		TitleRomaji:         "Updated Title",
		AliasesJson:         "[]",
		Status:              "RELEASING",
		PreferredGroupsJson: "[]",
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertSeries (update): %v", err)
	}

	got, _ := q.GetSeriesByID(ctx, 1)
	if got.TitleRomaji != "Updated Title" {
		t.Errorf("TitleRomaji = %q, want Updated Title", got.TitleRomaji)
	}

	all, _ := q.ListAllSeries(ctx)
	if len(all) != 1 {
		t.Errorf("ListAllSeries: got %d rows, want 1 (no duplicates)", len(all))
	}
}

func TestSeriesGetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	_, err := q.GetSeriesByID(ctx, 9999)
	if err == nil {
		t.Error("expected error for missing series, got nil")
	}
}

func TestSeriesFollowStateTransitions(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	upsertSeries(t, q, 1, "Test")

	cases := []struct {
		state int64
		name  string
	}{
		{1, "follow"},
		{2, "archive"},
		{0, "ignore"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := q.UpdateSeriesFollowState(ctx, dbgen.UpdateSeriesFollowStateParams{
				ID: 1, FollowState: tc.state,
			}); err != nil {
				t.Fatalf("UpdateSeriesFollowState: %v", err)
			}
			got, _ := q.GetSeriesByID(ctx, 1)
			if got.FollowState != tc.state {
				t.Errorf("FollowState = %d, want %d", got.FollowState, tc.state)
			}
		})
	}
}

func TestListFollowedSeries(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	upsertSeries(t, q, 1, "Followed")
	upsertSeries(t, q, 2, "Ignored")
	q.UpdateSeriesFollowState(ctx, dbgen.UpdateSeriesFollowStateParams{ID: 1, FollowState: 1})

	followed, err := q.ListFollowedSeries(ctx)
	if err != nil {
		t.Fatalf("ListFollowedSeries: %v", err)
	}
	if len(followed) != 1 {
		t.Fatalf("got %d followed series, want 1", len(followed))
	}
	if followed[0].ID != 1 {
		t.Errorf("followed series ID = %d, want 1", followed[0].ID)
	}
}

func insertRelease(t *testing.T, q *dbgen.Queries, infoHash, status string) {
	t.Helper()
	if err := q.InsertRelease(context.Background(), dbgen.InsertReleaseParams{
		InfoHash:   infoHash,
		RawTitle:   "[SubsPlease] Test - 01 (1080p)",
		Magnet:     "magnet:?xt=urn:btih:" + infoHash,
		DetectedAt: time.Now().UTC(),
		Status:     status,
	}); err != nil {
		t.Fatalf("InsertRelease(%q): %v", infoHash, err)
	}
}

func TestReleaseInsertAndGet(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	insertRelease(t, q, "deadbeef", "queued")

	got, err := q.GetReleaseByInfoHash(ctx, "deadbeef")
	if err != nil {
		t.Fatalf("GetReleaseByInfoHash: %v", err)
	}
	if got.Status != "queued" {
		t.Errorf("Status = %q, want queued", got.Status)
	}
	if got.Progress != 0 {
		t.Errorf("Progress = %f, want 0", got.Progress)
	}
}

func TestReleaseInsert_DuplicateNoop(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	insertRelease(t, q, "dup", "queued")
	insertRelease(t, q, "dup", "queued")

	rows, _ := q.ListReleasesByStatus(ctx, "queued")
	if len(rows) != 1 {
		t.Errorf("expected 1 row after duplicate insert, got %d", len(rows))
	}
}

func TestReleaseStatusTransition(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	insertRelease(t, q, "abc", "queued")

	if err := q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		Status: "downloading", InfoHash: "abc",
	}); err != nil {
		t.Fatalf("UpdateReleaseStatus: %v", err)
	}
	r, _ := q.GetReleaseByInfoHash(ctx, "abc")
	if r.Status != "downloading" {
		t.Errorf("Status = %q, want downloading", r.Status)
	}

	if err := q.UpdateReleaseProgress(ctx, dbgen.UpdateReleaseProgressParams{
		Progress: 0.75, InfoHash: "abc",
	}); err != nil {
		t.Fatalf("UpdateReleaseProgress: %v", err)
	}
	r, _ = q.GetReleaseByInfoHash(ctx, "abc")
	if r.Progress != 0.75 {
		t.Errorf("Progress = %f, want 0.75", r.Progress)
	}

	if err := q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		Status: "completed", InfoHash: "abc",
	}); err != nil {
		t.Fatalf("UpdateReleaseStatus completed: %v", err)
	}
	r, _ = q.GetReleaseByInfoHash(ctx, "abc")
	if r.Status != "completed" {
		t.Errorf("Status = %q, want completed", r.Status)
	}
}

func TestReleaseListByStatus(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	insertRelease(t, q, "h1", "queued")
	insertRelease(t, q, "h2", "queued")
	insertRelease(t, q, "h3", "downloading")
	insertRelease(t, q, "h4", "completed")

	queued, _ := q.ListReleasesByStatus(ctx, "queued")
	if len(queued) != 2 {
		t.Errorf("queued: got %d, want 2", len(queued))
	}

	downloading, _ := q.ListReleasesByStatus(ctx, "downloading")
	if len(downloading) != 1 {
		t.Errorf("downloading: got %d, want 1", len(downloading))
	}

	completed, _ := q.ListReleasesByStatus(ctx, "completed")
	if len(completed) != 1 {
		t.Errorf("completed: got %d, want 1", len(completed))
	}
}

func TestReleaseListByStatus_Empty(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)

	rows, err := q.ListReleasesByStatus(ctx, "queued")
	if err != nil {
		t.Fatalf("ListReleasesByStatus on empty DB: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty slice, got %d", len(rows))
	}
}

func TestReleaseStatusError(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	insertRelease(t, q, "err1", "queued")

	if err := q.UpdateReleaseStatus(ctx, dbgen.UpdateReleaseStatusParams{
		Status:   "failed",
		ErrorMsg: sql.NullString{String: "disk full", Valid: true},
		InfoHash: "err1",
	}); err != nil {
		t.Fatalf("UpdateReleaseStatus failed: %v", err)
	}

	r, _ := q.GetReleaseByInfoHash(ctx, "err1")
	if r.Status != "failed" {
		t.Errorf("Status = %q, want failed", r.Status)
	}
	if !r.ErrorMsg.Valid || r.ErrorMsg.String != "disk full" {
		t.Errorf("ErrorMsg = %v, want disk full", r.ErrorMsg)
	}
}

func TestAiringScheduleUpsertAndList(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	upsertSeries(t, q, 1, "My Hero Academia")

	future := time.Now().UTC().Add(48 * time.Hour)
	if err := q.UpsertAiringSchedule(ctx, dbgen.UpsertAiringScheduleParams{
		SeriesID: 1, Episode: 5, AiringAt: future,
	}); err != nil {
		t.Fatalf("UpsertAiringSchedule: %v", err)
	}

	slots, err := q.ListAiringScheduleForSeries(ctx, 1)
	if err != nil {
		t.Fatalf("ListAiringScheduleForSeries: %v", err)
	}
	if len(slots) != 1 || slots[0].Episode != 5 {
		t.Errorf("got slots %v, want [{episode:5}]", slots)
	}
}

func TestGetNextAiringForSeries_NoRows(t *testing.T) {
	ctx := context.Background()
	_, q := testDB(t)
	upsertSeries(t, q, 1, "Anime")

	_, err := q.GetNextAiringForSeries(ctx, dbgen.GetNextAiringForSeriesParams{
		SeriesID: 1, AiringAt: time.Now().UTC(),
	})
	if err == nil {
		t.Error("expected sql.ErrNoRows for series with no airing schedule, got nil")
	}
}
