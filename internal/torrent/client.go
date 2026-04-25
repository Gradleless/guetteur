package torrent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	atort "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"github.com/gradleless/guetteur/internal/config"
)

type torrentEntry struct {
	t          *atort.Torrent
	storageDir string
}

type speedSample struct {
	bytes int64
	at    time.Time
	bps   int64
}

type Client struct {
	tc        *atort.Client
	mediaDir  string
	stateDir  string
	minFreeGB int
	mu        sync.RWMutex
	torrents  map[string]*torrentEntry

	speedMu      sync.Mutex
	speedSamples map[string]speedSample
}

func New(cfg *config.Config) (*Client, error) {
	stateDir := filepath.Join(filepath.Dir(cfg.Env.DBPath), "torrent-state")
	tcfg := atort.NewDefaultClientConfig()
	tcfg.ListenPort = cfg.Torrent.ListenPort
	tcfg.DataDir = stateDir
	if cfg.Torrent.MaxPeersPerTorrent > 0 {
		tcfg.EstablishedConnsPerTorrent = cfg.Torrent.MaxPeersPerTorrent
	}
	if cfg.Torrent.DownloadRateLimitMbps > 0 {
		bytesPerSec := rate.Limit(cfg.Torrent.DownloadRateLimitMbps * 1e6 / 8)

		tcfg.DownloadRateLimiter = rate.NewLimiter(bytesPerSec, 1<<20)
	}
	if cfg.Torrent.UploadRateLimitMbps > 0 {
		bytesPerSec := rate.Limit(cfg.Torrent.UploadRateLimitMbps * 1e6 / 8)
		tcfg.UploadRateLimiter = rate.NewLimiter(bytesPerSec, 1<<20)
	}

	tc, err := atort.NewClient(tcfg)
	if err != nil {
		return nil, fmt.Errorf("create torrent client: %w", err)
	}

	return &Client{
		tc:           tc,
		mediaDir:     cfg.Env.MediaDir,
		stateDir:     stateDir,
		minFreeGB:    cfg.MinFreeSpaceGB,
		torrents:     make(map[string]*torrentEntry),
		speedSamples: make(map[string]speedSample),
	}, nil
}

func (c *Client) CheckDiskSpace() error {
	free, err := freeSpaceGB(c.mediaDir)
	if err != nil {
		return fmt.Errorf("check disk space: %w", err)
	}
	if free < c.minFreeGB {
		return fmt.Errorf("insufficient disk space: %d GB free, need %d GB", free, c.minFreeGB)
	}
	return nil
}

func (c *Client) Add(ctx context.Context, infoHash, magnet, storageDir string) error {
	ih := strings.ToLower(infoHash)

	c.mu.Lock()
	if _, ok := c.torrents[ih]; ok {
		c.mu.Unlock()
		slog.Debug("torrent already tracked", "info_hash", ih)
		return nil
	}
	c.mu.Unlock()

	spec, err := atort.TorrentSpecFromMagnetUri(magnet)
	if err != nil {
		return fmt.Errorf("parse magnet %s: %w", ih, err)
	}
	// Per-torrent bolt DB stored in stateDir/completion/{hash}/ — isolated from
	// the media folder so concurrent downloads to the same Season dir don't
	// fight over the same lock. Falls back to in-memory if bolt fails.
	completionDir := filepath.Join(c.stateDir, "completion", ih)
	if err := os.MkdirAll(completionDir, 0o755); err != nil {
		slog.Warn("torrent: couldn't create completion dir, using in-memory", "info_hash", ih, "err", err)
		spec.Storage = storage.NewFileWithCompletion(storageDir, storage.NewMapPieceCompletion())
	} else if pc, err := storage.NewBoltPieceCompletion(completionDir); err != nil {
		slog.Warn("torrent: bolt piece completion unavailable, using in-memory", "info_hash", ih, "err", err)
		spec.Storage = storage.NewFileWithCompletion(storageDir, storage.NewMapPieceCompletion())
	} else {
		spec.Storage = storage.NewFileWithCompletion(storageDir, pc)
	}

	t, _, err := c.tc.AddTorrentSpec(spec)
	if err != nil {
		return fmt.Errorf("add torrent spec: %w", err)
	}

	ctx90, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	select {
	case <-t.GotInfo():
	case <-ctx90.Done():
		return fmt.Errorf("timeout waiting for torrent metadata: %w", ctx90.Err())
	}

	t.DownloadAll()

	c.mu.Lock()
	c.torrents[ih] = &torrentEntry{t: t, storageDir: storageDir}
	c.mu.Unlock()

	slog.Info("torrent started", "info_hash", ih, "name", t.Name(), "storage_dir", storageDir)
	return nil
}

