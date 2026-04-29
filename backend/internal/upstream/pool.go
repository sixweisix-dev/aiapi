package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ai-api-gateway/internal/models"
	"gorm.io/gorm"
)

// Channel wraps a DB upstream channel with runtime state.
type Channel struct {
	ID             string
	Name           string
	Provider       string
	APIKey         string
	BaseURL        string
	Weight         int
	MaxConcurrent  int
	HealthStatus   string  // healthy / unhealthy / unknown
	concurrent     int64
	ErrorCount     int64
}

// Pool manages upstream channels with load balancing.
type Pool struct {
	mu       sync.RWMutex
	channels []*Channel
	db       *gorm.DB
	client   *http.Client
}

func NewPool(db *gorm.DB) *Pool {
	p := &Pool{
		db: db,
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          200,
				MaxIdleConnsPerHost:   50,
				MaxConnsPerHost:       200,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ForceAttemptHTTP2:     true,
				DisableCompression:    false,
			},
		},
	}
	p.Refresh()
	go p.warmup()
	go p.periodicRefresh()
	return p
}

// warmup 启动时建立到所有 provider 的 TCP+TLS 连接, 避免第一个用户等握手
func (p *Pool) warmup() {
	time.Sleep(2 * time.Second)
	p.mu.RLock()
	defer p.mu.RUnlock()
	seen := map[string]bool{}
	for _, ch := range p.channels {
		if ch == nil || ch.HealthStatus != "healthy" {
			continue
		}
		if seen[ch.BaseURL] {
			continue
		}
		seen[ch.BaseURL] = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(ch.BaseURL, "/")+"/", nil)
		if resp, err := p.client.Do(req); err == nil {
			_ = resp.Body.Close()
			log.Printf("[Pool] warmup ok: %s", ch.BaseURL)
		} else {
			log.Printf("[Pool] warmup err (non-fatal): %s %v", ch.BaseURL, err)
		}
		cancel()
	}
}

func (p *Pool) periodicRefresh() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		p.Refresh()
	}
}

func (p *Pool) Refresh() {
	var rows []models.UpstreamChannel
	if err := p.db.Where("is_enabled = ?", true).Find(&rows).Error; err != nil {
		log.Printf("Upstream pool refresh failed: %v", err)
		return
	}

	channels := make([]*Channel, 0, len(rows))
	for _, r := range rows {
		baseURL := ""
		if r.BaseURL != nil {
			baseURL = *r.BaseURL
		}
		channels = append(channels, &Channel{
			ID:            r.ID.String(),
			Name:          r.Name,
			Provider:      r.Provider,
			APIKey:        r.APIKeyEncrypted,
			BaseURL:       baseURL,
			Weight:        r.Weight,
			MaxConcurrent: r.MaxConcurrent,
			HealthStatus:  r.HealthStatus,
		})
	}

	p.mu.Lock()
	p.channels = channels
	p.mu.Unlock()
	log.Printf("Upstream pool refreshed: %d channels loaded", len(channels))
}

// Select picks a channel for the given provider using weighted random.
func (p *Pool) Select(provider string) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*Channel
	for _, c := range p.channels {
		// 过滤：provider 匹配 && 并发未满 && 健康状态非 unhealthy
		if strings.EqualFold(c.Provider, provider) &&
			c.concurrent < int64(c.MaxConcurrent) &&
			c.HealthStatus != "unhealthy" {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	// Weighted selection: iterate with weight-based probability
	totalWeight := 0
	for _, c := range candidates {
		totalWeight += c.Weight
	}
	if totalWeight == 0 {
		totalWeight = len(candidates)
	}

	pick := int(time.Now().UnixNano()) % totalWeight
	for _, c := range candidates {
		pick -= c.Weight
		if pick < 0 {
			return c
		}
	}

	return candidates[len(candidates)-1]
}

// Do sends a request through the selected channel and handles retry.
func (p *Pool) Do(ctx context.Context, ch *Channel, method, path string, reqBody []byte) (*http.Response, error) {
	atomic.AddInt64(&ch.concurrent, 1)
	defer atomic.AddInt64(&ch.concurrent, -1)

	baseURL := ch.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(ch.Provider)
	}

	url := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	bodyReader := bytes.NewReader(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	setHeaders(httpReq, ch)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		atomic.AddInt64(&ch.ErrorCount, 1)
		return nil, fmt.Errorf("upstream do: %w", err)
	}

	return resp, nil
}

// DoWithRetry sends a request with retry logic (1 retry by default).
func (p *Pool) DoWithRetry(ctx context.Context, ch *Channel, method, path string, reqBody []byte) (*http.Response, error) {
	resp, err := p.Do(ctx, ch, method, path, reqBody)
	if err == nil {
		return resp, nil
	}

	// One retry on failure
	log.Printf("Upstream request failed, retrying: %v", err)
	if resp != nil {
		resp.Body.Close()
	}
	return p.Do(ctx, ch, method, path, reqBody)
}

// HealthCheck pings all channels.
func (p *Pool) HealthCheck() {
	p.mu.RLock()
	channels := make([]*Channel, len(p.channels))
	copy(channels, p.channels)
	p.mu.RUnlock()

	for _, ch := range channels {
		healthy := p.ping(ch)
		status := "healthy"
		if !healthy {
			status = "unhealthy"
		}
		p.db.Model(&models.UpstreamChannel{}).Where("id = ?", ch.ID).Updates(map[string]interface{}{
			"health_status":    status,
			"last_health_check": time.Now(),
		})
	}
}

