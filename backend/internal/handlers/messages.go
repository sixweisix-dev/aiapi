package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/channelmetrics"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessagesHandler struct {
	db            *gorm.DB
	pool          *upstream.Pool
	billingEngine *billing.Engine
	tracker       *channelmetrics.Tracker
}

func NewMessagesHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine, tracker *channelmetrics.Tracker) *MessagesHandler {
	return &MessagesHandler{db: db, pool: pool, billingEngine: be, tracker: tracker}
}

type anthropicPassthruReq struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Stream    bool   `json:"stream"`
}

func (h *MessagesHandler) Handle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if userIDStr == "" {
		c.JSON(401, gin.H{"error": gin.H{"message": "authentication required", "type": "auth_error"}})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "failed to read request body", "type": "invalid_request_error"}})
		return
	}

	var peek anthropicPassthruReq
	if err := json.Unmarshal(bodyBytes, &peek); err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid JSON", "type": "invalid_request_error"}})
		return
	}
	if peek.Model == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "model is required", "type": "invalid_request_error"}})
		return
	}

	var model models.Model
	if err := h.db.Where("name = ? AND is_enabled = true", peek.Model).First(&model).Error; err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": fmt.Sprintf("model not found or disabled: %s", peek.Model), "type": "invalid_request_error"}})
		return
	}

	var user models.User
	if err := h.db.First(&user, "id = ?", userIDStr).Error; err != nil {
		c.JSON(401, gin.H{"error": gin.H{"message": "user not found", "type": "auth_error"}})
		return
	}
	if user.Balance <= 0 {
		c.JSON(402, gin.H{"error": gin.H{"message": "insufficient balance", "type": "billing_error"}})
		return
	}

	ch := h.pool.SelectSticky(model.Provider, userIDStr)
	if ch != nil {
		log.Printf("[messages] route user=%s -> ch=%s name=%s", userIDStr, ch.ID, ch.Name)
	}
	if ch == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "no available upstream channel", "type": "api_error"}})
		return
	}

	baseURL := strings.TrimRight(ch.BaseURL, "/")
	upstreamURL := baseURL + "/v1/messages"
	startTime := time.Now()

	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "failed to create upstream request", "type": "api_error"}})
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("x-api-key", ch.APIKey)
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	// 根据 channel 配置决定是否注入 1h cache beta + 透传用户 beta
	ab := c.GetHeader("anthropic-beta")
	if ch.EnableCache1hBeta {
		cacheBeta := "extended-cache-ttl-2025-04-11"
		if ab == "" {
			ab = cacheBeta
		} else if !strings.Contains(ab, "extended-cache-ttl") {
			ab = ab + "," + cacheBeta
		}
	}
	if ab != "" {
		upstreamReq.Header.Set("anthropic-beta", ab)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(upstreamReq)

	// 上游失败/thinking-signature 错误 → 剥离 thinking blocks 并切到其他上游重试一次
	needFallback := err != nil
	if !needFallback && resp != nil && resp.StatusCode == 400 {
		peekBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if bytes.Contains(peekBody, []byte("thinking")) || bytes.Contains(peekBody, []byte("signature")) {
			needFallback = true
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(peekBody))
		}
	}
	if needFallback {
		// 剥离 thinking blocks 并选另一个上游
		cleanedBody := stripThinkingBlocks(bodyBytes)
		altCh := h.pool.Select(model.Provider)
		if altCh != nil && altCh.ID != ch.ID {
			log.Printf("[messages] sticky failed, fallback %s -> %s, stripped thinking", ch.ID, altCh.ID)
			ch = altCh
			altURL := strings.TrimRight(ch.BaseURL, "/") + "/v1/messages"
			retryReq, _ := http.NewRequest("POST", altURL, bytes.NewReader(cleanedBody))
			retryReq.Header.Set("Content-Type", "application/json")
			retryReq.Header.Set("x-api-key", ch.APIKey)
			retryReq.Header.Set("anthropic-version", "2023-06-01")
			// retry 也用 channel 配置决定
			ab2 := c.GetHeader("anthropic-beta")
			if ch.EnableCache1hBeta {
				cacheBeta := "extended-cache-ttl-2025-04-11"
				if ab2 == "" {
					ab2 = cacheBeta
				} else if !strings.Contains(ab2, "extended-cache-ttl") {
					ab2 = ab2 + "," + cacheBeta
				}
			}
			if ab2 != "" {
				retryReq.Header.Set("anthropic-beta", ab2)
			}
			resp, err = client.Do(retryReq)
		}
	}

	if err != nil {
		log.Printf("[messages] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Header(k, v)
		}
	}
	c.Status(resp.StatusCode)

	if peek.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")

		var promptTokens, completionTokens, cacheCreate, cacheRead int
		flusher, canFlush := c.Writer.(http.Flusher)

		// SSE 按行解析 (用 bufio.Scanner 处理跨 chunk 行边界)
		buf := make([]byte, 0, 8192)
		tmp := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(tmp)
			if n > 0 {
				chunk := tmp[:n]
				c.Writer.Write(chunk)
				if canFlush {
					flusher.Flush()
				}
				// 累积到行缓冲, 按 \n 切出完整 SSE 行
				buf = append(buf, chunk...)
				for {
					idx := bytes.IndexByte(buf, '\n')
					if idx < 0 {
						break
					}
					line := buf[:idx]
					buf = buf[idx+1:]
					line = bytes.TrimSpace(line)
					if !bytes.HasPrefix(line, []byte("data:")) {
						continue
					}
					jsonPart := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
					if len(jsonPart) == 0 || bytes.Equal(jsonPart, []byte("[DONE]")) {
						continue
					}
					var ev map[string]interface{}
					if err := json.Unmarshal(jsonPart, &ev); err != nil {
						continue
					}
					evType, _ := ev["type"].(string)
					// message_start: usage.input_tokens 在 message.usage 里
					if evType == "message_start" {
						if msg, ok := ev["message"].(map[string]interface{}); ok {
							if u, ok := msg["usage"].(map[string]interface{}); ok {
								if v, ok := u["input_tokens"].(float64); ok {
									promptTokens = int(v)
								}
								if v, ok := u["output_tokens"].(float64); ok {
									completionTokens = int(v)
								}
								if v, ok := u["cache_creation_input_tokens"].(float64); ok {
									cacheCreate = int(v)
								}
								if v, ok := u["cache_read_input_tokens"].(float64); ok {
									cacheRead = int(v)
								}
							}
						}
					}
					// message_delta: usage.output_tokens 是最终累计值
					if evType == "message_delta" {
						if u, ok := ev["usage"].(map[string]interface{}); ok {
							if v, ok := u["output_tokens"].(float64); ok {
								completionTokens = int(v)
							}
							if v, ok := u["input_tokens"].(float64); ok {
								promptTokens = int(v)
							}
						}
					}
				}
			}
			if readErr == io.EOF || readErr != nil {
				break
			}
		}
		log.Printf("[messages] stream done: prompt=%d completion=%d cacheCreate=%d cacheRead=%d", promptTokens, completionTokens, cacheCreate, cacheRead)
		h.billWithCache(userIDStr, model, ch, promptTokens, completionTokens, cacheCreate, cacheRead, startTime, resp.StatusCode)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		c.Writer.Write(respBody)

		var anthropicResp struct {
			Usage struct {
				InputTokens              int `json:"input_tokens"`
				OutputTokens             int `json:"output_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		}
		promptTokens, completionTokens, cacheCreate, cacheRead := 0, 0, 0, 0
		if resp.StatusCode == 200 {
			if json.Unmarshal(respBody, &anthropicResp) == nil {
				promptTokens = anthropicResp.Usage.InputTokens
				completionTokens = anthropicResp.Usage.OutputTokens
				cacheCreate = anthropicResp.Usage.CacheCreationInputTokens
				cacheRead = anthropicResp.Usage.CacheReadInputTokens
			}
		}
		h.billWithCache(userIDStr, model, ch, promptTokens, completionTokens, cacheCreate, cacheRead, startTime, resp.StatusCode)
	}
}

func (h *MessagesHandler) bill(userIDStr string, model models.Model, ch *upstream.Channel, promptTokens, completionTokens int, startTime time.Time, statusCode int) {
	if statusCode != 200 || (promptTokens == 0 && completionTokens == 0) {
		return
	}
	totalTokens := promptTokens + completionTokens
	cost := billing.CalculateCost(promptTokens, completionTokens, model.InputPrice, model.OutputPrice, model.Multiplier)
	if cost <= 0 {
		return
	}
	if _, err := h.billingEngine.DeductBalance(userIDStr, cost); err != nil {
		log.Printf("[messages] deduct balance error: %v", err)
		return
	}
	requestID := fmt.Sprintf("msg-%d", startTime.UnixNano())
	note := fmt.Sprintf("anthropic-native model=%s ch=%s", model.Name, ch.ID)
	if _, err := h.billingEngine.RecordBilling(userIDStr, model.ID.String(), requestID, promptTokens, completionTokens, totalTokens, cost, note); err != nil {
		log.Printf("[messages] record billing error: %v", err)
	}

	// 记入 requests 表
	userUUID, _ := uuid.Parse(userIDStr)
	chUUID, _ := uuid.Parse(ch.ID)
	durationMs := time.Since(startTime).Milliseconds()
	req := &models.Request{
		UserID:            userUUID,
		ModelID:           model.ID,
		UpstreamChannelID: &chUUID,
		Path:              "/v1/messages",
		Method:            "POST",
		StatusCode:        statusCode,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       totalTokens,
		Cost:              cost,
		DurationMs:        int(durationMs),
	}
	if err := h.db.Create(req).Error; err != nil {
		log.Printf("[messages] record request error: %v", err)
	}
}

// stripThinkingBlocks 把请求 body 里 messages.*.content 中的 thinking 块剥离
func stripThinkingBlocks(body []byte) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		return body
	}
	for i, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := mm["content"].([]interface{})
		if !ok {
			continue
		}
		cleaned := make([]interface{}, 0, len(content))
		for _, blk := range content {
			bm, ok := blk.(map[string]interface{})
			if !ok {
				cleaned = append(cleaned, blk)
				continue
			}
			if t, _ := bm["type"].(string); t == "thinking" || t == "redacted_thinking" {
				continue
			}
			cleaned = append(cleaned, blk)
		}
		mm["content"] = cleaned
		msgs[i] = mm
	}
	req["messages"] = msgs
	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}


// billWithCache 按 prompt/completion/cache_create/cache_read 分别计价
// 价格规则 (Anthropic):
//   prompt input  = inputPrice
//   cache_create  = inputPrice * 1.25
//   cache_read    = inputPrice * 0.1
//   completion    = outputPrice
func (h *MessagesHandler) billWithCache(userIDStr string, model models.Model, ch *upstream.Channel, promptTokens, completionTokens, cacheCreate, cacheRead int, startTime time.Time, statusCode int) {
	if statusCode != 200 {
		return
	}
	totalInput := promptTokens + cacheCreate + cacheRead
	if totalInput == 0 && completionTokens == 0 {
		return
	}
	// 输入成本: prompt + cache_create*1.25 + cache_read*0.1, 全部除以 1e6 (因为 InputPrice 是 per 1M token)
	inputCost := (float64(promptTokens) + float64(cacheCreate)*1.25 + float64(cacheRead)*0.1) * model.InputPrice / 1000.0
	outputCost := float64(completionTokens) * model.OutputPrice / 1000.0
	cost := (inputCost + outputCost) * model.Multiplier
	if cost <= 0 {
		return
	}
	totalTokens := totalInput + completionTokens

	if _, err := h.billingEngine.DeductBalance(userIDStr, cost); err != nil {
		log.Printf("[messages] deduct balance error: %v", err)
		return
	}
	requestID := fmt.Sprintf("msg-%d", startTime.UnixNano())
	note := fmt.Sprintf("anthropic-native model=%s ch=%s cache(create=%d,read=%d)", model.Name, ch.ID, cacheCreate, cacheRead)
	if _, err := h.billingEngine.RecordBilling(userIDStr, model.ID.String(), requestID, totalInput, completionTokens, totalTokens, cost, note); err != nil {
		log.Printf("[messages] record billing error: %v", err)
	}

	// 记入 requests 表
	userUUID, _ := uuid.Parse(userIDStr)
	chUUID, _ := uuid.Parse(ch.ID)
	durationMs := time.Since(startTime).Milliseconds()
	req := &models.Request{
		UserID:            userUUID,
		ModelID:           model.ID,
		UpstreamChannelID: &chUUID,
		Path:              "/v1/messages",
		Method:            "POST",
		StatusCode:        statusCode,
		PromptTokens:      totalInput,
		CompletionTokens:  completionTokens,
		TotalTokens:       totalTokens,
		Cost:              cost,
		DurationMs:        int(durationMs),
	}
	if err := h.db.Create(req).Error; err != nil {
		log.Printf("[messages] record request error: %v", err)
	}

	// 上报渠道指标 + 审计
	if h.tracker != nil {
		h.tracker.RecordSuccess(ch.ID, cost, cacheRead, promptTokens+cacheRead, durationMs)
		h.tracker.CheckAutoDedicate(ch.ID, userIDStr, cost)
		h.tracker.AuditBigCost(userIDStr, model.Name, ch.ID, cost, promptTokens+cacheCreate+cacheRead, completionTokens)
		h.tracker.AuditHighRPM(userIDStr)
		h.tracker.AuditFailureRate(userIDStr, statusCode)
	}
}
