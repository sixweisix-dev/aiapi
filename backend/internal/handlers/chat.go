package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/adapter"
	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/middleware"
	"ai-api-gateway/internal/monitoring"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatHandler struct {
	db            *gorm.DB
	pool          *upstream.Pool
	billingEngine *billing.Engine
	alerter       *monitoring.TelegramAlerter
	contentFilter *middleware.ContentFilter
}

func NewChatHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine, alerter *monitoring.TelegramAlerter, cf *middleware.ContentFilter) *ChatHandler {
	return &ChatHandler{db: db, pool: pool, billingEngine: be, alerter: alerter, contentFilter: cf}
}

func (h *ChatHandler) Handle(c *gin.Context) {
	// === Auth (set by APIKey middleware) ===
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if userIDStr == "" {
		c.JSON(401, gin.H{"error": gin.H{"message": "authentication required", "type": "auth_error"}})
		return
	}

	// Parse OpenAI request
	var req adapter.OpenAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid request: " + err.Error(), "type": "invalid_request_error"}})
		return
	}

	// === 内容过滤检查 ===
	if h.contentFilter != nil {
		var checkText strings.Builder
		for _, m := range req.Messages {
			if m.Content != "" {
				checkText.WriteString(m.Content)
				checkText.WriteString(" ")
			}
		}
		result := h.contentFilter.Check(checkText.String())
		if result.Blocked {
			parsedUserID, _ := uuid.Parse(userIDStr)
			var apiKeyUUIDPtr *uuid.UUID
			if v, ok := c.Get("api_key_id"); ok {
				if s, ok2 := v.(string); ok2 && s != "" {
					if u, err := uuid.Parse(s); err == nil {
						apiKeyUUIDPtr = &u
					}
				}
			}
			snippet := checkText.String()
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			h.contentFilter.LogViolation(parsedUserID, apiKeyUUIDPtr, result.Category, result.MatchedKeyword, snippet, c.ClientIP())

			if result.ShouldBlacklist {
				reason := fmt.Sprintf("[%s] 高危关键词: %s", result.Category, result.MatchedKeyword)
				h.contentFilter.MarkBlacklist(parsedUserID, reason)
				if h.alerter != nil {
					go h.alerter.Send(fmt.Sprintf("⛔ <b>用户已被自动拉黑</b>\n\n<b>UserID:</b> <code>%s</code>\n<b>类别:</b> %s\n<b>关键词:</b> <code>%s</code>\n<b>IP:</b> %s",
						userIDStr, result.Category, result.MatchedKeyword, c.ClientIP()))
				}
			} else {
				count, blacklisted := h.contentFilter.IncrementViolation(parsedUserID, result.Category)
				if blacklisted && h.alerter != nil {
					go h.alerter.Send(fmt.Sprintf("⛔ <b>用户累计违规被拉黑</b>\n\n<b>UserID:</b> <code>%s</code>\n<b>累计次数:</b> %d\n<b>最近类别:</b> %s",
						userIDStr, count, result.Category))
				}
			}

			c.JSON(400, gin.H{"error": gin.H{
				"message": "content policy violation: request contains prohibited content",
				"type":    "content_policy_violation",
			}})
			return
		}
	}

	// Look up model in DB
	resolvedModelID, provider, err := h.resolveModel(req.Model)
	if err != nil {
		c.JSON(404, gin.H{"error": gin.H{"message": err.Error(), "type": "model_not_found"}})
		return
	}

	// Get model pricing for later cost calculation
	priceRow, err := h.billingEngine.GetModelPrice(resolvedModelID)
	if err != nil {
		c.JSON(502, gin.H{"error": gin.H{"message": "pricing lookup failed", "type": "internal_error"}})
		return
	}

	// Check allowed models for the API key
	apiKeyHashVal, _ := c.Get("api_key_hash")
	apiKeyHash, _ := apiKeyHashVal.(string)
	if apiKeyHash != "" {
		if err := h.checkAllowedModel(resolvedModelID, apiKeyHash); err != nil {
			c.JSON(403, gin.H{"error": gin.H{"message": err.Error(), "type": "model_not_allowed"}})
			return
		}
	}

	// Get adapter for provider
	chatAdapter, ok := adapter.GetAdapter(provider)
	if !ok {
		c.JSON(502, gin.H{"error": gin.H{"message": "unsupported provider: " + provider, "type": "provider_error"}})
		return
	}

	// Pre-check: ensure at least one healthy upstream exists for this provider
	if healthy := h.pool.SelectAllHealthy(provider); len(healthy) == 0 {
		c.JSON(503, gin.H{"error": gin.H{"message": "no available upstream channel for " + provider, "type": "service_unavailable"}})
		return
	}
	// 占位通道：handleStream/handleNonStream 内部用 DoWithFailover 真正选通道
	var ch *upstream.Channel = nil

	// Estimate max tokens for balance pre-check
	estPromptTokens := estimatePromptTokens(&req)
	maxCompletionTokens := 4096 // safe upper bound
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxCompletionTokens = *req.MaxTokens
	}
	estimatedCost := billing.CalculateCost(estPromptTokens, maxCompletionTokens, priceRow.InputPrice, priceRow.OutputPrice, priceRow.Multiplier)

	// Pre-check balance
	if err := h.billingEngine.PreCheckBalance(userIDStr, estimatedCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"message": err.Error(), "type": "insufficient_balance"}})
		return
	}

	// Pre-check API key 月度预算（防止超额消费）
	if apiKeyIDVal, ok := c.Get("api_key_id"); ok {
		if apiKeyIDStr, ok2 := apiKeyIDVal.(string); ok2 && apiKeyIDStr != "" {
			var budgetRow struct {
				MonthlyBudget *float64
				BudgetUsed    float64
			}
			h.db.Raw("SELECT monthly_budget, budget_used FROM api_keys WHERE id = ?", apiKeyIDStr).Scan(&budgetRow)
			if budgetRow.MonthlyBudget != nil {
				remaining := *budgetRow.MonthlyBudget - budgetRow.BudgetUsed
				if remaining <= 0 {
					c.JSON(429, gin.H{"error": gin.H{
						"message": fmt.Sprintf("API key monthly budget exhausted: ¥%.2f / ¥%.2f", budgetRow.BudgetUsed, *budgetRow.MonthlyBudget),
						"type":    "budget_exceeded",
					}})
					return
				}
				if estimatedCost > remaining {
					c.JSON(429, gin.H{"error": gin.H{
						"message": fmt.Sprintf("estimated cost ¥%.4f exceeds remaining budget ¥%.4f", estimatedCost, remaining),
						"type":    "budget_exceeded",
					}})
					return
				}
			}
		}
	}

	// Convert request to provider format
	upstreamReq, err := chatAdapter.ConvertReq(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "request conversion failed", "type": "internal_error"}})
		return
	}

	// Build upstream path
	upstreamPath := upstreamPath(provider)

	startTime := time.Now()

	if req.Stream {
		h.handleStream(c, userIDStr, &req, chatAdapter, ch, upstreamReq, upstreamPath, resolvedModelID, provider, priceRow, apiKeyHash, startTime)
	} else {
		h.handleNonStream(c, userIDStr, &req, chatAdapter, ch, upstreamReq, upstreamPath, resolvedModelID, provider, priceRow, apiKeyHash, startTime)
	}
}

