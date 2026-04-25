package torrent_test

import (
	"sync"
	"testing"

	"github.com/gradleless/guetteur/internal/config"
	torrentclient "github.com/gradleless/guetteur/internal/torrent"
)

func newTestClient(t *testing.T) *torrentclient.Client {
	t.Helper()
	cfg := &config.Config{
		Torrent: config.TorrentConfig{
			ListenPort: 0,
		},
		Env: config.EnvConfig{
			MediaDir: t.TempDir(),
			DBPath:   t.TempDir() + "/test.db",
		},
		MinFreeSpaceGB: 0,
	}
	c, err := torrentclient.New(cfg)
	if err != nil {
		t.Fatalf("torrent.New: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestProgress_Untracked(t *testing.T) {
	c := newTestClient(t)

	got := c.Progress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if got != -1 {
		t.Errorf("Progress(untracked) = %f, want -1", got)
	}
}

func TestProgress_CaseInsensitive(t *testing.T) {
	c := newTestClient(t)

	upper := c.Progress("DEADBEEFDEADBEEFDEADBEEF")
	lower := c.Progress("deadbeefdeadbeefdeadbeef")

	if upper != lower {
		t.Errorf("Progress with upper=%f vs lower=%f — case should not matter", upper, lower)
	}
}

func TestClose_Empty(t *testing.T) {
	cfg := &config.Config{
		Torrent: config.TorrentConfig{ListenPort: 0},
		Env:     config.EnvConfig{MediaDir: t.TempDir(), DBPath: t.TempDir() + "/test.db"},
	}
	c, err := torrentclient.New(cfg)
	if err != nil {
		t.Fatalf("torrent.New: %v", err)
	}

	c.Close()
}

func TestCheckDiskSpace_PassesWithZeroMinimum(t *testing.T) {
	c := newTestClient(t)
	if err := c.CheckDiskSpace(); err != nil {
		t.Errorf("CheckDiskSpace with 0 minimum: unexpected error: %v", err)
	}
}

func TestCheckDiskSpace_FailsWithHugeMinimum(t *testing.T) {
	cfg := &config.Config{
		Torrent:        config.TorrentConfig{ListenPort: 0},
		Env:            config.EnvConfig{MediaDir: t.TempDir(), DBPath: t.TempDir() + "/test.db"},
		MinFreeSpaceGB: 1<<30 - 1,
	}
	c, err := torrentclient.New(cfg)
	if err != nil {
		t.Fatalf("torrent.New: %v", err)
	}
	t.Cleanup(c.Close)

	if err := c.CheckDiskSpace(); err == nil {
		t.Error("CheckDiskSpace with impossibly high minimum: expected error, got nil")
	}
}

func TestNewReader_Untracked(t *testing.T) {
	c := newTestClient(t)

	reader, size, name, ok := c.NewReader("unknown000000000000000000000000000000000")
	if ok {
		t.Error("NewReader(untracked) ok = true, want false")
	}
	if reader != nil {
		t.Error("NewReader(untracked) reader should be nil")
	}
	if size != 0 {
		t.Errorf("NewReader(untracked) size = %d, want 0", size)
	}
	if name != "" {
		t.Errorf("NewReader(untracked) name = %q, want empty", name)
	}
}

func TestConcurrentProgress(t *testing.T) {
	c := newTestClient(t)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c.Progress("cafebabecafebabecafebabecafebabe00000000")
		}()
	}
	wg.Wait()

}

func TestConcurrentNewReader(t *testing.T) {
	c := newTestClient(t)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _, _, _ = c.NewReader("deadbeef000000000000000000000000")
		}()
	}
	wg.Wait()
}
