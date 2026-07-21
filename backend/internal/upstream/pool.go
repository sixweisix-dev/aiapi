package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/vertex"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Channel wraps a DB upstream channel with runtime state.
type Channel struct {
	ID                string
	Name              string
	Provider          string
	APIKey            string
	BaseURL           string
	Weight            int
	MaxConcurrent     int
	HealthStatus      string  // healthy / unhealthy / unknown
	QuotaStatus       string  // normal / warning / critical / exhausted
	EnableCache1hBeta bool   // 注入 anthropic-beta: prompt-caching-1h header
	AutoInjectCache   bool   // 网关层自动注入 cache_control
	GroupID           *uint   // channel_group 关联
	GroupSlug         string  // 分组 slug (economy/official)
	GroupMultiplier   float64 // 分组倍率
	SupportedModels   string  // 模型 whitelist (逗号分隔), 空=支持本组所有模型
	FallbackChannelIDs string  // 显式故障转移链 (逗号分隔 UUID)
	IsDedicated       bool
	DedicatedUserIDs  string  // 逗号分隔
	DedicatedUserIDsAuto string  // 自动隔离名单
	concurrent        int64
	ErrorCount        int64
	sem               chan struct{} // 信号量, 容量=MaxConcurrent, 超出排队等待
}

// Pool manages upstream channels with load balancing.
type Pool struct {
	mu       sync.RWMutex
	channels []*Channel
	db       *gorm.DB
	client   *http.Client
	rdb      *redis.Client // 错误指标滑动窗口
	vertexTM *vertex.TokenManager
	// modelBinding: model_name -> upstream_channel_id (为空表示无绑定)
	// Refresh 时一次性加载, 避免热路径 DB 查询
	modelBinding map[string]string
}

