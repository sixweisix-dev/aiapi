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
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
		},
	}
	p.Refresh()
	go p.periodicRefresh()
	return p
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
		if strings.EqualFold(c.Provider, provider) && c.concurrent < int64(c.MaxConcurrent) {
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
