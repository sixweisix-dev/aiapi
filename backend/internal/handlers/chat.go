package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/adapter"
	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatHandler struct {
	db           *gorm.DB
	pool         *upstream.Pool
	billingEngine *billing.Engine
}

func NewChatHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine) *ChatHandler {
	return &ChatHandler{db: db, pool: pool, billingEngine: be}
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

	// Select upstream channel
	ch := h.pool.Select(provider)
	if ch == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "no available upstream channel for " + provider, "type": "service_unavailable"}})
		return
	}

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

	resp, err := h.pool.DoWithRetry(ctx, ch, "POST", upstreamPath, upstreamReq)
	if err != nil {
		log.Printf("Upstream request failed: provider=%s model=%s err=%v", provider, req.Model, err)
		h.logRequest(c, userID, nil, modelID, ch, req, 502, 0, 0, 0, 0, startTime, "upstream request failed")
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "upstream_error"}})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logRequest(c, userID, nil, modelID, ch, req, 502, 0, 0, 0, 0, startTime, "failed to read response")
		c.JSON(502, gin.H{"error": gin.H{"message": "failed to read upstream response", "type": "upstream_error"}})
		return
	}

	if resp.StatusCode >= 400 {
		log.Printf("Upstream error: provider=%s status=%d body=%s", provider, resp.StatusCode, string(body))
		h.logRequest(c, userID, nil, modelID, ch, req, resp.StatusCode, 0, 0, 0, 0, startTime, "upstream error")
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
		h.logRequest(c, userID, nil, modelID, ch, req, 502, 0, 0, 0, 0, startTime, "response conversion failed")
		c.JSON(502, gin.H{"error": gin.H{"message": "response conversion failed", "type": "upstream_error"}})
		return
	}

	// Track usage and billing
	if openAIResp.Usage != nil {
		promptTokens := openAIResp.Usage.PromptTokens
		completionTokens := openAIResp.Usage.CompletionTokens
		totalTokens := openAIResp.Usage.TotalTokens
		cost := billing.CalculateCost(promptTokens, completionTokens, priceRow.InputPrice, priceRow.OutputPrice, priceRow.Multiplier)

		go h.postProcess(userID, modelID, ch, req.Model, provider, apiKeyHash, promptTokens, completionTokens, totalTokens, cost, nil, startTime, resp.StatusCode)
	}

	c.JSON(200, openAIResp)
}

func (h *ChatHandler) handleStream(c *gin.Context, userID string, req *adapter.OpenAIRequest, chatAdapter adapter.ChatAdapter, ch *upstream.Channel, upstreamReq []byte, upstreamPath, modelID, provider string, priceRow *billing.PriceRow, apiKeyHash string, startTime time.Time) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	resp, err := h.pool.DoWithRetry(ctx, ch, "POST", upstreamPath, upstreamReq)
	if err != nil {
		log.Printf("Upstream stream request failed: provider=%s model=%s err=%v", provider, req.Model, err)
		h.logRequest(c, userID, nil, modelID, ch, req, 502, 0, 0, 0, 0, startTime, "upstream request failed")
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "upstream_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Upstream stream error: provider=%s status=%d body=%s", provider, resp.StatusCode, string(body))
		h.logRequest(c, userID, nil, modelID, ch, req, resp.StatusCode, 0, 0, 0, 0, startTime, "upstream stream error")
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
			if len(chunk.Choices) > 0 && strings.Contains(chunk.Choices[0].Delta.Content, "%%USAGE%%") {
				usageStr := strings.TrimPrefix(chunk.Choices[0].Delta.Content, "%%USAGE%%:")
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
	go h.postProcess(userID, modelID, ch, req.Model, provider, apiKeyHash, totalPromptTokens, totalCompletionTokens, totalTokens, cost, nil, startTime, http.StatusOK)
}

// postProcess handles billing deduction, request logging, and stats update after a request completes.
func (h *ChatHandler) postProcess(userID, modelID string, ch *upstream.Channel, modelName, provider, apiKeyHash string, promptTokens, completionTokens, totalTokens int, cost float64, errMsg *string, startTime time.Time, statusCode int) {
	durationMs := time.Since(startTime).Milliseconds()

	// Deduct balance
	if cost > 0 {
		if _, err := h.billingEngine.DeductBalance(userID, cost); err != nil {
			log.Printf("Balance deduction failed: user=%s model=%s cost=%.8f err=%v", userID, modelName, cost, err)
		}
	}

	parsedUserID, _ := uuid.Parse(userID)

	// Build request log entry
	reqLog := &models.Request{
		UserID:           parsedUserID,
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
		h.db.Exec("UPDATE api_keys SET total_used = total_used + 1, last_used_at = NOW() WHERE key_hash = ?", apiKeyHash)
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

	reqLog := &models.Request{
		UserID:           parsedUserID,
		ModelID:          parseUUID(modelID),
		UpstreamChannelID: parseUUIDPtr(ch.ID),
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