func NewPool(db *gorm.DB, rdb *redis.Client, vertexTM *vertex.TokenManager) *Pool {
	p := &Pool{
		db:       db,
		rdb:      rdb,
		vertexTM: vertexTM,
		client: &http.Client{
			Timeout: 240 * time.Second,
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
	// 加载所有 channel_groups 到 map (便于 lookup)
	var groups []models.ChannelGroup
	if err := p.db.Find(&groups).Error; err != nil {
		log.Printf("ChannelGroups load failed: %v", err)
	}
	groupMap := make(map[uint]*models.ChannelGroup, len(groups))
	for i := range groups {
		groupMap[groups[i].ID] = &groups[i]
	}

	channels := make([]*Channel, 0, len(rows))
	for _, r := range rows {
		baseURL := ""
		if r.BaseURL != nil {
			baseURL = *r.BaseURL
		}
		var groupSlug string
		groupMult := 1.0
		if r.GroupID != nil {
			if g, ok := groupMap[*r.GroupID]; ok {
				groupSlug = g.Slug
				groupMult = g.Multiplier
			}
		}
		channels = append(channels, &Channel{
			ID:               r.ID.String(),
			Name:             r.Name,
			Provider:         r.Provider,
			APIKey:           r.APIKeyEncrypted,
			BaseURL:          baseURL,
			Weight:           r.Weight,
			MaxConcurrent:    r.MaxConcurrent,
			sem:              makeSemaphore(r.MaxConcurrent),
			HealthStatus:     r.HealthStatus,
			QuotaStatus:      r.QuotaStatus,
			EnableCache1hBeta: r.EnableCache1hBeta,
			AutoInjectCache:   r.AutoInjectCache,
			GroupID:           r.GroupID,
			GroupSlug:         groupSlug,
			GroupMultiplier:   groupMult,
			IsDedicated:      r.IsDedicated,
			DedicatedUserIDs: r.DedicatedUserIDs,
			DedicatedUserIDsAuto: r.DedicatedUserIDsAuto,
			SupportedModels:    r.SupportedModels,
			FallbackChannelIDs: r.FallbackChannelIDs,
		})
	}

	// 加载模型绑定映射 (model_name -> upstream_channel_id)
	type modelBindRow struct {
		Name              string
		UpstreamChannelID *string `gorm:"column:upstream_channel_id"`
	}
	var modelBindings []modelBindRow
	if err := p.db.Table("models").Select("name, upstream_channel_id").Where("is_enabled = ? AND upstream_channel_id IS NOT NULL", true).Scan(&modelBindings).Error; err != nil {
		log.Printf("Model bindings load failed: %v", err)
	}
	bindingMap := make(map[string]string, len(modelBindings))
	for _, r := range modelBindings {
		if r.UpstreamChannelID != nil && *r.UpstreamChannelID != "" {
			bindingMap[r.Name] = *r.UpstreamChannelID
		}
	}

	p.mu.Lock()
	p.channels = channels
	p.modelBinding = bindingMap
	p.mu.Unlock()
	log.Printf("Upstream pool refreshed: %d channels loaded, %d model bindings", len(channels), len(bindingMap))
}

// Select picks a channel for the given provider using weighted random.
// modelName 用于按渠道 SupportedModels 白名单过滤; 空字符串表示不按模型过滤
func (p *Pool) Select(provider, modelName string, modelGroupID ...*uint) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// variadic 实现可选参数, 向后兼容旧 caller
	var groupID *uint
	if len(modelGroupID) > 0 {
		groupID = modelGroupID[0]
	}

	var candidates []*Channel
	for _, c := range p.channels {
		// 过滤: provider/group/模型白名单/并发/健康/额度状态/非专属
		if (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) &&
			groupMatches(c.GroupID, groupID) &&
			channelSupportsModel(c, modelName) &&
			c.HealthStatus != "unhealthy" &&
			c.QuotaStatus != "critical" && c.QuotaStatus != "exhausted" &&
			!c.IsDedicated {
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


// SelectSticky 基于 userID 一致性哈希选择上游, 失败时回退到 weighted Select
// modelName 用于按渠道 SupportedModels 白名单过滤; 空字符串表示不按模型过滤
func (p *Pool) SelectSticky(provider, modelName string, groupID *uint, userID string) *Channel {
	p.mu.RLock()
	// 0. 优先看是否在某专属渠道列表中
	if userID != "" {
		for _, c := range p.channels {
			if c.IsDedicated && (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) &&
				groupMatches(c.GroupID, groupID) &&
				channelSupportsModel(c, modelName) &&
				c.concurrent < int64(c.MaxConcurrent) &&
				c.HealthStatus != "unhealthy" &&
				c.QuotaStatus != "critical" && c.QuotaStatus != "exhausted" &&
				(containsUserID(c.DedicatedUserIDs, userID) || containsUserID(c.DedicatedUserIDsAuto, userID)) {
				p.mu.RUnlock()
				return c
			}
		}
	}
	var healthy []*Channel
	for _, c := range p.channels {
		if (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) &&
			groupMatches(c.GroupID, groupID) &&
			channelSupportsModel(c, modelName) &&
			c.HealthStatus != "unhealthy" &&
			c.QuotaStatus != "critical" && c.QuotaStatus != "exhausted" &&
			!c.IsDedicated {
			healthy = append(healthy, c)
		}
	}
	p.mu.RUnlock()
	if len(healthy) == 0 {
		return nil
	}
	if len(healthy) == 1 || userID == "" {
		return healthy[0]
	}
	// 按 weight 排序后用 userID hash 选, 让 weight 大的被选中概率更高
	// 简单方案: 把 userID hash 后对 totalWeight 取模
	totalWeight := 0
	for _, c := range healthy {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	h := uint32(0)
	for i := 0; i < len(userID); i++ {
		h = h*31 + uint32(userID[i])
	}
	pick := int(h) % totalWeight
	for _, c := range healthy {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		pick -= w
		if pick < 0 {
			return c
		}
	}
	return healthy[len(healthy)-1]
}

// makeSemaphore 创建信号量; size <= 0 则 nil (不限流)
func makeSemaphore(size int) chan struct{} {
	if size <= 0 {
		return nil
	}
	return make(chan struct{}, size)
}

// Do sends a request through the selected channel and handles retry.
func (p *Pool) Do(ctx context.Context, ch *Channel, method, path string, reqBody []byte) (*http.Response, error) {
	// 信号量排队: 超过 MaxConcurrent 时等待最多 10 秒
	if ch.sem != nil {
		select {
		case ch.sem <- struct{}{}:
			defer func() { <-ch.sem }()
		case <-time.After(10 * time.Second):
			return nil, fmt.Errorf("channel %s: queue timeout (>10s)", ch.Name)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	atomic.AddInt64(&ch.concurrent, 1)
	defer atomic.AddInt64(&ch.concurrent, -1)

	baseURL := ch.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(ch.Provider)
	}

	// === Vertex AI: 动态拼 baseURL ===
	if strings.EqualFold(ch.Provider, "vertex_ai") {
		if p.vertexTM == nil || !p.vertexTM.IsEnabled() {
			return nil, fmt.Errorf("vertex_ai channel %s but TokenManager disabled (set VERTEX_CREDENTIALS_PATH)", ch.Name)
		}
		parts := strings.SplitN(ch.APIKey, "|", 2)
		var projectID, region string
		if len(parts) == 2 {
			projectID = strings.TrimSpace(parts[0])
			region = strings.TrimSpace(parts[1])
		} else {
			region = strings.TrimSpace(parts[0])
		}
		if projectID == "" {
			projectID = p.vertexTM.ProjectID()
		}
		if region == "" {
			region = "us-central1"
		}
		baseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/projects/%s/locations/%s/endpoints/openapi", region, projectID, region)
		// Vertex OpenAI 兼容端点 path 不带 /v1 前缀
		path = strings.TrimPrefix(path, "/v1")
	}

	url := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	bodyReader := bytes.NewReader(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	setHeaders(httpReq, ch)

	// === Vertex AI: 用 OAuth2 token 覆盖 Authorization header ===
	if strings.EqualFold(ch.Provider, "vertex_ai") && p.vertexTM != nil && p.vertexTM.IsEnabled() {
		token, terr := p.vertexTM.GetToken(ctx)
		if terr != nil {
			return nil, fmt.Errorf("vertex token: %w", terr)
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

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
	// vertex_ai: URL 是运行时动态构建, 没有静态 /v1/models 端点, 直接判健康
	if strings.EqualFold(ch.Provider, "vertex_ai") {
		return true
	}
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
		path = "/v1/models" // lightweight ping
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
		return p.Select(provider, "")
	case StrategyRoundRobin:
		return p.roundRobin(provider)
	case StrategyLeastUsed:
		return p.leastUsed(provider)
	default:
		return p.Select(provider, "")
	}
}

func (p *Pool) roundRobin(provider string) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*Channel
	for _, c := range p.channels {
		if (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) {
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
		if (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) && c.concurrent < lowest {
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
	case "vertex_ai":
		return "" // dynamically built in Do()
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
func (p *Pool) SelectAllHealthy(provider string, modelName ...string) []*Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var mName string
	if len(modelName) > 0 {
		mName = modelName[0]
	}

	var candidates []*Channel
	for _, c := range p.channels {
		if (strings.EqualFold(c.Provider, provider) || (strings.EqualFold(c.Provider, "multi_aggregator") && !strings.EqualFold(provider, "vertex_ai"))) &&
			c.HealthStatus != "unhealthy" &&
			channelSupportsModel(c, mName) {
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

// SelectByChannelID 按 channel_id 直接查找 (不做过滤, 用于 model 绑定优先)
func (p *Pool) SelectByChannelID(channelID string) *Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, c := range p.channels {
		if c.ID == channelID {
			return c
		}
	}
	return nil
}

// buildFailoverChain 构造 DoWithFailover 使用的候选链
// - 如果模型有绑定渠道: 首位 = 绑定渠道, 后接 fallback 链, 兜底同 provider 健康渠道
// - 无绑定: 完全走 SelectAllHealthy (weight 排序)
// 全过程去重, 只保留健康渠道
func (p *Pool) buildFailoverChain(provider, modelName string) []*Channel {
	p.mu.RLock()
	boundID := p.modelBinding[modelName]
	p.mu.RUnlock()

	// 无绑定 -> 走旧逻辑
	if boundID == "" {
		return p.SelectAllHealthy(provider, modelName)
	}

	seen := make(map[string]bool)
	var chain []*Channel
	add := func(c *Channel) {
		if c == nil || seen[c.ID] {
			return
		}
		if c.HealthStatus == "unhealthy" {
			return
		}
		seen[c.ID] = true
		chain = append(chain, c)
	}

	// 1. 绑定渠道置首位
	bound := p.SelectByChannelID(boundID)
	add(bound)

	// 2. 追加绑定渠道的显式 fallback 链
	if bound != nil && bound.FallbackChannelIDs != "" {
		for _, fid := range strings.Split(bound.FallbackChannelIDs, ",") {
			fid = strings.TrimSpace(fid)
			if fid == "" {
				continue
			}
			add(p.SelectByChannelID(fid))
		}
	}

	// 3. 兜底: 同 provider 其他健康渠道 (按 weight)
	for _, c := range p.SelectAllHealthy(provider, modelName) {
		add(c)
	}

	return chain
}

// DoWithFailover 按权重顺序依次尝试通道，遇上游故障自动切到下一个。
// 触发故障转移的条件：5xx 错误 / 网络错误 / 429 限流 / 401 认证失败
// 返回值: (响应, 实际使用的 channel, 错误)
func (p *Pool) DoWithFailover(ctx context.Context, provider, method, path string, reqBody []byte, modelName ...string) (*http.Response, *Channel, error) {
	var mName string
	if len(modelName) > 0 && modelName[0] != "" {
		mName = modelName[0]
	} else {
		mName = extractModelFromBody(reqBody)
	}
	candidates := p.buildFailoverChain(provider, mName)
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no healthy upstream channels for provider %s", provider)
	}

	var lastErr error
	const sameChannelRetries = 3 // 5xx 临时故障时同通道重试次数
	for i, ch := range candidates {
		var resp *http.Response
		var err error
		var attempt int
		for attempt = 1; attempt <= sameChannelRetries; attempt++ {
			resp, err = p.Do(ctx, ch, method, path, reqBody)

			// 网络错误：可重试同通道一次, 再失败转下一个
			if err != nil {
				if attempt < sameChannelRetries {
					log.Printf("[failover] channel %s network error: %v, retry %d/%d on same channel", ch.Name, err, attempt+1, sameChannelRetries)
					continue
				}
				break // 用完重试次数, 跳出去切下一个通道
			}

			// 5xx 临时故障: 同通道重试
			if resp.StatusCode >= 500 && attempt < sameChannelRetries {
				log.Printf("[failover] channel %s HTTP %d, retry %d/%d on same channel", ch.Name, resp.StatusCode, attempt+1, sameChannelRetries)
				resp.Body.Close()
				continue
			}

			// 非 5xx 或重试用完, 退出循环走 failover 判定
			break
		}

		// 网络错误：尝试下一个
		if err != nil {
			log.Printf("[failover] channel %s (%s) network error: %v, trying next channel", ch.Name, ch.ID, err)
			atomic.AddInt64(&ch.ErrorCount, 1)
			p.recordError(ch.ID, 0)
			lastErr = err
			continue
		}

		// HTTP 错误状态：5xx / 429 / 401 / 403 触发转移
		if shouldFailover(resp.StatusCode) {
			log.Printf("[failover] channel %s (%s) returned HTTP %d, trying next (attempt %d/%d)",
				ch.Name, ch.ID, resp.StatusCode, i+1, len(candidates))
			atomic.AddInt64(&ch.ErrorCount, 1)
			p.recordError(ch.ID, resp.StatusCode)

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

// recordError 记录一次上游错误到 Redis (按分钟桶, 1h 滑动窗口)
func (p *Pool) recordError(channelID string, statusCode int) {
	if p.rdb == nil {
		return
	}
	bucket := time.Now().Unix() / 60
	key := fmt.Sprintf("errors:%s:%d", channelID, bucket)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	p.rdb.Incr(ctx, key)
	p.rdb.Expire(ctx, key, 65*time.Minute)
}

// GetErrorsLastHour 返回最近 60 分钟某 channel 累计错误数 (429/5xx)
func (p *Pool) GetErrorsLastHour(channelID string) int64 {
	if p.rdb == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	now := time.Now()
	keys := make([]string, 60)
	for i := 0; i < 60; i++ {
		bucket := now.Add(-time.Duration(i)*time.Minute).Unix() / 60
		keys[i] = fmt.Sprintf("errors:%s:%d", channelID, bucket)
	}
	res, err := p.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return 0
	}
	var total int64
	for _, v := range res {
		if sv, ok := v.(string); ok {
			if n, err := strconv.ParseInt(sv, 10, 64); err == nil {
				total += n
			}
		}
	}
	return total
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


// containsUserID 检查逗号分隔的 user IDs 列表是否包含目标 ID
func containsUserID(list, target string) bool {
if list == "" {
return false
}
for _, id := range strings.Split(list, ",") {
if strings.TrimSpace(id) == target {
return true
}
}
return false
}

// groupMatches: model 没指定 group → 任何 channel 都行(向后兼容); 否则严格匹配

// channelSupportsModel 检查渠道是否支持给定模型名 (空 SupportedModels 表示支持所有)
// 自动处理 "google/" 前缀 (Vertex AI 协议要求): google/gemini-2.5-flash 等同于 gemini-2.5-flash
func channelSupportsModel(c *Channel, modelName string) bool {
	if c.SupportedModels == "" || modelName == "" {
		return true
	}
	// 去掉常见前缀做兼容匹配
	normalized := strings.TrimPrefix(modelName, "google/")
	normalized = strings.TrimPrefix(normalized, "publishers/google/models/")
	for _, m := range strings.Split(c.SupportedModels, ",") {
		name := strings.TrimSpace(m)
		if name == modelName || name == normalized {
			return true
		}
	}
	return false
}

// extractModelFromBody 从 JSON 请求体中提取 model 字段 (chat/images 通用)
func extractModelFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var m struct{ Model string `json:"model"` }
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	return m.Model
}

func groupMatches(channelGroupID, modelGroupID *uint) bool {
	if modelGroupID == nil {
		return true
	}
	if channelGroupID == nil {
		return false
	}
	return *channelGroupID == *modelGroupID
}
