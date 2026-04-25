package nyaa

import (
	"os"
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	f, err := os.Open("testdata/nyaa_rss_sample.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	releases, err := ParseRSS(f)
	if err != nil {
		t.Fatalf("ParseRSS: %v", err)
	}

	if len(releases) != 4 {
		t.Fatalf("got %d releases, want 4", len(releases))
	}

	r0 := releases[0]
	if r0.Title != "[SubsPlease] Frieren - 12 (1080p) [ABC12345].mkv" {
		t.Errorf("r0.Title: got %q", r0.Title)
	}
	if r0.InfoHash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("r0.InfoHash: got %q", r0.InfoHash)
	}
	if r0.Seeders != 42 {
		t.Errorf("r0.Seeders: got %d, want 42", r0.Seeders)
	}
	if r0.Leechers != 5 {
		t.Errorf("r0.Leechers: got %d, want 5", r0.Leechers)
	}
	if r0.NyaaURL != "https://nyaa.si/view/1234567" {
		t.Errorf("r0.NyaaURL: got %q", r0.NyaaURL)
	}
	if r0.Magnet == "" {
		t.Error("r0.Magnet: empty")
	}
	wantPub := time.Date(2023, time.December, 9, 12, 0, 0, 0, time.UTC)
	if !r0.PubDate.Equal(wantPub) {
		t.Errorf("r0.PubDate: got %v, want %v", r0.PubDate, wantPub)
	}

	r1 := releases[1]
	if r1.InfoHash != "1122334455667788990011223344556677889900" {
		t.Errorf("r1.InfoHash: got %q", r1.InfoHash)
	}
	if r1.Seeders != 18 {
		t.Errorf("r1.Seeders: got %d, want 18", r1.Seeders)
	}

	r2 := releases[2]
	if r2.InfoHash != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("r2.InfoHash: got %q", r2.InfoHash)
	}
	if r2.Seeders != 120 {
		t.Errorf("r2.Seeders: got %d, want 120", r2.Seeders)
	}

	r3 := releases[3]
	if r3.InfoHash != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("r3.InfoHash: got %q", r3.InfoHash)
	}
	if r3.Seeders != 7 {
		t.Errorf("r3.Seeders: got %d, want 7", r3.Seeders)
	}
}

func TestBuildRSSURL(t *testing.T) {

	got := BuildRSSURL("Frieren", "1080p", "")
	want := "https://nyaa.si/?page=rss&c=1_0&f=0&s=seeders&o=desc&q=Frieren+1080p"
	if got != want {
		t.Errorf("no-group: got %q, want %q", got, want)
	}

	got2 := BuildRSSURL("Frieren", "1080p", "SubsPlease")
	want2 := "https://nyaa.si/?page=rss&c=1_0&f=0&s=seeders&o=desc&q=Frieren+1080p&u=SubsPlease"
	if got2 != want2 {
		t.Errorf("with-group: got %q, want %q", got2, want2)
	}

	got3 := BuildRSSURL("Frieren", "1080p", "SubsPlease", "VOSTFR")
	want3 := "https://nyaa.si/?page=rss&c=1_0&f=0&s=seeders&o=desc&q=Frieren+1080p+VOSTFR&u=SubsPlease"
	if got3 != want3 {
		t.Errorf("with-group+lang: got %q, want %q", got3, want3)
	}
}
