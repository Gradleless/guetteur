package anilist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func serveFixture(t *testing.T, file string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture %q: %v", file, err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
}

func TestSeasonPage(t *testing.T) {
	ts := serveFixture(t, "testdata/season_page.json")
	defer ts.Close()

	c := New()
	c.baseURL = ts.URL

	result, err := c.SeasonPage(context.Background(), "SPRING", 2025, 1)
	if err != nil {
		t.Fatalf("SeasonPage error: %v", err)
	}

	if result.HasNextPage {
		t.Error("HasNextPage should be false")
	}
	if len(result.Media) != 2 {
		t.Fatalf("expected 2 media, got %d", len(result.Media))
	}

	m := result.Media[0]
	if m.ID != 177709 {
		t.Errorf("ID = %d, want 177709", m.ID)
	}
	if m.Title.Romaji != "Dungeon ni Deai wo Motomeru no wa Machigatteiru Darou ka V" {
		t.Errorf("unexpected romaji: %q", m.Title.Romaji)
	}
	if m.Title.English != "Is It Wrong to Try to Pick Up Girls in a Dungeon? V" {
		t.Errorf("unexpected english: %q", m.Title.English)
	}
	if m.Episodes == nil || *m.Episodes != 13 {
		t.Errorf("Episodes = %v, want 13", m.Episodes)
	}
	if m.Status != "NOT_YET_RELEASED" {
		t.Errorf("Status = %q, want NOT_YET_RELEASED", m.Status)
	}
	if len(m.AiringSchedule.Nodes) != 2 {
		t.Errorf("expected 2 airing nodes, got %d", len(m.AiringSchedule.Nodes))
	}
	if m.NextAiringEpisode == nil || m.NextAiringEpisode.Episode != 1 {
		t.Errorf("unexpected NextAiringEpisode: %v", m.NextAiringEpisode)
	}

	m2 := result.Media[1]
	if m2.NextAiringEpisode != nil {
		t.Errorf("expected nil NextAiringEpisode for finished series")
	}
}

func TestMediaByID(t *testing.T) {
	ts := serveFixture(t, "testdata/media_by_id.json")
	defer ts.Close()

	c := New()
	c.baseURL = ts.URL

	m, err := c.MediaByID(context.Background(), 21)
	if err != nil {
		t.Fatalf("MediaByID error: %v", err)
	}

	if m.ID != 21 {
		t.Errorf("ID = %d, want 21", m.ID)
	}
	if m.Title.Romaji != "ONE PIECE" {
		t.Errorf("Romaji = %q, want ONE PIECE", m.Title.Romaji)
	}
	if m.Episodes != nil {
		t.Errorf("Episodes should be nil (ongoing), got %v", *m.Episodes)
	}
	if m.Status != "RELEASING" {
		t.Errorf("Status = %q, want RELEASING", m.Status)
	}
	if m.NextAiringEpisode == nil || m.NextAiringEpisode.Episode != 1118 {
		t.Errorf("unexpected NextAiringEpisode: %v", m.NextAiringEpisode)
	}
}

func TestSeasonOf(t *testing.T) {
	cases := []struct {
		month  time.Month
		season string
	}{
		{time.January, "WINTER"},
		{time.March, "WINTER"},
		{time.April, "SPRING"},
		{time.June, "SPRING"},
		{time.July, "SUMMER"},
		{time.September, "SUMMER"},
		{time.October, "FALL"},
		{time.December, "FALL"},
	}
	for _, tc := range cases {
		t.Run(tc.month.String(), func(t *testing.T) {
			got, year := SeasonOf(time.Date(2025, tc.month, 1, 0, 0, 0, 0, time.UTC))
			if got != tc.season {
				t.Errorf("SeasonOf(%v) = %q, want %q", tc.month, got, tc.season)
			}
			if year != 2025 {
				t.Errorf("year = %d, want 2025", year)
			}
		})
	}
}
