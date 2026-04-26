package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// TelegramAlerter sends alert messages via Telegram Bot API.
type TelegramAlerter struct {
	botToken string
	chatID   string
	client   *http.Client
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
