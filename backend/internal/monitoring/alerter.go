package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// TelegramAlerter sends alert messages via Telegram Bot API.
type TelegramAlerter struct {
	botToken string
	chatID   string
	client   *http.Client
	lastSent sync.Map // key -> time.Time (cooldown)
}

func NewTelegramAlerter(botToken, chatID string) *TelegramAlerter {
	if botToken == "" || chatID == "" {
		log.Println("TelegramAlerter: missing bot_token or chat_id, alerts disabled")
		return nil
	}
	return &TelegramAlerter{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Send sends an HTML-formatted message to the configured Telegram chat.
func (a *TelegramAlerter) Send(message string) error {
	if a == nil {
		return nil
	}
	body, _ := json.Marshal(map[string]string{
		"chat_id":    a.chatID,
		"text":       message,
		"parse_mode": "HTML",
	})
	resp, err := a.client.Post(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.botToken),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send: status %d", resp.StatusCode)
	}
	return nil
}

// SendThrottled sends an alert only if the same key wasn't sent within cooldown.
// Used to avoid spam when an upstream error fires many times per minute.
func (a *TelegramAlerter) SendThrottled(key, message string, cooldown time.Duration) error {
	if a == nil {
		return nil
	}
	now := time.Now()
	if v, ok := a.lastSent.Load(key); ok {
		if last, ok := v.(time.Time); ok {
			if now.Sub(last) < cooldown {
				return nil // 在 cooldown 内, 静默
			}
		}
	}
	a.lastSent.Store(key, now)
	return a.Send(message)
}
