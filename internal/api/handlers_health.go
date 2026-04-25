package api

import (
	"net/http"
	"os"
	"syscall"
	"time"
)

const appVersion = "v0.1.0"

type healthResponse struct {
	OK            bool    `json:"ok"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	VpnIP         *string `json:"vpn_ip"`
	DBSizeBytes   int64   `json:"db_size_bytes"`
	DiskFreeBytes uint64  `json:"disk_free_bytes"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		OK:            true,
		Version:       appVersion,
		UptimeSeconds: time.Since(s.startAt).Seconds(),
	}

	if kv, err := s.q.GetKV(r.Context(), "system.vpn_ip"); err == nil && kv.V != "" {
		resp.VpnIP = &kv.V
	}

	if dbPath := s.cfg.Env.DBPath; dbPath != "" {
		if fi, err := os.Stat(dbPath); err == nil {
			resp.DBSizeBytes = fi.Size()
		}
	}

	dir := s.cfg.Env.MediaDir
	if dir == "" {
		dir = "."
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err == nil {
		resp.DiskFreeBytes = st.Bavail * uint64(st.Bsize)
	}

	writeJSON(w, http.StatusOK, resp)
}
