package notifier_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gradleless/guetteur/internal/notifier"
)

func TestNotify_Discord(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := notifier.New(srv.URL, "")
	n.Notify(notifier.Event{Title: "Download complete", Message: "Frieren ep 3 done"})

	embeds, ok := received["embeds"].([]any)
	if !ok || len(embeds) == 0 {
		t.Fatal("expected embeds in Discord payload")
	}
	embed := embeds[0].(map[string]any)
	if embed["title"] != "Download complete" {
		t.Errorf("embed title = %q, want %q", embed["title"], "Download complete")
	}
}

func TestNotify_Ntfy_TopicOnly(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := notifier.New("", srv.URL+"/my-topic")
	n.Notify(notifier.Event{Title: "Test", Message: "hello"})

	if !strings.HasSuffix(path, "/my-topic") {
		t.Errorf("ntfy path = %q, want suffix /my-topic", path)
	}
}

func TestNotify_NoBackends(t *testing.T) {

	n := notifier.New("", "")
	n.Notify(notifier.Event{Title: "ignored", Message: "ignored"})
}