func (h *ChatHandler) handleNonStream(c *gin.Context, userID string, req *adapter.OpenAIRequest, chatAdapter adapter.ChatAdapter, ch *upstream.Channel, upstreamReq []byte, upstreamPath, modelID, provider string, priceRow *billing.PriceRow, apiKeyHash string, startTime time.Time) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	resp, usedCh, err := h.pool.DoWithFailover(ctx, provider, "POST", upstreamPath, upstreamReq)
	if err != nil {
		log.Printf("[failover] all channels failed: provider=%s model=%s err=%v", provider, req.Model, err)
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, nil, req, 502, 0, 0, 0, 0, startTime, "all upstream channels failed")
		if h.alerter != nil {
			go h.alerter.Send(fmt.Sprintf("🚨 <b>所有上游通道失败</b>\n\n<b>Provider:</b> %s\n<b>Model:</b> %s\n<b>Error:</b> %s\n<b>Time:</b> %s",
				provider, req.Model, err.Error(), time.Now().Format("2006-01-02 15:04:05")))
		}
		c.JSON(502, gin.H{"error": gin.H{"message": "all upstream channels failed: " + err.Error(), "type": "upstream_error"}})
		return
	}
	ch = usedCh
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, ch, req, 502, 0, 0, 0, 0, startTime, "failed to read response")
		c.JSON(502, gin.H{"error": gin.H{"message": "failed to read upstream response", "type": "upstream_error"}})
		return
	}

	if resp.StatusCode >= 400 {
		log.Printf("Upstream error: provider=%s status=%d body=%s", provider, resp.StatusCode, string(body))
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, ch, req, resp.StatusCode, 0, 0, 0, 0, startTime, "upstream error")
		c.JSON(resp.StatusCode, gin.H{
			"error": gin.H{
				"message": "upstream returned error",
				"type":    "upstream_error",
				"code":    resp.StatusCode,
			},
		})
		return
	}

	openAIResp, err := chatAdapter.ConvertResp(body, req.Model)
	if err != nil {
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, ch, req, 502, 0, 0, 0, 0, startTime, "response conversion failed")
		c.JSON(502, gin.H{"error": gin.H{"message": "response conversion failed", "type": "upstream_error"}})
		return
	}

	// Track usage and billing
	if openAIResp.Usage != nil {
		promptTokens := openAIResp.Usage.PromptTokens
		completionTokens := openAIResp.Usage.CompletionTokens
		totalTokens := openAIResp.Usage.TotalTokens
		cost := billing.CalculateCost(promptTokens, completionTokens, priceRow.InputPrice, priceRow.OutputPrice, priceRow.Multiplier)

		apiKeyIDForLog := ""
		if v, ok := c.Get("api_key_id"); ok {
			if s, ok2 := v.(string); ok2 {
				apiKeyIDForLog = s
			}
		}
		go h.postProcess(userID, modelID, apiKeyIDForLog, ch, req.Model, provider, apiKeyHash, promptTokens, completionTokens, totalTokens, cost, nil, startTime, resp.StatusCode)
	}

	c.JSON(200, openAIResp)
}