func (p *Pool) ping(ch *Channel) bool {
	baseURL := ch.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(ch.Provider)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var path string
	switch ch.Provider {
	case "openai":
		path = "/v1/models"
	case "anthropic":
		path = "/v1/messages" // lightweight ping
	case "google":
		path = "/v1/models"
	default:
		path = "/v1/models"
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(baseURL, "/")+path, nil)
	setHeaders(req, ch)

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode < 500
}

type SelectStrategy int

const (
	StrategyWeighted SelectStrategy = iota
	StrategyRoundRobin
	StrategyLeastUsed
)

func (p *Pool) SelectWithStrategy(provider string, strategy SelectStrategy) *Channel {
	switch strategy {
	case StrategyWeighted:
		return p.Select(provider)
	case StrategyRoundRobin:
		return p.roundRobin(provider)
	case StrategyLeastUsed:
		return p.leastUsed(provider)
	default:
		return p.Select(provider)
	}
}

func (p *Pool) roundRobin(provider string) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*Channel
	for _, c := range p.channels {
		if strings.EqualFold(c.Provider, provider) {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	idx := int(time.Now().UnixNano()) % len(candidates)
	return candidates[idx]
}

func (p *Pool) leastUsed(provider string) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *Channel
	lowest := int64(1<<63 - 1)
	for _, c := range p.channels {
		if strings.EqualFold(c.Provider, provider) && c.concurrent < lowest {
			lowest = c.concurrent
			best = c
		}
	}
	return best
}

func defaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "google":
		return "https://generativelanguage.googleapis.com"
	default:
		return "https://api.openai.com"
	}
}

func setHeaders(req *http.Request, ch *Channel) {
	switch ch.Provider {
	case "openai":
		req.Header.Set("Authorization", "Bearer "+ch.APIKey)
	case "anthropic":
		req.Header.Set("x-api-key", ch.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "google":
		q := req.URL.Query()
		q.Set("key", ch.APIKey)
		req.URL.RawQuery = q.Encode()
	default:
		req.Header.Set("Authorization", "Bearer "+ch.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
}

// ---- Failover Support ----

// SelectAllHealthy 返回该 provider 下所有健康可用通道（按权重降序）。
// 用于故障转移：第一个失败时尝试第二个、第三个。
func (p *Pool) SelectAllHealthy(provider string) []*Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*Channel
	for _, c := range p.channels {
		if strings.EqualFold(c.Provider, provider) &&
			c.concurrent < int64(c.MaxConcurrent) &&
			c.HealthStatus != "unhealthy" {
			candidates = append(candidates, c)
		}
	}

	// 按 weight 降序：高权重优先
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Weight > candidates[i].Weight {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	return candidates
}

// DoWithFailover 按权重顺序依次尝试通道，遇上游故障自动切到下一个。
// 触发故障转移的条件：5xx 错误 / 网络错误 / 429 限流 / 401 认证失败
// 返回值: (响应, 实际使用的 channel, 错误)
func (p *Pool) DoWithFailover(ctx context.Context, provider, method, path string, reqBody []byte) (*http.Response, *Channel, error) {
	candidates := p.SelectAllHealthy(provider)
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no healthy upstream channels for provider %s", provider)
	}

	var lastErr error
	for i, ch := range candidates {
		resp, err := p.Do(ctx, ch, method, path, reqBody)

		// 网络错误：尝试下一个
		if err != nil {
			log.Printf("[failover] channel %s (%s) network error: %v, trying next", ch.Name, ch.ID, err)
			atomic.AddInt64(&ch.ErrorCount, 1)
			lastErr = err
			continue
		}

		// HTTP 错误状态：5xx / 429 / 401 / 403 触发转移
		if shouldFailover(resp.StatusCode) {
			log.Printf("[failover] channel %s (%s) returned HTTP %d, trying next (attempt %d/%d)",
				ch.Name, ch.ID, resp.StatusCode, i+1, len(candidates))
			atomic.AddInt64(&ch.ErrorCount, 1)

			// 401/403：标记为不健康，由下次 health check 复活
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				p.markUnhealthy(ch.ID)
			}

			resp.Body.Close()
			lastErr = fmt.Errorf("upstream HTTP %d", resp.StatusCode)
			continue
		}

		// 成功
		return resp, ch, nil
	}

	return nil, nil, fmt.Errorf("all %d upstream channels exhausted: %v", len(candidates), lastErr)
}

// shouldFailover 判断 HTTP 状态码是否需要切到下一个通道
func shouldFailover(status int) bool {
	if status >= 500 {
		return true // 5xx
	}
	switch status {
	case 401, 403: // 认证失败 → 通道 key 失效
		return true
	case 429: // 限流
		return true
	}
	return false
}

func (p *Pool) markUnhealthy(channelID string) {
	p.db.Model(&models.UpstreamChannel{}).
		Where("id = ?", channelID).
		Update("health_status", "unhealthy")
	log.Printf("[failover] channel %s marked unhealthy", channelID)

	// 同时更新内存中的状态，避免下次 Refresh 之前继续选中它
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.channels {
		if c.ID == channelID {
			c.HealthStatus = "unhealthy"
			break
		}
	}
}

