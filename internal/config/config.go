package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

type TorrentConfig struct {
	ListenPort             int      `yaml:"listen_port"`
	MaxPeersPerTorrent     int      `yaml:"max_peers_per_torrent"`
	SequentialDownload     bool     `yaml:"sequential_download"`
	SeedRatio              float64  `yaml:"seed_ratio"`
	SeedDuration           Duration `yaml:"seed_duration"`
	MaxConcurrentDownloads int      `yaml:"max_concurrent_downloads"`
	DownloadRateLimitMbps  float64  `yaml:"download_rate_limit_mbps"`
	UploadRateLimitMbps    float64  `yaml:"upload_rate_limit_mbps"`
}

type MatchConfig struct {
	FuzzyThreshold float32 `yaml:"fuzzy_threshold"`
}

type NotificationsConfig struct {
	OnEpisodeDetected  bool `yaml:"on_episode_detected"`
	OnDownloadStart    bool `yaml:"on_download_start"`
	OnDownloadComplete bool `yaml:"on_download_complete"`
	OnError            bool `yaml:"on_error"`
}

type Config struct {
	PollInterval          Duration            `yaml:"poll_interval"`
	QualityPriority       []string            `yaml:"quality_priority"`
	DefaultGroups         []string            `yaml:"default_groups"`
	PreferredLanguage     []string            `yaml:"preferred_language"`
	GroupFallbackAfter    Duration            `yaml:"group_fallback_after"`
	MinSeeders            int                 `yaml:"min_seeders"`
	PreferSingleEpisodes  bool                `yaml:"prefer_single_episodes"`
	ReplaceWithBatch      bool                `yaml:"replace_with_batch"`
	MinFreeSpaceGB        int                 `yaml:"min_free_space_gb"`
	MediaFilenameTemplate string              `yaml:"media_filename_template"`
	AnilistRefresh        Duration            `yaml:"anilist_refresh"`
	Torrent               TorrentConfig       `yaml:"torrent"`
	Notifications         NotificationsConfig `yaml:"notifications"`
	Match                 MatchConfig         `yaml:"match"`
	DeleteSupersededAfter Duration            `yaml:"delete_superseded_after"`

	Env EnvConfig `yaml:"-"`
}

type EnvConfig struct {
	WireguardPrivateKey string
	ServerCountries     string
	MediaDir            string
	ConfigPath          string
	DBPath              string
	APIPort             string
	DiscordWebhook      string
	NtfyTopic           string
}

func Load() (*Config, error) {
	env := loadEnv()

	f, err := os.Open(env.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("open config file %q: %w", env.ConfigPath, err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config yaml: %w", err)
	}

	cfg.Env = env
	return &cfg, nil
}

func loadEnv() EnvConfig {
	return EnvConfig{
		WireguardPrivateKey: os.Getenv("WIREGUARD_PRIVATE_KEY"),
		ServerCountries:     os.Getenv("SERVER_COUNTRIES"),
		MediaDir:            envOr("MEDIA_DIR", "/media"),
		ConfigPath:          envOr("CONFIG_PATH", "/config/config.yaml"),
		DBPath:              envOr("DB_PATH", "/data/guetteur.db"),
		APIPort:             envOr("API_PORT", "8080"),
		DiscordWebhook:      os.Getenv("DISCORD_WEBHOOK"),
		NtfyTopic:           os.Getenv("NTFY_TOPIC"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
