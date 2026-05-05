package monitoring

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BarkNotifier 推送到 Bark (iPhone) https://github.com/Finb/Bark
type BarkNotifier struct {
	deviceKey string
	server    string
	client    *http.Client
}

func NewBarkNotifier(deviceKey string) *BarkNotifier {
	if deviceKey == "" {
		return nil
	}
	return &BarkNotifier{
		deviceKey: deviceKey,
		server:    "https://api.day.app",
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 发送普通推送
func (b *BarkNotifier) Send(title, body string) error {
	return b.SendWithLevel(title, body, "active")
}

// SendWithLevel level: passive(静默)/active(默认)/timeSensitive(紧急, 锁屏可见)/critical(突破免打扰)
func (b *BarkNotifier) SendWithLevel(title, body, level string) error {
	if b == nil || b.deviceKey == "" {
		return nil
	}
	u := fmt.Sprintf("%s/%s/%s/%s", b.server, b.deviceKey, url.PathEscape(title), url.PathEscape(body))
	params := url.Values{}
	params.Set("level", level)
	params.Set("group", "TransitAI") // iOS 通知分组
	params.Set("icon", "https://transitai.cloud/favicon.ico")
	if level == "critical" || level == "timeSensitive" {
		params.Set("sound", "alarm")
	}
	u += "?" + params.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("bark: %w", err)
	}
	defer resp.Body.Close()
	body2, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("bark http %d: %s", resp.StatusCode, strings.TrimSpace(string(body2)))
	}
	return nil
}
