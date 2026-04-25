package api

import (
	"encoding/json"
	"net/http"
	"strings"

	dbgen "github.com/gradleless/guetteur/internal/db/generated"
)

const (
	kvDiscordWebhook   = "settings.discord_webhook"
	kvNtfyTopic        = "settings.ntfy_topic"
	kvDefaultGroups    = "settings.default_groups"
	kvPreferredQuality = "settings.preferred_quality"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	discord, ntfy := s.notifier.Config()

	defaultGroups := strings.Join(s.cfg.DefaultGroups, ", ")
	if kv, err := s.q.GetKV(ctx, kvDefaultGroups); err == nil && kv.V != "" {
		defaultGroups = kv.V
	}

	preferredQuality := ""
	if len(s.cfg.QualityPriority) > 0 {
		preferredQuality = s.cfg.QualityPriority[0]
	}
	if kv, err := s.q.GetKV(ctx, kvPreferredQuality); err == nil && kv.V != "" {
		preferredQuality = kv.V
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"discord_webhook":   discord,
		"ntfy_topic":        ntfy,
		"default_groups":    defaultGroups,
		"preferred_quality": preferredQuality,
		"media_dir":         s.cfg.Env.MediaDir,
	})
}

func (s *Server) handlePatchSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DiscordWebhook   *string `json:"discord_webhook"`
		NtfyTopic        *string `json:"ntfy_topic"`
		DefaultGroups    *string `json:"default_groups"`
		PreferredQuality *string `json:"preferred_quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx := r.Context()
	if body.DiscordWebhook != nil {
		_ = s.q.SetKV(ctx, dbgen.SetKVParams{K: kvDiscordWebhook, V: *body.DiscordWebhook})
	}
	if body.NtfyTopic != nil {
		_ = s.q.SetKV(ctx, dbgen.SetKVParams{K: kvNtfyTopic, V: *body.NtfyTopic})
	}
	if body.DefaultGroups != nil {
		_ = s.q.SetKV(ctx, dbgen.SetKVParams{K: kvDefaultGroups, V: *body.DefaultGroups})
	}
	if body.PreferredQuality != nil {
		_ = s.q.SetKV(ctx, dbgen.SetKVParams{K: kvPreferredQuality, V: *body.PreferredQuality})
	}

	discord, ntfy := s.cfg.Env.DiscordWebhook, s.cfg.Env.NtfyTopic
	if kv, err := s.q.GetKV(ctx, kvDiscordWebhook); err == nil {
		discord = kv.V
	}
	if kv, err := s.q.GetKV(ctx, kvNtfyTopic); err == nil {
		ntfy = kv.V
	}
	s.notifier.Update(discord, ntfy)

	w.WriteHeader(http.StatusNoContent)
}