func (h *ChatHandler) handleStream(c *gin.Context, userID string, req *adapter.OpenAIRequest, chatAdapter adapter.ChatAdapter, ch *upstream.Channel, upstreamReq []byte, upstreamPath, modelID, provider string, priceRow *billing.PriceRow, apiKeyHash string, startTime time.Time) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	resp, usedCh, err := h.pool.DoWithFailover(ctx, provider, "POST", upstreamPath, upstreamReq)
	if err != nil {
		log.Printf("[failover] stream all channels failed: provider=%s model=%s err=%v", provider, req.Model, err)
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, nil, req, 502, 0, 0, 0, 0, startTime, "all upstream channels failed")
		if h.alerter != nil {
			go h.alerter.Send(fmt.Sprintf("🚨 <b>所有上游通道失败 (流式)</b>\n\n<b>Provider:</b> %s\n<b>Model:</b> %s\n<b>Error:</b> %s\n<b>Time:</b> %s",
				provider, req.Model, err.Error(), time.Now().Format("2006-01-02 15:04:05")))
		}
		c.JSON(502, gin.H{"error": gin.H{"message": "all upstream channels failed: " + err.Error(), "type": "upstream_error"}})
		return
	}
	ch = usedCh
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Upstream stream error: provider=%s status=%d body=%s", provider, resp.StatusCode, string(body))
		h.logRequest(c, userID, getAPIKeyIDPtr(c), modelID, ch, req, resp.StatusCode, 0, 0, 0, 0, startTime, "upstream stream error")
		c.JSON(resp.StatusCode, gin.H{"error": gin.H{"message": "upstream error", "type": "upstream_error"}})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Status(200)

	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		c.JSON(500, gin.H{"error": gin.H{"message": "streaming not supported", "type": "internal_error"}})
		return
	}

	var totalPromptTokens, totalCompletionTokens int
	reader := NewSSEBufferedReader(resp.Body)

	for {
		data, err := reader.ReadEvent()
		if err == io.EOF {
			adapter.WriteSSEDone(c.Writer)
			flusher.Flush()
			break
		}
		if err != nil {
			log.Printf("SSE read error: %v", err)
			break
		}

		chunks, done, err := chatAdapter.ConvertStream(data, req.Model)
		if err != nil {
			continue
		}

		for _, chunk := range chunks {
			// Extract usage metadata from special marker
			if len(chunk.Choices) > 0 && strings.Contains(chunk.Choices[0].Delta.Content, "%USAGE%") {
				usageStr := strings.TrimPrefix(strings.TrimPrefix(chunk.Choices[0].Delta.Content, "\n"), "%USAGE%:")
				var usageMeta struct {
					PromptTokens int `json:"prompt_tokens"`
				}
				if err := json.Unmarshal([]byte(usageStr), &usageMeta); err == nil {
					totalPromptTokens = usageMeta.PromptTokens
				}
				continue
			}

			// Count completion tokens from content
			if len(chunk.Choices) > 0 {
				totalCompletionTokens += len([]rune(chunk.Choices[0].Delta.Content)) / 4 // rough estimate
			}

			b, _ := json.Marshal(chunk)
			adapter.WriteSSEChunk(c.Writer, b)
			flusher.Flush()
		}

		if done {
			adapter.WriteSSEDone(c.Writer)
			flusher.Flush()
			break
		}
	}

	// Post-process: billing + logging after stream ends
	totalTokens := totalPromptTokens + totalCompletionTokens
	cost := billing.CalculateCost(totalPromptTokens, totalCompletionTokens, priceRow.InputPrice, priceRow.OutputPrice, priceRow.Multiplier)
	apiKeyIDForLog := ""
	if v, ok := c.Get("api_key_id"); ok {
		if s, ok2 := v.(string); ok2 {
			apiKeyIDForLog = s
		}
	}
	go h.postProcess(userID, modelID, apiKeyIDForLog, ch, req.Model, provider, apiKeyHash, totalPromptTokens, totalCompletionTokens, totalTokens, cost, nil, startTime, http.StatusOK)
}

