package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testYAML = `
poll_interval: 5m
quality_priority:
  - "1080p"
  - "720p"
default_groups:
  - "SubsPlease"
  - "Erai-raws"
group_fallback_after: 12h
min_seeders: 3
prefer_single_episodes: true
replace_with_batch: true
min_free_space_gb: 20
media_filename_template: "{title}/Season {season:02d}/{title} - S{season:02d}E{episode:02d} [{group}][{resolution}].{ext}"
anilist_refresh: 6h
torrent:
  listen_port: 42069
  max_peers_per_torrent: 80
  sequential_download: true
  seed_ratio: 1.0
  seed_duration: 48h
notifications:
  on_episode_detected: true
  on_download_start: true
  on_download_complete: true
  on_error: true
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return filepath.ToSlash(f.Name())
}

func TestLoad(t *testing.T) {
	path := writeTempConfig(t, testYAML)
	t.Setenv("CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.PollInterval.Duration != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", cfg.PollInterval)
	}
	if cfg.AnilistRefresh.Duration != 6*time.Hour {
		t.Errorf("AnilistRefresh = %v, want 6h", cfg.AnilistRefresh)
	}
	if cfg.Torrent.ListenPort != 42069 {
		t.Errorf("Torrent.ListenPort = %d, want 42069", cfg.Torrent.ListenPort)
	}
	if cfg.Torrent.SeedDuration.Duration != 48*time.Hour {
		t.Errorf("Torrent.SeedDuration = %v, want 48h", cfg.Torrent.SeedDuration)
	}
	if len(cfg.DefaultGroups) != 2 || cfg.DefaultGroups[0] != "SubsPlease" {
		t.Errorf("DefaultGroups = %v", cfg.DefaultGroups)
	}
	if !cfg.Notifications.OnDownloadComplete {
		t.Error("Notifications.OnDownloadComplete should be true")
	}
}

func TestEnvDefaults(t *testing.T) {

	for _, k := range []string{"CONFIG_PATH", "DB_PATH", "API_PORT", "MEDIA_DIR"} {
		t.Setenv(k, "")
	}

	env := loadEnv()

	if env.ConfigPath != "/config/config.yaml" {
		t.Errorf("ConfigPath default = %q", env.ConfigPath)
	}
	if env.DBPath != "/data/guetteur.db" {
		t.Errorf("DBPath default = %q", env.DBPath)
	}
	if env.APIPort != "8080" {
		t.Errorf("APIPort default = %q", env.APIPort)
	}
}
