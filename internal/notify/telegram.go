package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/agent"
)

const defaultTelegramBaseURL = "https://api.telegram.org"

type TelegramNotifier interface {
	Notify(context.Context, *ksv1alpha1.HealingRequest) error
}

type TelegramConfig struct {
	BotToken string
	ChatID   string
	BaseURL  string
	Timeout  time.Duration
}

type telegramNotifier struct {
	config TelegramConfig
	client *http.Client
}

type sendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

func NewTelegramNotifier(cfg TelegramConfig) TelegramNotifier {
	if strings.TrimSpace(cfg.BotToken) == "" || strings.TrimSpace(cfg.ChatID) == "" {
		return nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultTelegramBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &telegramNotifier{config: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (n *telegramNotifier) Notify(ctx context.Context, req *ksv1alpha1.HealingRequest) error {
	if req == nil {
		return fmt.Errorf("healing request is required")
	}
	report := agent.BuildReport(req, agent.Evidence{})
	for _, message := range []string{report.Notification.ShortMessage, report.Notification.LongMessage} {
		if strings.TrimSpace(message) == "" {
			continue
		}
		if err := n.send(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

func (n *telegramNotifier) send(ctx context.Context, message string) error {
	payload, err := json.Marshal(sendMessageRequest{ChatID: n.config.ChatID, Text: message, DisableWebPagePreview: true})
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}
	url := strings.TrimRight(n.config.BaseURL, "/") + "/bot" + n.config.BotToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api returned status %d", resp.StatusCode)
	}
	return nil
}