// postProcess handles billing deduction, request logging, and stats update after a request completes.
func (h *ChatHandler) postProcess(userID, modelID, apiKeyID string, ch *upstream.Channel, modelName, provider, apiKeyHash string, promptTokens, completionTokens, totalTokens int, cost float64, errMsg *string, startTime time.Time, statusCode int) {
	if ch == nil {
		log.Printf("[postProcess] called with nil channel, skipping")
		return
	}
	durationMs := time.Since(startTime).Milliseconds()

	// Deduct balance
	if cost > 0 {
		if _, err := h.billingEngine.DeductBalance(userID, cost); err != nil {
			log.Printf("Balance deduction failed: user=%s model=%s cost=%.8f err=%v", userID, modelName, cost, err)
		}
	}

	parsedUserID, _ := uuid.Parse(userID)

	// 解析 api_key_id 为 *uuid.UUID
	var apiKeyUUIDPtr *uuid.UUID
	if apiKeyID != "" {
		if u, err := uuid.Parse(apiKeyID); err == nil {
			apiKeyUUIDPtr = &u
		}
	}

	// Build request log entry
	reqLog := &models.Request{
		UserID:           parsedUserID,
		APIKeyID:         apiKeyUUIDPtr,
		ModelID:          parseUUID(modelID),
		UpstreamChannelID: parseUUIDPtr(ch.ID),
		Path:             "/v1/chat/completions",
		Method:           "POST",
		StatusCode:       statusCode,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Cost:             cost,
		DurationMs:       int(durationMs),
		ErrorMessage:     errMsg,
	}

	if err := billing.LogRequest(h.db, reqLog); err != nil {
		log.Printf("Failed to log request: %v", err)
	}

	// Create billing record
	if cost > 0 {
		go func() {
			if _, err := h.billingEngine.RecordBilling(userID, modelID, reqLog.ID.String(), promptTokens, completionTokens, totalTokens, cost, provider+"/"+modelName); err != nil {
				log.Printf("Failed to create billing record: %v", err)
			}
		}()
	}

	// Update API key usage
	if apiKeyHash != "" {
		// 累加 total_used + budget_used（CNY 计费）
		h.db.Exec(`UPDATE api_keys 
			SET total_used = total_used + 1, 
			    last_used_at = NOW(),
			    budget_used = budget_used + ?
			WHERE key_hash = ?`, cost, apiKeyHash)

		// 检查是否触发预算告警（异步，避免阻塞）
		go h.checkBudgetAlert(apiKeyHash)
	}

	// Update channel stats
	h.db.Exec("UPDATE upstream_channels SET total_requests = total_requests + 1, total_tokens = total_tokens + ? WHERE id = ?", totalTokens, ch.ID)

	log.Printf("Usage: user=%s model=%s prompt=%d completion=%d cost=%.8f duration=%dms",
		userID[:8], modelName, promptTokens, completionTokens, cost, durationMs)
}

