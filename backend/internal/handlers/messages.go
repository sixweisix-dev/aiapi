package handlers

import (
	"bufio"
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
	// 精确预检: 估算 prompt token + max_tokens, 算 estimated cost
	priceRow, err := h.billingEngine.GetModelPrice(model.ID.String())
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "pricing lookup failed: " + err.Error(), "type": "internal_error"}})
		return
	}
	estPromptTokens := billing.EstimatePromptTokensFromBytes(bodyBytes)
	maxCompletionTokens := 4096
	if peek.MaxTokens > 0 {
		maxCompletionTokens = peek.MaxTokens
	}
	estimatedCost := billing.CalculateCost(estPromptTokens, maxCompletionTokens, priceRow.InputPrice, priceRow.OutputPrice, priceRow.Multiplier*priceRow.GroupMultiplier)
	if err := h.billingEngine.PreCheckBalance(userIDStr, estimatedCost); err != nil {
		c.JSON(402, gin.H{"error": gin.H{"message": err.Error(), "type": "insufficient_balance"}})
		return
	}

	ch := h.pool.SelectSticky(model.Provider, peek.Model, model.GroupID, userIDStr)
	if ch != nil {
		log.Printf("[messages] route user=%s -> ch=%s name=%s", userIDStr, ch.ID, ch.Name)
	}
	if ch == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "no available upstream channel", "type": "api_error"}})
		return
	}

	// 反向协议路由: 非 anthropic 上游 → messages→chat 转换
	if model.Provider != "anthropic" {
		upstreamModel := peek.Model
		if model.UpstreamName != nil && *model.UpstreamName != "" {
			upstreamModel = *model.UpstreamName
		}
		chatBody, _, err := buildChatBodyFromMessages(bodyBytes, upstreamModel)
		if err != nil {
			c.JSON(400, gin.H{"error": gin.H{"message": "convert request failed: " + err.Error(), "type": "invalid_request_error"}})
			return
		}
		h.handleChatToMessages(c, userIDStr, model, ch, chatBody, peek.Model)
		return
	}

	// 预先 strip Anthropic 独有 content blocks (target model 不是 Claude 时)
	// 上游不识别 thinking/reasoning blocks 会返回 400 "unknown content type" 等错误
	modelLower := strings.ToLower(peek.Model)
	if !strings.HasPrefix(modelLower, "claude") {
		original := bodyBytes
		bodyBytes = stripThinkingBlocks(bodyBytes)
		if len(bodyBytes) != len(original) {
			log.Printf("[messages] pre-strip thinking blocks for non-claude model=%s, saved %d bytes", peek.Model, len(original)-len(bodyBytes))
		}
	}

	baseURL := strings.TrimRight(ch.BaseURL, "/")
	upstreamURL := baseURL + "/v1/messages"
	startTime := time.Now()

	// 如果 model 有 upstream_name 别名, 替换 body 中的 model 字段
	if model.UpstreamName != nil && *model.UpstreamName != "" && *model.UpstreamName != model.Name {
		var bodyMap map[string]interface{}
		if json.Unmarshal(bodyBytes, &bodyMap) == nil {
			bodyMap["model"] = *model.UpstreamName
			if newBody, err := json.Marshal(bodyMap); err == nil {
				bodyBytes = newBody
			}
		}
	}

	// 自动注入 cache_control (如果 channel 配置开启)
	if ch.AutoInjectCache {
		bodyBytes = injectCacheControl(bodyBytes, ch.EnableCache1hBeta)
	}

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

	// 预先发 SSE 注释让 Cloudflare 看到 stream 已开始, 避免 524
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") || peek.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteString(": keep-alive\n\n")
		c.Writer.Flush()
	}

	// 起一个 goroutine 定时发 keep-alive 直到 channel close
	keepAliveDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-keepAliveDone:
				return
			case <-ticker.C:
				if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}()
	defer close(keepAliveDone)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(upstreamReq)

	// 上游失败/thinking-signature 错误 → 剥离 thinking blocks 并切到其他上游重试一次
	needFallback := err != nil
	if !needFallback && resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
		peekBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// 任何 4xx 错误都试 strip+retry (覆盖 "unknown content type" 等非 thinking 字样错误)
		log.Printf("[messages] upstream 4xx (status=%d), will strip+retry. body sample: %.200s", resp.StatusCode, string(peekBody))
		needFallback = true
		_ = peekBody
	}
	if needFallback {
		// 剥离 thinking blocks 并选另一个上游
		cleanedBody := stripThinkingBlocks(bodyBytes)
		altCh := h.pool.Select(model.Provider, peek.Model, model.GroupID)
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
				// 修复 id 前缀: chatcmpl- → msg_ (Anthropic 协议规范)
				chunk = bytes.Replace(chunk, []byte(`"id":"chatcmpl-`), []byte(`"id":"msg_`), -1)
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
		// 修复 id 前缀: chatcmpl- → msg_ (Anthropic 协议规范)
		respBody = bytes.Replace(respBody, []byte(`"id":"chatcmpl-`), []byte(`"id":"msg_`), 1)
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
	effectiveMult := model.Multiplier
	if ch != nil && ch.GroupMultiplier > 0 {
		effectiveMult *= ch.GroupMultiplier
	}
	cost := billing.CalculateCost(promptTokens, completionTokens, model.InputPrice, model.OutputPrice, effectiveMult)
	if cost <= 0 {
		return
	}
	if _, err := h.billingEngine.DeductBalance(userIDStr, cost); err != nil {
		// 余额不足等错误: 不退出, 继续记账 (坏账记录, 后续可追溯/补扣)
		log.Printf("[messages] WARN deduct balance failed (continuing to record billing): %v", err)
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
			if t, _ := bm["type"].(string); t == "thinking" || t == "redacted_thinking" || t == "reasoning" {
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
	effectiveMult := model.Multiplier
	if ch != nil && ch.GroupMultiplier > 0 {
		effectiveMult *= ch.GroupMultiplier
	}
	cost := (inputCost + outputCost) * effectiveMult
	if cost <= 0 {
		return
	}
	totalTokens := totalInput + completionTokens

	if _, err := h.billingEngine.DeductBalance(userIDStr, cost); err != nil {
		// 余额不足等错误: 不退出, 继续记账 (坏账记录, 后续可追溯/补扣)
		log.Printf("[messages] WARN deduct balance failed (continuing to record billing): %v", err)
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

// injectCacheControl 根据 channel 配置自动给 request body 的 system block 加 cache_control.
// 返回修改后 body; 解析失败/不需要修改时返回原 body.
// 长度阈值: system text >= 4000 字符 (≈ 2000 tokens) 才加 cache, 避免无效 cache_creation.
func injectCacheControl(body []byte, useExtended1h bool) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	ttl := "5m"
	if useExtended1h {
		ttl = "1h"
	}
	cacheControl := map[string]interface{}{
		"type": "ephemeral",
		"ttl":  ttl,
	}
	sys, ok := req["system"]
	if !ok {
		return body
	}
	switch v := sys.(type) {
	case string:
		if len(v) < 4000 {
			return body
		}
		req["system"] = []map[string]interface{}{
			{
				"type":          "text",
				"text":          v,
				"cache_control": cacheControl,
			},
		}
	case []interface{}:
		if len(v) == 0 {
			return body
		}
		last, ok := v[len(v)-1].(map[string]interface{})
		if !ok {
			return body
		}
		if _, has := last["cache_control"]; has {
			return body
		}
		text, _ := last["text"].(string)
		if len(text) < 4000 {
			return body
		}
		last["cache_control"] = cacheControl
	default:
		return body
	}
	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

func buildChatBodyFromMessages(bodyBytes []byte, upstreamModel string) ([]byte, int, error) {
	var anth map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &anth); err != nil {
		return nil, 0, err
	}

	ccMsgs := []map[string]interface{}{}

	// system (Anthropic) → role  system (leading message)
	if system, ok := anth["system"]; ok {
		if s, ok := system.(string); ok && s != "" {
			ccMsgs = append(ccMsgs, map[string]interface{}{
				"role":    "system",
				"content": s,
			})
		}
	}

	// messages 欽乊lw
	if msgs, ok := anth["messages"].([]interface{}); ok {
		for _, m := range msgs {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := mm["role"].(string)
			content := mm["content"]
			if str, ok := content.(string); ok {
				ccMsgs = append(ccMsgs, map[string]interface{}{
					"role":    role,
					"content": str,
				})
			} else if parts, ok := content.([]interface{}); ok {
				texts := []string{}
				for _, p := range parts {
					pm, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					pType, _ := pm["type"].(string)
					if pType == "text" {
						if t, ok := pm["text"].(string); ok {
							texts = append(texts, t)
						}
					}
				}
				if len(texts) > 0 {
					ccMsgs = append(ccMsgs, map[string]interface{}{
						"role":    role,
						"content": strings.Join(texts, "\n"),
					})
				}
			}
		}
	}

	cc := map[string]interface{}{
		"model":    upstreamModel,
		"messages": ccMsgs,
		"stream":   true,
		"stream_options": map[string]interface{}{"include_usage": true},
	}

	maxTokens := 0
	if mt, ok := anth["max_tokens"].(float64); ok {
		cc["max_tokens"] = int(mt)
		maxTokens = int(mt)
	}
	if temp, ok := anth["temperature"].(float64); ok {
		cc["temperature"] = temp
	}
	if tp, ok := anth["top_p"].(float64); ok {
		cc["top_p"] = tp
	}

	out, err := json.Marshal(cc)
	return out, maxTokens, err
}

func (h *MessagesHandler) handleChatToMessages(c *gin.Context, userIDStr string, model models.Model, ch *upstream.Channel, chatBody []byte, peekModel string) {
	startTime := time.Now()

	resp, err := h.pool.Do(c.Request.Context(), ch, "POST", "/v1/chat/completions", chatBody)
	if err != nil {
		log.Printf("[msgs-chat] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[msgs-chat] upstream %d: %.300s", resp.StatusCode, string(errBody))
		c.Data(resp.StatusCode, "application/json", errBody)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	// Anthropic msg ID
	msgID := "msg_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24]

	sendSSE := func(event string, payload map[string]interface{}) {
		j, _ := json.Marshal(payload)
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, j)
		flusher.Flush()
	}

	// 发 message_start
	sendSSE("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"model":         peekModel,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	})

	// 发 content_block_start
	sendSSE("content_block_start", map[string]interface{}{
		"type":         "content_block_start",
		"index":        0,
		"content_block": map[string]interface{}{"type": "text", "text": ""},
	})

	var totalPrompt, totalCompletion int

	reader := bufio.NewReaderSize(resp.Body, 65536)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
				if err == io.EOF {
					break
				}
				continue
			}
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if bytes.Equal(data, []byte("[DONE]")) {
				break
			}

			var chunk map[string]interface{}
			if jerr := json.Unmarshal(data, &chunk); jerr != nil {
				if err == io.EOF {
					break
				}
				continue
			}

			// usage
			if u, ok := chunk["usage"].(map[string]interface{}); ok {
				if v, ok := u["prompt_tokens"].(float64); ok {
					totalPrompt = int(v)
				}
				if v, ok := u["completion_tokens"].(float64); ok {
					totalCompletion = int(v)
				}
			}

			// choices[0].delta.content
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				ch0, ok := choices[0].(map[string]interface{})
				if !ok {
					if err == io.EOF {
						break
					}
					continue
				}
				if delta, ok := ch0["delta"].(map[string]interface{}); ok {
					if text, ok := delta["content"].(string); ok && text != "" {
						sendSSE("content_block_delta", map[string]interface{}{
							"type":  "content_block_delta",
							"index": 0,
							"delta": map[string]interface{}{
								"type": "text_delta",
								"text": text,
							},
						})
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[msgs-chat] read err: %v", err)
			break
		}
	}

	// 发 content_block_stop
	sendSSE("content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	})

	// 发 message_delta
	sendSSE("message_delta", map[string]interface{}{
		"type":  "message_delta",
		"delta": map[string]interface{}{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]interface{}{"output_tokens": totalCompletion},
	})

	// 发 message_stop
	sendSSE("message_stop", map[string]interface{}{"type": "message_stop"})

	log.Printf("[msgs-chat] done user=%s model=%s prompt=%d completion=%d duration=%dms", userIDStr[:8], peekModel, totalPrompt, totalCompletion, time.Since(startTime).Milliseconds())

	if totalPrompt > 0 || totalCompletion > 0 {
		h.bill(userIDStr, model, ch, totalPrompt, totalCompletion, startTime, 200)
	}
}
