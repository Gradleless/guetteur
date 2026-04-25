package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ch, unsub := s.bus.Subscribe()
	defer unsub()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	var id int
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			payload, _ := json.Marshal(ev.Payload)
			id++
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, ev.Type, payload)
			flusher.Flush()
		}
	}
}