func (h *ChatHandler) resolveModel(modelName string) (modelID, provider string, err error) {
	type modelRow struct {
		ID       string
		Provider string
	}
	var row modelRow
	result := h.db.Table("models").Select("id, provider").Where("name = ? AND is_enabled = ?", modelName, true).First(&row)
	if result.Error != nil {
		return "", "", result.Error
	}
	return row.ID, row.Provider, nil
}

func (h *ChatHandler) checkAllowedModel(modelID, apiKeyHash string) error {
	var count int64
	err := h.db.Table("api_key_allowed_models").
		Joins("JOIN api_keys ON api_keys.id = api_key_allowed_models.api_key_id").
		Where("api_keys.key_hash = ? AND api_key_allowed_models.model_id = ?", apiKeyHash, modelID).
		Count(&count).Error
	if err != nil {
		return nil // allow on error (no restriction)
	}
	// If any allowed model entries exist for this key, the model must be in them
	var total int64
	h.db.Table("api_key_allowed_models").
		Joins("JOIN api_keys ON api_keys.id = api_key_allowed_models.api_key_id").
		Where("api_keys.key_hash = ?", apiKeyHash).
		Count(&total)

	if total > 0 && count == 0 {
		return &modelNotAllowedError{modelID: modelID}
	}
	return nil
}

func (h *ChatHandler) logRequest(c *gin.Context, userID string, apiKeyID *string, modelID string, ch *upstream.Channel, req *adapter.OpenAIRequest, statusCode, promptTokens, completionTokens, totalTokens int, cost float64, startTime time.Time, errMsg string) {
	ip := c.ClientIP()
	ua := c.Request.UserAgent()
	durationMs := int(time.Since(startTime).Milliseconds())

	parsedUserID, _ := uuid.Parse(userID)
	errMsgPtr := &errMsg

	// nil 保护：故障转移耗尽时 ch 为 nil
	var channelIDPtr *uuid.UUID
	if ch != nil {
		channelIDPtr = parseUUIDPtr(ch.ID)
	}

	var apiKeyUUIDPtr2 *uuid.UUID
	if apiKeyID != nil && *apiKeyID != "" {
		if u, err := uuid.Parse(*apiKeyID); err == nil {
			apiKeyUUIDPtr2 = &u
		}
	}

	reqLog := &models.Request{
		UserID:           parsedUserID,
		APIKeyID:         apiKeyUUIDPtr2,
		ModelID:          parseUUID(modelID),
		UpstreamChannelID: channelIDPtr,
		Path:             c.Request.URL.Path,
		Method:           c.Request.Method,
		StatusCode:       statusCode,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Cost:             cost,
		DurationMs:       durationMs,
		IPAddress:        &ip,
		UserAgent:        &ua,
		ErrorMessage:     errMsgPtr,
	}

	if err := billing.LogRequest(h.db, reqLog); err != nil {
		log.Printf("Failed to log request: %v", err)
	}
}

func upstreamPath(provider string) string {
	switch provider {
	case "openai":
		return "/v1/chat/completions"
	case "anthropic":
		return "/v1/messages"
	case "google":
		return "/v1/models/gemini-pro:streamGenerateContent"
	default:
		return "/v1/chat/completions"
	}
}

// estimatePromptTokens roughly estimates the number of prompt tokens for pre-check.
func estimatePromptTokens(req *adapter.OpenAIRequest) int {
	total := 0
	for _, msg := range req.Messages {
		total += len([]rune(msg.Content)) / 4 // rough: ~4 chars per token
		total += 4                            // overhead per message
	}
	if total < 50 {
		total = 50 // minimum
	}
	return total
}