func (c *Client) Progress(infoHash string) float64 {
	c.mu.RLock()
	entry, ok := c.torrents[strings.ToLower(infoHash)]
	c.mu.RUnlock()
	if !ok {
		return -1
	}
	t := entry.t
	if t.Info() == nil {
		return 0
	}
	total := t.Length()
	if total == 0 {
		return 0
	}
	return float64(t.BytesCompleted()) / float64(total)
}

func (c *Client) BytesCompleted(infoHash string) int64 {
	c.mu.RLock()
	entry, ok := c.torrents[strings.ToLower(infoHash)]
	c.mu.RUnlock()
	if !ok || entry.t.Info() == nil {
		return 0
	}
	return entry.t.BytesCompleted()
}

func (c *Client) Seeders(infoHash string) int {
	c.mu.RLock()
	entry, ok := c.torrents[strings.ToLower(infoHash)]
	c.mu.RUnlock()
	if !ok {
		return 0
	}
	stats := entry.t.Stats()
	return int(stats.ConnectedSeeders)
}

func (c *Client) SpeedBps(infoHash string) int64 {
	ih := strings.ToLower(infoHash)

	c.mu.RLock()
	entry, ok := c.torrents[ih]
	c.mu.RUnlock()
	if !ok || entry.t.Info() == nil {
		return 0
	}

	current := entry.t.BytesCompleted()
	now := time.Now()

	c.speedMu.Lock()
	defer c.speedMu.Unlock()

	prev, hasPrev := c.speedSamples[ih]
	if !hasPrev {
		c.speedSamples[ih] = speedSample{bytes: current, at: now, bps: 0}
		return 0
	}

	elapsed := now.Sub(prev.at).Seconds()
	if elapsed < 1.0 {

		return prev.bps
	}

	diff := current - prev.bytes
	if diff < 0 {
		diff = 0
	}
	bps := int64(float64(diff) / elapsed)
	c.speedSamples[ih] = speedSample{bytes: current, at: now, bps: bps}
	return bps
}

func (c *Client) LargestFilePath(infoHash string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.torrents[strings.ToLower(infoHash)]
	c.mu.RUnlock()
	if !ok || entry.t.Info() == nil {
		return "", false
	}

	var largest *atort.File
	for _, f := range entry.t.Files() {
		if largest == nil || f.Length() > largest.Length() {
			largest = f
		}
	}
	if largest == nil {
		return "", false
	}

	return filepath.Join(entry.storageDir, largest.Path()), true
}

func (c *Client) NewReader(infoHash string) (io.ReadSeekCloser, int64, string, bool) {
	c.mu.RLock()
	entry, ok := c.torrents[strings.ToLower(infoHash)]
	c.mu.RUnlock()
	if !ok || entry.t.Info() == nil {
		return nil, 0, "", false
	}

	var largest *atort.File
	for _, f := range entry.t.Files() {
		if largest == nil || f.Length() > largest.Length() {
			largest = f
		}
	}
	if largest == nil {
		return nil, 0, "", false
	}

	r := largest.NewReader()
	r.SetReadahead(8 << 20)
	r.SetResponsive()

	return r, largest.Length(), largest.DisplayPath(), true
}

type ProgressCallback func(infoHash string, progress float64, bytesCompleted int64, speedBps int64, seeders int)

func (c *Client) Watch(ctx context.Context, infoHash string, cb ProgressCallback) {
	ih := strings.ToLower(infoHash)

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		var lastProgress float64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p := c.Progress(ih)
				if p < 0 {

					continue
				}

				if p-lastProgress >= 0.0005 || p >= 1.0 {
					lastProgress = p
					cb(ih, p, c.BytesCompleted(ih), c.SpeedBps(ih), c.Seeders(ih))
				}
				if p >= 1.0 {
					return
				}
			}
		}
	}()
}

// PruneCompletionDB removes the per-torrent bolt piece-completion directory
// once a download is finished. Safe to call after completion; anacrolix only
// reads the DB while pieces are still outstanding.
func (c *Client) PruneCompletionDB(infoHash string) {
	dir := filepath.Join(c.stateDir, "completion", strings.ToLower(infoHash))
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("torrent: prune completion db", "info_hash", infoHash, "err", err)
	}
}

func (c *Client) Drop(infoHash string) {
	ih := strings.ToLower(infoHash)

	c.mu.Lock()
	entry, ok := c.torrents[ih]
	if ok {
		delete(c.torrents, ih)
	}
	c.mu.Unlock()

	if ok {
		entry.t.Drop()
	}

	c.speedMu.Lock()
	delete(c.speedSamples, ih)
	c.speedMu.Unlock()
}

func (c *Client) Close() {
	c.tc.Close()
}
