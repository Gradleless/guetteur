package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Notifier struct {
	mu             sync.RWMutex
	discordWebhook string
	ntfyTopic      string
	client         *http.Client
}

func New(discordWebhook, ntfyTopic string) *Notifier {
	return &Notifier{
		discordWebhook: discordWebhook,
		ntfyTopic:      ntfyTopic,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *Notifier) Update(discordWebhook, ntfyTopic string) {
	n.mu.Lock()
	n.discordWebhook = discordWebhook
	n.ntfyTopic = ntfyTopic
	n.mu.Unlock()
	slog.Info("notifier updated", "discord_set", discordWebhook != "", "ntfy_set", ntfyTopic != "")
}

func (n *Notifier) Config() (discordWebhook, ntfyTopic string) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.discordWebhook, n.ntfyTopic
}

type Event struct {
	Title   string
	Message string
}

func (n *Notifier) Notify(ev Event) {
	discord, ntfy := n.Config()
	if discord != "" {
		if err := n.sendDiscord(ev, discord); err != nil {
			slog.Error("notifier: discord", "err", err, "event_title", ev.Title)
		}
	}
	if ntfy != "" {
		if err := n.sendNtfy(ev, ntfy); err != nil {
			slog.Error("notifier: ntfy", "err", err, "event_title", ev.Title)
		}
	}
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
}

func (n *Notifier) sendDiscord(ev Event, webhook string) error {
	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title:       ev.Title,
				Description: ev.Message,
				Color:       0x5865F2,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send discord request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}

func resolveNtfyURL(topic string) string {
	if strings.HasPrefix(topic, "http://") || strings.HasPrefix(topic, "https://") {
		return topic
	}
	return "https://ntfy.sh/" + topic
}

func (n *Notifier) sendNtfy(ev Event, topic string) error {
	req, err := http.NewRequest(http.MethodPost, resolveNtfyURL(topic), strings.NewReader(ev.Message))
	if err != nil {
		return fmt.Errorf("create ntfy request: %w", err)
	}
	req.Header.Set("Title", ev.Title)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send ntfy request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}
	return nil
}