type modelNotAllowedError struct {
	modelID string
}

func (e *modelNotAllowedError) Error() string {
	return "model not allowed for this API key"
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

func parseUUIDPtr(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// --- SSE buffered reader ---

type SSEReader struct {
	reader   io.Reader
	buf      []byte
	readPos  int
	writePos int
}

func NewSSEBufferedReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: r,
		buf:    make([]byte, 65536),
	}
}

func (sr *SSEReader) ReadEvent() ([]byte, error) {
	for {
		// Scan for double newline delimiter
		for i := sr.readPos; i < sr.writePos-1; i++ {
			if sr.buf[i] == '\n' && sr.buf[i+1] == '\n' {
				event := make([]byte, i-sr.readPos)
				copy(event, sr.buf[sr.readPos:i])
				sr.readPos = i + 2
				return event, nil
			}
		}

		if sr.writePos >= len(sr.buf) {
			if sr.readPos > 0 {
				copy(sr.buf, sr.buf[sr.readPos:sr.writePos])
				sr.writePos -= sr.readPos
				sr.readPos = 0
			} else {
				return nil, io.ErrNoProgress
			}
		}

		n, err := sr.reader.Read(sr.buf[sr.writePos:])
		if err != nil {
			return nil, err
		}
		sr.writePos += n
	}
}

// checkBudgetAlert 检查 API key 是否达到预算告警阈值，触发 Telegram 告警。
// 同时如果 budget_used >= monthly_budget 直接禁用 key（防止超额消费）。
func (h *ChatHandler) checkBudgetAlert(apiKeyHash string) {
	var row struct {
		ID             string
		Name           string
		ProjectName    *string
		UserID         string
		Email          string
		MonthlyBudget  *float64
		BudgetUsed     float64
		BudgetAlertPct int
		BudgetAlerted  bool
	}
	err := h.db.Raw(`
		SELECT k.id, k.name, k.project_name, k.user_id, u.email,
		       k.monthly_budget, k.budget_used, k.budget_alert_pct, k.budget_alerted
		FROM api_keys k JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = ? AND k.monthly_budget IS NOT NULL`, apiKeyHash).Scan(&row).Error
	if err != nil || row.MonthlyBudget == nil {
		return
	}

	budget := *row.MonthlyBudget
	used := row.BudgetUsed
	pct := used / budget * 100

	// 100% 超额 → 直接禁用 key
	if used >= budget {
		h.db.Exec("UPDATE api_keys SET is_active = false WHERE id = ?", row.ID)
		if h.alerter != nil {
			projName := ""
			if row.ProjectName != nil {
				projName = *row.ProjectName
			}
			go h.alerter.Send(fmt.Sprintf("⛔ <b>API Key 预算超额已禁用</b>\n\n<b>用户:</b> %s\n<b>项目:</b> %s\n<b>Key:</b> %s\n<b>预算:</b> ¥%.2f\n<b>已用:</b> ¥%.2f (%.1f%%)",
				row.Email, projName, row.Name, budget, used, pct))
		}
		return
	}

	// 达到告警阈值且尚未发过告警 → 发一次告警
	if int(pct) >= row.BudgetAlertPct && !row.BudgetAlerted {
		h.db.Exec("UPDATE api_keys SET budget_alerted = true WHERE id = ?", row.ID)
		if h.alerter != nil {
			projName := ""
			if row.ProjectName != nil {
				projName = *row.ProjectName
			}
			go h.alerter.Send(fmt.Sprintf("⚠️ <b>API Key 预算告警</b>\n\n<b>用户:</b> %s\n<b>项目:</b> %s\n<b>Key:</b> %s\n<b>预算:</b> ¥%.2f\n<b>已用:</b> ¥%.2f (%.1f%%)\n<b>告警阈值:</b> %d%%",
				row.Email, projName, row.Name, budget, used, pct, row.BudgetAlertPct))
		}
	}
}

// getAPIKeyIDPtr 从 gin context 提取 api_key_id，返回 *string 供 logRequest 使用。
func getAPIKeyIDPtr(c *gin.Context) *string {
	if v, ok := c.Get("api_key_id"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return &s
		}
	}
	return nil
}
