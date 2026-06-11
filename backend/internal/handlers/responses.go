// /home/ubuntu/token-api/backend/internal/handlers/responses.go
//
// OpenAI Responses API 适配层 - Phase 1 + Phase 2 (含流式)
// 让 OpenAI Codex CLI 等基于 Responses API 的客户端能接入 transitai
//
// 端点:
//   POST   /v1/responses           创建 response (含 stream)
//   GET    /v1/responses/:id       查询 response (Redis 24h TTL)
//   POST   /v1/responses/:id/cancel  取消
//   DELETE /v1/responses/:id       删除
package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"math/rand"
	"time"
"sync/atomic"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/channelmetrics"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ============================================================
// Handler struct
// ============================================================

type ResponsesHandler struct {
	db            *gorm.DB
	pool          *upstream.Pool
	billingEngine *billing.Engine
	tracker       *channelmetrics.Tracker
	redis         *redis.Client
}

func NewResponsesHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine, tracker *channelmetrics.Tracker, rdb *redis.Client) *ResponsesHandler {
	return &ResponsesHandler{db: db, pool: pool, billingEngine: be, tracker: tracker, redis: rdb}
}

// ============================================================
// Request/Response types
// ============================================================

type responseCreateRequest struct {
	Model              string                `json:"model"`
	Input              json.RawMessage       `json:"input"`
	Instructions       string                `json:"instructions,omitempty"`
	MaxOutputTokens    int                   `json:"max_output_tokens,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"top_p,omitempty"`
	Tools              []responseTool        `json:"tools,omitempty"`
	ToolChoice         json.RawMessage       `json:"tool_choice,omitempty"`
	Stream             bool                  `json:"stream,omitempty"`
	Reasoning          *responseReasoningCfg `json:"reasoning,omitempty"`
	Store              *bool                 `json:"store,omitempty"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
	ParallelToolCalls  *bool                 `json:"parallel_tool_calls,omitempty"`
}

type responseTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type responseReasoningCfg struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type responseObject struct {
	ID                 string                 `json:"id"`
	Object             string                 `json:"object"`
	CreatedAt          int64                  `json:"created_at"`
	Status             string                 `json:"status"`
	Error              *responseError         `json:"error"`
	IncompleteDetails  interface{}            `json:"incomplete_details"`
	Model              string                 `json:"model"`
	Output             []outputItem           `json:"output"`
	Usage              *responseUsage         `json:"usage,omitempty"`
	Reasoning          map[string]interface{} `json:"reasoning,omitempty"`
	PreviousResponseID *string               `json:"previous_response_id"`
	Tools              []responseTool         `json:"tools"`
	ParallelToolCalls  bool                   `json:"parallel_tool_calls"`
	ToolChoice         interface{}            `json:"tool_choice,omitempty"`
	Temperature        interface{}            `json:"temperature"`
	TopP               interface{}            `json:"top_p"`
	Truncation         string                 `json:"truncation,omitempty"`
	User               interface{}            `json:"user"`
	Metadata           map[string]string      `json:"metadata"`
Instructions       string                 `json:"instructions"`
PromptCacheKey     string                 `json:"prompt_cache_key,omitempty"`
PromptCacheRetention string                 `json:"prompt_cache_retention,omitempty"`
SafetyIdentifier   string                 `json:"safety_identifier,omitempty"`
ServiceTier        string                 `json:"service_tier,omitempty"`
Text               map[string]interface{} `json:"text,omitempty"`
ToolUsage          map[string]interface{} `json:"tool_usage,omitempty"`
TopLogprobs        int                    `json:"top_logprobs"`
Store              bool                   `json:"store"`
Background         bool                   `json:"background"`
CompletedAt        int64                  `json:"completed_at,omitempty"`
FrequencyPenalty   float64                `json:"frequency_penalty"`
PresencePenalty    float64                `json:"presence_penalty"`
MaxToolCalls       interface{}            `json:"max_tool_calls"`
Moderation         interface{}            `json:"moderation"`
MaxOutputTokens    interface{}            `json:"max_output_tokens"`
}

type outputItem struct {
	Type      string             `json:"type"`
	ID        string             `json:"id,omitempty"`
	Role      string             `json:"role,omitempty"`
	Content   []contentPart      `json:"content,omitempty"`
Phase     string             `json:"phase,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Summary   []reasoningSummary `json:"summary,omitempty"`
	Status    string             `json:"status,omitempty"`
}

type contentPart struct {
	Type        string        `json:"type"`
	Text        string        `json:"text,omitempty"`
	Annotations []interface{} `json:"annotations"`
Logprobs    []interface{} `json:"logprobs"`
}

type reasoningSummary struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type responseUsage struct {
	InputTokens         int                    `json:"input_tokens"`
	OutputTokens        int                    `json:"output_tokens"`
	TotalTokens         int                    `json:"total_tokens"`
	InputTokensDetails  map[string]interface{} `json:"input_tokens_details,omitempty"`
	OutputTokensDetails map[string]interface{} `json:"output_tokens_details,omitempty"`
}

// ============================================================
// Endpoint 1: POST /v1/responses (Phase 1 非流式 + Phase 2 流式)
// ============================================================

func (h *ResponsesHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if userIDStr == "" {
		c.JSON(401, gin.H{"error": gin.H{"message": "authentication required", "type": "auth_error"}})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "failed to read body", "type": "invalid_request_error"}})
		return
	}

	var req responseCreateRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid JSON: " + err.Error(), "type": "invalid_request_error"}})
		return
	}
	if req.Model == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "model is required", "type": "invalid_request_error"}})
		return
	}

	// 校验 model
	var model models.Model
	if err := h.db.Where("name = ? AND is_enabled = true", req.Model).First(&model).Error; err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": fmt.Sprintf("model not found or disabled: %s", req.Model), "type": "invalid_request_error"}})
		return
	}

	// 校验 user balance
	var user models.User
	if err := h.db.First(&user, "id = ?", userIDStr).Error; err != nil {
		c.JSON(401, gin.H{"error": gin.H{"message": "user not found", "type": "auth_error"}})
		return
	}
	if user.Balance <= 0 {
		c.JSON(402, gin.H{"error": gin.H{"message": "insufficient balance", "type": "billing_error"}})
		return
	}


	var prevMessages []map[string]interface{}
	if req.PreviousResponseID != "" {
		prevMessages = h.loadPreviousMessages(req.PreviousResponseID)
		log.Printf("[responses] prev_resp_id=%s loaded %d prev messages", req.PreviousResponseID, len(prevMessages))
	}
	// 转 ResponsesAPI -> ChatCompletions body
	ccBody, err := buildChatCompletionsBody(&req, &model, prevMessages)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "failed to build upstream request: " + err.Error(), "type": "invalid_request_error"}})
		return
	}

	// 选 channel
	ch := h.pool.SelectSticky(model.Provider, req.Model, model.GroupID, userIDStr)
	if ch == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "no available upstream channel", "type": "api_error"}})
		return
	}

	if req.Stream {
		log.Printf("[responses-stream] route user=%s -> ch=%s name=%s model=%s", userIDStr, ch.ID, ch.Name, req.Model)
		h.handleStream(c, userIDStr, &req, &model, ch, bodyBytes, ccBody)
		return
	}

	log.Printf("[responses] route user=%s -> ch=%s name=%s model=%s", userIDStr, ch.ID, ch.Name, req.Model)
	h.handleNonStream(c, userIDStr, &req, &model, ch, ccBody)
}

// ============================================================
// 非流式处理 (Phase 1, 已通过测试)
// ============================================================

func (h *ResponsesHandler) handleNonStream(c *gin.Context, userIDStr string, req *responseCreateRequest, model *models.Model, ch *upstream.Channel, ccBody []byte) {
	startTime := time.Now()
resp, err := h.pool.Do(c.Request.Context(), ch, "POST", "/v1/chat/completions", ccBody)
	if err != nil {
		log.Printf("[responses] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		log.Printf("[responses] upstream %d: %.300s", resp.StatusCode, string(respBody))
		c.Data(resp.StatusCode, "application/json", respBody)
		return
	}

	var ccResp map[string]interface{}
	if err := json.Unmarshal(respBody, &ccResp); err != nil {
		c.JSON(502, gin.H{"error": gin.H{"message": "failed to parse upstream response", "type": "api_error"}})
		return
	}

	respObj := convertChatToResponseObject(ccResp, req)

	var promptTokens, completionTokens int
	if respObj.Usage != nil {
		promptTokens = respObj.Usage.InputTokens
		completionTokens = respObj.Usage.OutputTokens
	}
	h.bill(userIDStr, *model, ch, promptTokens, completionTokens, startTime, resp.StatusCode)

	if h.redis != nil {
		go h.storeInRedis(respObj)
	}

	c.JSON(200, respObj)
}

// ============================================================
// 流式处理 (Phase 2 - 核心)
//
// 上游 ChatCompletions SSE → Responses API SSE 转换
//
// 上游格式 (OpenAI):
//   data: {"choices":[{"delta":{"content":"Hello"}}]}
//   data: {"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"..."}}]}}]}
//   data: [DONE]
//
// 输出格式 (Responses API):
//   event: response.created
//   event: response.in_progress
//   event: response.output_item.added (item: message or function_call or reasoning)
//   event: response.content_part.added (for message)
//   event: response.output_text.delta (for message text)
//   event: response.output_text.done
//   event: response.content_part.done
//   event: response.output_item.done
//   event: response.completed
// ============================================================

type streamState struct {
	respObj          *responseObject
	outputIndex      int    // 当前 output item 的 index
	contentIndex     int    // 当前 content part 的 index
	currentItemType  string // "message" / "function_call" / "reasoning"
	currentItemID    string // 当前 output item ID
	currentTextBuf   strings.Builder
	currentToolName  string
	currentToolArgs  strings.Builder
	currentToolCallID string
	currentReasoning strings.Builder
	itemStarted      bool
	contentStarted   bool
	totalPrompt      int
	totalCompletion  int
	totalUpstream    int
}

func (h *ResponsesHandler) handleStream(c *gin.Context, userIDStr string, req *responseCreateRequest, model *models.Model, ch *upstream.Channel, bodyBytes []byte, ccBody []byte) {
	startTime := time.Now()

	// Provider 路由: 非 openai 走 chat→responses 转换
	if ch.Provider == "anthropic" {
		anthBody, _ := buildAnthropicMessagesBody(req, model, nil)
		h.handleStreamMessagesToResponses(c, userIDStr, req, model, ch, anthBody, startTime)
		return
	}
	if ch.Provider != "openai" && ch.Provider != "multi_aggregator" {
		h.handleStreamChatToResponses(c, userIDStr, req, model, ch, ccBody, startTime)
		return
	}

	// 强制 stream=true 到 body
	var reqMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid body", "type": "invalid_request_error"}})
		return
	}
	reqMap["stream"] = true
	upstreamBody, _ := json.Marshal(reqMap)

	// 调上游 /v1/responses (直通不变)
	resp, err := h.pool.Do(c.Request.Context(), ch, "POST", "/v1/responses", upstreamBody)
	if err != nil {
		log.Printf("[responses-stream] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[responses-stream] upstream %d: %.300s", resp.StatusCode, string(errBody))
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

	// 字节透传 + 先扫描繇第 usage
	var totalPrompt, totalCompletion int
	respID := ""
	reader := bufio.NewReaderSize(resp.Body, 65536)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			c.Writer.Write(line)
			flusher.Flush()

			// 尝试 parse usage (response.completed)
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("data:")) {
				payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
				var event map[string]interface{}
				if json.Unmarshal(payload, &event) == nil {
					if respObj, ok := event["response"].(map[string]interface{}); ok {
						if id, ok := respObj["id"].(string); ok {
							respID = id
						}
						if usage, ok := respObj["usage"].(map[string]interface{}); ok {
							if v, ok := usage["input_tokens"].(float64); ok {
								totalPrompt = int(v)
							}
							if v, ok := usage["output_tokens"].(float64); ok {
								totalCompletion = int(v)
							}
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	log.Printf("[responses-stream] done user=%s model=%s prompt=%d completion=%d duration=%dms respID=%s", userIDStr[:8], req.Model, totalPrompt, totalCompletion, durationMs, respID)

	if totalPrompt > 0 || totalCompletion > 0 {
		h.bill(userIDStr, *model, ch, totalPrompt, totalCompletion, startTime, 200)
	}
}

func processChunk(w io.Writer, flusher http.Flusher, chunk map[string]interface{}, state *streamState) {
	// 处理 usage (通常在最后一个 chunk)
	if u, ok := chunk["usage"].(map[string]interface{}); ok {
		if v, ok := u["prompt_tokens"].(float64); ok {
			state.totalPrompt = int(v)
		}
		if v, ok := u["completion_tokens"].(float64); ok {
			state.totalCompletion = int(v)
		}
		if v, ok := u["total_tokens"].(float64); ok {
			state.totalUpstream = int(v)
		}
	}

	choices, _ := chunk["choices"].([]interface{})
	if len(choices) == 0 {
		return
	}
	choice, _ := choices[0].(map[string]interface{})
	delta, _ := choice["delta"].(map[string]interface{})
	if delta == nil {
		return
	}

	// 1. reasoning_content (gpt-5.5 等 reasoning 模型)
	if rsn, ok := delta["reasoning_content"].(string); ok && rsn != "" {
		handleReasoningDelta(w, flusher, rsn, state)
	}

	// 2. tool_calls 增量
	if tcs, ok := delta["tool_calls"].([]interface{}); ok && len(tcs) > 0 {
		for _, tc := range tcs {
			tcm, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			handleToolCallDelta(w, flusher, tcm, state)
		}
	}

	// 3. content (text)
	if content, ok := delta["content"].(string); ok && content != "" {
		handleTextDelta(w, flusher, content, state)
	}

	// 处理 finish_reason (本 choice 完成)
	if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
		// finish_reason 触发 — 但不在这里结束 (由 [DONE] 结束)
		_ = fr
	}
}

// 处理 reasoning_content 增量
func handleReasoningDelta(w io.Writer, flusher http.Flusher, text string, state *streamState) {
	// 如果当前不是 reasoning item, 先关闭旧的, 开新的
	if state.currentItemType != "reasoning" {
		closeCurrentItem(w, flusher, state)

		state.currentItemID = "rs_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24]
		state.currentItemType = "reasoning"
		state.currentReasoning.Reset()
		state.itemStarted = true

		// response.output_item.added (reasoning)
		writeSSEEvent(w, "response.output_item.added", map[string]interface{}{
			"output_index": state.outputIndex,
			"item": outputItem{
				Type:    "reasoning",
				ID:      state.currentItemID,
				Summary: []reasoningSummary{},
				Status:  "in_progress",
			},
		})
		flusher.Flush()

		// response.reasoning_summary_part.added
		writeSSEEvent(w, "response.reasoning_summary_part.added", map[string]interface{}{
			"item_id":       state.currentItemID,
			"output_index":  state.outputIndex,
			"summary_index": 0,
			"part": map[string]interface{}{
				"type": "summary_text",
				"text": "",
			},
		})
		flusher.Flush()
	}

	state.currentReasoning.WriteString(text)

	// response.reasoning_summary_text.delta
	writeSSEEvent(w, "response.reasoning_summary_text.delta", map[string]interface{}{
		"item_id":       state.currentItemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"delta":         text,
	})
	flusher.Flush()
}

// 处理 tool_calls 增量
func handleToolCallDelta(w io.Writer, flusher http.Flusher, tcm map[string]interface{}, state *streamState) {
	fn, _ := tcm["function"].(map[string]interface{})
	name, _ := fn["name"].(string)
	args, _ := fn["arguments"].(string)
	tcID, _ := tcm["id"].(string)

	// 如果是新工具 (有 name) 或当前不是 function_call → 开新 item
	isNewTool := (name != "" && state.currentToolName != name) || state.currentItemType != "function_call"

	if isNewTool {
		closeCurrentItem(w, flusher, state)

		state.currentItemID = "fc_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24]
		state.currentItemType = "function_call"
		state.currentToolName = name
		state.currentToolCallID = tcID
		state.currentToolArgs.Reset()
		state.itemStarted = true

		writeSSEEvent(w, "response.output_item.added", map[string]interface{}{
			"output_index": state.outputIndex,
			"item": outputItem{
				Type:      "function_call",
				ID:        state.currentItemID,
				CallID:    tcID,
				Name:      name,
				Arguments: "",
				Status:    "in_progress",
			},
		})
		flusher.Flush()
	}

	if args != "" {
		state.currentToolArgs.WriteString(args)

		writeSSEEvent(w, "response.function_call_arguments.delta", map[string]interface{}{
			"item_id":      state.currentItemID,
			"output_index": state.outputIndex,
			"delta":        args,
		})
		flusher.Flush()
	}
}

// 处理 text content 增量
func handleTextDelta(w io.Writer, flusher http.Flusher, text string, state *streamState) {
	// 如果当前不是 message item, 先关闭旧的, 开新的
	if state.currentItemType != "message" {
		closeCurrentItem(w, flusher, state)

		state.currentItemID = "msg_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
		state.currentItemType = "message"
		state.currentTextBuf.Reset()
		state.contentIndex = 0
		state.itemStarted = true
		state.contentStarted = false

		// response.output_item.added (message)
		writeSSEEvent(w, "response.output_item.added", map[string]interface{}{
			"output_index": state.outputIndex,
			"item": outputItem{
				Type:    "message",
				ID:      state.currentItemID,
				Role:    "assistant",
				Content: []contentPart{},
				Status:  "in_progress",
			},
		})
		flusher.Flush()
	}

	// 如果 content_part 还没开始 (第一次 text)
	if !state.contentStarted {
		writeSSEEvent(w, "response.content_part.added", map[string]interface{}{
			"item_id":       state.currentItemID,
			"output_index":  state.outputIndex,
			"content_index": state.contentIndex,
			"part": contentPart{
				Type:        "output_text",
				Text:        "",
				Annotations: []interface{}{},
Logprobs:    []interface{}{},
			},
		})
		flusher.Flush()
		state.contentStarted = true
	}

	state.currentTextBuf.WriteString(text)

	// response.output_text.delta
	writeSSEEvent(w, "response.output_text.delta", map[string]interface{}{
		"item_id":       state.currentItemID,
		"output_index":  state.outputIndex,
		"content_index": state.contentIndex,
		"delta":         text,
	})
	flusher.Flush()
}

// 关闭当前 output item, 发对应的 done events
func closeCurrentItem(w io.Writer, flusher http.Flusher, state *streamState) {
	if !state.itemStarted {
		return
	}

	switch state.currentItemType {
	case "message":
		fullText := state.currentTextBuf.String()

		if state.contentStarted {
			// response.output_text.done
			writeSSEEvent(w, "response.output_text.done", map[string]interface{}{
				"item_id":       state.currentItemID,
				"output_index":  state.outputIndex,
				"content_index": state.contentIndex,
				"text":          fullText,
			})
			flusher.Flush()

			// response.content_part.done
			writeSSEEvent(w, "response.content_part.done", map[string]interface{}{
				"item_id":       state.currentItemID,
				"output_index":  state.outputIndex,
				"content_index": state.contentIndex,
				"part": contentPart{
					Type:        "output_text",
					Text:        fullText,
					Annotations: []interface{}{},
Logprobs:    []interface{}{},
				},
			})
			flusher.Flush()
		}

		// response.output_item.done
		finalItem := outputItem{
			Type:   "message",
				Phase:  "final_answer",
			ID:     state.currentItemID,
			Role:   "assistant",
			Status: "completed",
			Content: []contentPart{
				{Type: "output_text", Text: fullText, Annotations: []interface{}{}, Logprobs: []interface{}{}},
			},
		}
		writeSSEEvent(w, "response.output_item.done", map[string]interface{}{
			"output_index": state.outputIndex,
			"item":         finalItem,
		})
		flusher.Flush()

		state.respObj.Output = append(state.respObj.Output, finalItem)

	case "function_call":
		fullArgs := state.currentToolArgs.String()

		// response.function_call_arguments.done
		writeSSEEvent(w, "response.function_call_arguments.done", map[string]interface{}{
			"item_id":      state.currentItemID,
			"output_index": state.outputIndex,
			"arguments":    fullArgs,
		})
		flusher.Flush()

		finalItem := outputItem{
			Type:      "function_call",
			ID:        state.currentItemID,
			CallID:    state.currentToolCallID,
			Name:      state.currentToolName,
			Arguments: fullArgs,
			Status:    "completed",
		}
		writeSSEEvent(w, "response.output_item.done", map[string]interface{}{
			"output_index": state.outputIndex,
			"item":         finalItem,
		})
		flusher.Flush()

		state.respObj.Output = append(state.respObj.Output, finalItem)

	case "reasoning":
		fullText := state.currentReasoning.String()

		// response.reasoning_summary_text.done
		writeSSEEvent(w, "response.reasoning_summary_text.done", map[string]interface{}{
			"item_id":       state.currentItemID,
			"output_index":  state.outputIndex,
			"summary_index": 0,
			"text":          fullText,
		})

		// response.reasoning_summary_part.done
		writeSSEEvent(w, "response.reasoning_summary_part.done", map[string]interface{}{
			"item_id":       state.currentItemID,
			"output_index":  state.outputIndex,
			"summary_index": 0,
			"part": map[string]interface{}{
				"type": "summary_text",
				"text": fullText,
			},
		})
		flusher.Flush()
		flusher.Flush()

		finalItem := outputItem{
			Type:   "reasoning",
			ID:     state.currentItemID,
			Status: "completed",
			Summary: []reasoningSummary{
				{Type: "summary_text", Text: fullText},
			},
		}
		writeSSEEvent(w, "response.output_item.done", map[string]interface{}{
			"output_index": state.outputIndex,
			"item":         finalItem,
		})
		flusher.Flush()

		state.respObj.Output = append(state.respObj.Output, finalItem)
	}

	state.outputIndex++
	state.currentItemType = ""
	state.itemStarted = false
	state.contentStarted = false
	state.currentTextBuf.Reset()
	state.currentToolArgs.Reset()
	state.currentReasoning.Reset()
}

// 写一个 SSE event (event: ... \n data: ... \n\n)

func writeSSEEvent(w io.Writer, eventType string, data interface{}) {
	jbytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	var dataMap map[string]interface{}
	if err := json.Unmarshal(jbytes, &dataMap); err != nil || dataMap == nil {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jbytes)
		return
	}
	dataMap["type"] = eventType
	dataMap["sequence_number"] = atomic.AddInt64(&sseSeqCounter, 1)

	if eventType == "response.output_text.delta" {
		if _, ok := dataMap["logprobs"]; !ok {
			dataMap["logprobs"] = []interface{}{}
		}
		if _, ok := dataMap["obfuscation"]; !ok {
			dataMap["obfuscation"] = randomBase62(14)
		}
	}
	if eventType == "response.output_text.done" {
		if _, ok := dataMap["logprobs"]; !ok {
			dataMap["logprobs"] = []interface{}{}
		}
	}
	if eventType == "response.content_part.added" || eventType == "response.content_part.done" {
		if part, ok := dataMap["part"].(map[string]interface{}); ok {
			if _, has := part["annotations"]; !has {
				part["annotations"] = []interface{}{}
			}
			if _, has := part["logprobs"]; !has {
				part["logprobs"] = []interface{}{}
			}
		}
	}

	final, err := json.Marshal(dataMap)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, final)
}

// randomBase62: generate random base62 string for obfuscation field
func randomBase62(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// ============================================================
// Endpoint 2: GET /v1/responses/:id
// ============================================================

func (h *ResponsesHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": gin.H{"message": "id required", "type": "invalid_request_error"}})
		return
	}
	if h.redis == nil {
		c.JSON(404, gin.H{"error": gin.H{"message": "response storage unavailable", "type": "not_found"}})
		return
	}
	ctx := context.Background()
	data, err := h.redis.Get(ctx, "response:"+id).Bytes()
	if err != nil {
		c.JSON(404, gin.H{"error": gin.H{"message": "response not found", "type": "not_found"}})
		return
	}
	c.Data(200, "application/json", data)
}

// ============================================================
// Endpoint 3: POST /v1/responses/:id/cancel
// ============================================================

func (h *ResponsesHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	if h.redis == nil {
		c.JSON(404, gin.H{"error": gin.H{"message": "not found", "type": "not_found"}})
		return
	}
	ctx := context.Background()
	data, err := h.redis.Get(ctx, "response:"+id).Bytes()
	if err != nil {
		c.JSON(404, gin.H{"error": gin.H{"message": "not found", "type": "not_found"}})
		return
	}
	var resp responseObject
	if err := json.Unmarshal(data, &resp); err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "corrupt data", "type": "api_error"}})
		return
	}
	resp.Status = "cancelled"
	newData, _ := json.Marshal(resp)
	h.redis.Set(ctx, "response:"+id, newData, 24*time.Hour)
	c.JSON(200, resp)
}

// ============================================================
// Endpoint 4: DELETE /v1/responses/:id
// ============================================================

func (h *ResponsesHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if h.redis != nil {
		ctx := context.Background()
		h.redis.Del(ctx, "response:"+id)
	}
	c.JSON(200, gin.H{
		"id":      id,
		"object":  "response",
		"deleted": true,
	})
}

// ============================================================
// 转换: ResponsesAPI request -> ChatCompletions request body
// ============================================================

func buildChatCompletionsBody(req *responseCreateRequest, model *models.Model, prevMessages []map[string]interface{}) ([]byte, error) {
	messages := []map[string]interface{}{}

	if req.Instructions != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": req.Instructions,
		})
	}


	if len(prevMessages) > 0 {
		messages = append(messages, prevMessages...)
	}
	if len(req.Input) > 0 {
		var strInput string
		if err := json.Unmarshal(req.Input, &strInput); err == nil {
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": strInput,
			})
		} else {
			var items []map[string]interface{}
			if err := json.Unmarshal(req.Input, &items); err != nil {
				return nil, fmt.Errorf("invalid input format")
			}
			for _, item := range items {
				role, _ := item["role"].(string)
				if role == "" {
					role = "user"
				}
				content := item["content"]

				if str, ok := content.(string); ok {
					messages = append(messages, map[string]interface{}{
						"role":    role,
						"content": str,
					})
				} else if parts, ok := content.([]interface{}); ok {
					ccContent := []map[string]interface{}{}
					for _, p := range parts {
						pm, ok := p.(map[string]interface{})
						if !ok {
							continue
						}
						partType, _ := pm["type"].(string)
						switch partType {
						case "input_text", "text", "output_text":
							ccContent = append(ccContent, map[string]interface{}{
								"type": "text",
								"text": pm["text"],
							})
						case "input_image":
							imgURL := pm["image_url"]
							if s, ok := imgURL.(string); ok {
								ccContent = append(ccContent, map[string]interface{}{
									"type":      "image_url",
									"image_url": map[string]interface{}{"url": s},
								})
							} else if m, ok := imgURL.(map[string]interface{}); ok {
								ccContent = append(ccContent, map[string]interface{}{
									"type":      "image_url",
									"image_url": m,
								})
							}
						}
					}
					if len(ccContent) > 0 {
						messages = append(messages, map[string]interface{}{
							"role":    role,
							"content": ccContent,
						})
					}
				}
			}
		}
	}

	body := map[string]interface{}{
		"messages": messages,
		"stream":   false,
	}

	if model.UpstreamName != nil && *model.UpstreamName != "" {
		body["model"] = *model.UpstreamName
	} else {
		body["model"] = req.Model
	}

	if req.MaxOutputTokens > 0 {
		body["max_tokens"] = req.MaxOutputTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}

	if len(req.Tools) > 0 {
		ccTools := []map[string]interface{}{}
		for _, t := range req.Tools {
			if t.Type == "function" {
				ccTools = append(ccTools, map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  t.Parameters,
					},
				})
			}
		}
		if len(ccTools) > 0 {
			body["tools"] = ccTools
		}
	}

	if len(req.ToolChoice) > 0 {
		body["tool_choice"] = json.RawMessage(req.ToolChoice)
	}

	if req.ParallelToolCalls != nil {
		body["parallel_tool_calls"] = *req.ParallelToolCalls
	}

	return json.Marshal(body)
}

// ============================================================
// 转换: ChatCompletions response (非流式) -> ResponsesAPI response
// ============================================================

func convertChatToResponseObject(ccResp map[string]interface{}, req *responseCreateRequest) *responseObject {
	respID := "resp_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]

	output := []outputItem{}

	choices, _ := ccResp["choices"].([]interface{})
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]interface{})
		message, _ := choice["message"].(map[string]interface{})

		if message != nil {
			if rsn, ok := message["reasoning_content"].(string); ok && strings.TrimSpace(rsn) != "" {
				output = append(output, outputItem{
					Type:   "reasoning",
					ID:     "rs_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24],
					Status: "completed",
					Summary: []reasoningSummary{
						{Type: "summary_text", Text: rsn},
					},
				})
			}

			if tcs, ok := message["tool_calls"].([]interface{}); ok {
				for _, tc := range tcs {
					tcm, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					callID, _ := tcm["id"].(string)
					fn, _ := tcm["function"].(map[string]interface{})
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					output = append(output, outputItem{
						Type:      "function_call",
						ID:        "fc_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24],
						CallID:    callID,
						Name:      name,
						Arguments: args,
						Status:    "completed",
					})
				}
			}

			if content, ok := message["content"].(string); ok && content != "" {
				output = append(output, outputItem{
					Type:   "message",
				Phase:  "final_answer",
					ID:     "msg_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15],
					Role:   "assistant",
					Status: "completed",
					Content: []contentPart{
						{Type: "output_text", Text: content, Annotations: []interface{}{}, Logprobs: []interface{}{}},
					},
				})
			}
		}
	}

	var usage *responseUsage
	if uRaw, ok := ccResp["usage"].(map[string]interface{}); ok {
		pt := toInt(uRaw["prompt_tokens"])
		ct := toInt(uRaw["completion_tokens"])
		tt := toInt(uRaw["total_tokens"])
		usage = &responseUsage{
			InputTokens:  pt,
			OutputTokens: ct,
			TotalTokens:  tt,
		}
	}

	return &responseObject{
		ID:                respID,
		Object:            "response",
		CreatedAt:         time.Now().Unix(),
		Status:            "completed",
		Error:             nil,
		IncompleteDetails: nil,
		Model:             req.Model,
		Output:            output,
		Usage:             usage,
		Tools:             []responseTool{},
		ParallelToolCalls: true,
		Temperature:       1.0,
		TopP:              0.98,
		Metadata:          map[string]string{},
	}
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	}
	return 0
}

// ============================================================
// Redis 存储
// ============================================================

func (h *ResponsesHandler) storeInRedis(resp *responseObject) {
	if h.redis == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.redis.Set(ctx, "response:"+resp.ID, data, 24*time.Hour).Err(); err != nil {
		log.Printf("[responses] redis set failed: %v", err)
	}
}

// ============================================================
// 计费
// ============================================================

func (h *ResponsesHandler) bill(userIDStr string, model models.Model, ch *upstream.Channel, promptTokens, completionTokens int, startTime time.Time, statusCode int) {
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
		log.Printf("[responses] deduct balance error: %v", err)
		return
	}

	requestID := fmt.Sprintf("resp-%d", startTime.UnixNano())
	note := fmt.Sprintf("openai-responses model=%s ch=%s", model.Name, ch.ID)
	if _, err := h.billingEngine.RecordBilling(userIDStr, model.ID.String(), requestID, promptTokens, completionTokens, totalTokens, cost, note); err != nil {
		log.Printf("[responses] record billing error: %v", err)
	}

	userUUID, _ := uuid.Parse(userIDStr)
	chUUID, _ := uuid.Parse(ch.ID)
	durationMs := time.Since(startTime).Milliseconds()
	req := &models.Request{
		UserID:            userUUID,
		ModelID:           model.ID,
		UpstreamChannelID: &chUUID,
		Path:              "/v1/responses",
		Method:            "POST",
		StatusCode:        statusCode,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       totalTokens,
		Cost:              cost,
		DurationMs:        int(durationMs),
	}
	if err := h.db.Create(req).Error; err != nil {
		log.Printf("[responses] record request error: %v", err)
	}

	log.Printf("[responses] Usage: user=%s model=%s prompt=%d completion=%d cost=%.8f duration=%dms",
		userIDStr[:8], model.Name, promptTokens, completionTokens, cost, durationMs)
}


func (h *ResponsesHandler) loadPreviousMessages(responseID string) []map[string]interface{} {
	if h.redis == nil || responseID == "" {
		return nil
	}

	messages := []map[string]interface{}{}
	currentID := responseID
	for depth := 0; depth < 10 && currentID != ""; depth++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		data, err := h.redis.Get(ctx, "response:"+currentID).Bytes()
		cancel()
		if err != nil {
			log.Printf("[responses] loadPreviousMessages: redis miss for %s (depth %d)", currentID, depth)
			break
		}

		var resp responseObject
		if err := json.Unmarshal(data, &resp); err != nil {
			log.Printf("[responses] loadPreviousMessages: parse error: %v", err)
			break
		}

		thisRoundMsgs := outputItemsToMessages(resp.Output)

		var inputMsgs []map[string]interface{}
		if rawInput, ok := resp.Metadata["_input_messages"]; ok && rawInput != "" {
			_ = json.Unmarshal([]byte(rawInput), &inputMsgs)
		}

		combined := append([]map[string]interface{}{}, inputMsgs...)
		combined = append(combined, thisRoundMsgs...)

		messages = append(combined, messages...)

		if resp.PreviousResponseID != nil { currentID = *resp.PreviousResponseID } else { currentID = "" }
	}

	return messages
}

func outputItemsToMessages(output []outputItem) []map[string]interface{} {
	messages := []map[string]interface{}{}
	var pendingToolCalls []map[string]interface{}

	for _, item := range output {
		switch item.Type {
		case "message":
			text := ""
			for _, part := range item.Content {
				if part.Type == "output_text" {
					text += part.Text
				}
			}
			if text != "" {
				msg := map[string]interface{}{
					"role":    "assistant",
					"content": text,
				}
				if len(pendingToolCalls) > 0 {
					msg["tool_calls"] = pendingToolCalls
					pendingToolCalls = nil
				}
				messages = append(messages, msg)
			}
		case "function_call":
			pendingToolCalls = append(pendingToolCalls, map[string]interface{}{
				"id":   item.CallID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      item.Name,
					"arguments": item.Arguments,
				},
			})
		case "reasoning":
		}
	}

	if len(pendingToolCalls) > 0 {
		messages = append(messages, map[string]interface{}{
			"role":       "assistant",
			"content":    "",
			"tool_calls": pendingToolCalls,
		})
	}

	return messages
}

func (h *ResponsesHandler) List(c *gin.Context) {
	c.JSON(200, gin.H{
		"object": "list",
		"data": []interface{}{},
		"has_more": false,
	})
}

var sseSeqCounter int64

func (h *ResponsesHandler) handleStreamChatToResponses(c *gin.Context, userIDStr string, req *responseCreateRequest, model *models.Model, ch *upstream.Channel, ccBody []byte, startTime time.Time) {
	atomic.StoreInt64(&sseSeqCounter, 0)

	// 改 ccBody 强制 stream=true
	var ccBodyMap map[string]interface{}
	if err := json.Unmarshal(ccBody, &ccBodyMap); err == nil {
		ccBodyMap["stream"] = true
		ccBodyMap["stream_options"] = map[string]interface{}{"include_usage": true}
		ccBody, _ = json.Marshal(ccBodyMap)
	}

resp, err := h.pool.Do(c.Request.Context(), ch, "POST", "/v1/chat/completions", ccBody)
	if err != nil {
		log.Printf("[responses-stream] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[responses-stream] upstream %d: %.300s", resp.StatusCode, string(errBody))
		c.Data(resp.StatusCode, "application/json", errBody)
		return
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Printf("[responses-stream] writer is not flusher")
		return
	}

	// 初始化 response object
	respID := "resp_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
	respObj := &responseObject{
		ID:                respID,
		Object:            "response",
		CreatedAt:         time.Now().Unix(),
		Status:            "in_progress",
		Model:             req.Model,
		Output:            []outputItem{},
		Tools:             []responseTool{},
		ParallelToolCalls: true,
		Temperature:       1.0,
		TopP:              0.98,
Metadata:          map[string]string{},
Instructions:      req.Instructions,
PromptCacheKey:     respID,
PromptCacheRetention: "24h",
SafetyIdentifier:   "user-" + userIDStr[:12],
ServiceTier:       "default",
Background:        false,
FrequencyPenalty:  0.0,
PresencePenalty:   0.0,
MaxToolCalls:      nil,
Moderation:        nil,
MaxOutputTokens:   nil,
Reasoning:         map[string]interface{}{"context": "current_turn", "effort": "low", "summary": "detailed"},
Text: map[string]interface{}{
	"format": map[string]interface{}{"type": "text"},
	"verbosity": "medium",
},
ToolUsage: map[string]interface{}{
	"image_gen": map[string]interface{}{
		"input_tokens": 0,
		"input_tokens_details": map[string]interface{}{"image_tokens": 0, "text_tokens": 0},
		"output_tokens": 0,
		"output_tokens_details": map[string]interface{}{"image_tokens": 0, "text_tokens": 0},
		"total_tokens": 0,
	},
	"web_search": map[string]interface{}{"num_requests": 0},
},
Store: req.Store != nil && *req.Store,
Truncation: "disabled",
ToolChoice: "auto",
	}

	state := &streamState{
		respObj:     respObj,
		outputIndex: 0,
	}

	// 发送初始 events: response.created + response.in_progress
	writeSSEEvent(c.Writer, "response.created", map[string]interface{}{"response": respObj})
	flusher.Flush()
	writeSSEEvent(c.Writer, "response.in_progress", map[string]interface{}{"response": respObj})
	flusher.Flush()

	// 读 SSE stream
	reader := bufio.NewReaderSize(resp.Body, 65536)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(data) == 0 {
				continue
			}
			if bytes.Equal(data, []byte("[DONE]")) {
				break
			}

			var chunk map[string]interface{}
			if jerr := json.Unmarshal(data, &chunk); jerr != nil {
				continue
			}

			processChunk(c.Writer, flusher, chunk, state)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[responses-stream] read error: %v", err)
			break
		}
	}

	// 关闭当前 output item (如果有未结束的)
	closeCurrentItem(c.Writer, flusher, state)

	// 计算 final usage 并设置 status
	state.respObj.Status = "completed"
	if state.totalPrompt > 0 || state.totalCompletion > 0 {
		state.respObj.Usage = &responseUsage{
			InputTokens:  state.totalPrompt,
			OutputTokens: state.totalCompletion,
			TotalTokens:  state.totalPrompt + state.totalCompletion,
InputTokensDetails:  map[string]interface{}{"cached_tokens": 0},
OutputTokensDetails: map[string]interface{}{"reasoning_tokens": 0},
		}
	}

	// 发 final event: response.completed
	state.respObj.CompletedAt = time.Now().Unix()
	writeSSEEvent(c.Writer, "response.completed", map[string]interface{}{"response": state.respObj})
	flusher.Flush()

	flusher.Flush()

	// 计费 + 存 Redis
	h.bill(userIDStr, *model, ch, state.totalPrompt, state.totalCompletion, startTime, 200)
	if h.redis != nil {
		go h.storeInRedis(state.respObj)
	}

	log.Printf("[responses-stream] done user=%s model=%s prompt=%d completion=%d duration=%dms",
		userIDStr[:8], req.Model, state.totalPrompt, state.totalCompletion, time.Since(startTime).Milliseconds())
}



func buildAnthropicMessagesBody(req *responseCreateRequest, model *models.Model, prevMessages []map[string]interface{}) ([]byte, error) {
	messages := []map[string]interface{}{}

	if len(prevMessages) > 0 {
		messages = append(messages, prevMessages...)
	}

	if len(req.Input) > 0 {
		var strInput string
		if err := json.Unmarshal(req.Input, &strInput); err == nil {
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": strInput,
			})
		} else {
			var items []map[string]interface{}
			if err := json.Unmarshal(req.Input, &items); err != nil {
				return nil, fmt.Errorf("invalid input format")
			}
			for _, item := range items {
				role, _ := item["role"].(string)
				if role == "" {
					role = "user"
				}
				if role == "system" {
					continue
				}
				content := item["content"]
				if str, ok := content.(string); ok {
					messages = append(messages, map[string]interface{}{
						"role":    role,
						"content": str,
					})
				} else if parts, ok := content.([]interface{}); ok {
					anthContent := []map[string]interface{}{}
					for _, p := range parts {
						pm, ok := p.(map[string]interface{})
						if !ok {
							continue
						}
						partType, _ := pm["type"].(string)
						switch partType {
						case "input_text", "text", "output_text":
							anthContent = append(anthContent, map[string]interface{}{
								"type": "text",
								"text": pm["text"],
							})
						}
					}
					if len(anthContent) > 0 {
						messages = append(messages, map[string]interface{}{
							"role":    role,
							"content": anthContent,
						})
					}
				}
			}
		}
	}

	body := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   true,
	}

	if req.MaxOutputTokens > 0 {
		body["max_tokens"] = req.MaxOutputTokens
	} else {
		body["max_tokens"] = 4096
	}

	if req.Instructions != "" {
		body["system"] = req.Instructions
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}

	return json.Marshal(body)
}

func (h *ResponsesHandler) handleStreamMessagesToResponses(c *gin.Context, userIDStr string, req *responseCreateRequest, model *models.Model, ch *upstream.Channel, anthBody []byte, startTime time.Time) {
	atomic.StoreInt64(&sseSeqCounter, 0)

	resp, err := h.pool.Do(c.Request.Context(), ch, "POST", "/v1/messages", anthBody)
	if err != nil {
		log.Printf("[responses-msgs-stream] upstream error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream request failed", "type": "api_error"}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[responses-msgs-stream] upstream %d: %.300s", resp.StatusCode, string(errBody))
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

	// 初始化 response object
	respID := "resp_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
	respObj := &responseObject{
		ID:                  respID,
		Object:              "response",
		CreatedAt:           time.Now().Unix(),
		Status:              "in_progress",
		Model:               req.Model,
		Output:              []outputItem{},
		Tools:               []responseTool{},
		ParallelToolCalls:   true,
		Temperature:         1.0,
		TopP:                0.98,
		Metadata:            map[string]string{},
		Instructions:        req.Instructions,
		PromptCacheKey:      respID,
		PromptCacheRetention: "24h",
		SafetyIdentifier:    "user-" + userIDStr[:12],
		ServiceTier:         "default",
		Background:          false,
		FrequencyPenalty:    0.0,
		PresencePenalty:     0.0,
		MaxToolCalls:        nil,
		Moderation:          nil,
		MaxOutputTokens:     nil,
		Reasoning:           map[string]interface{}{"effort": "low", "summary": "detailed"},
		Text:                map[string]interface{}{"format": map[string]interface{}{"type": "text"}, "verbosity": "medium"},
		ToolUsage:           map[string]interface{}{"image_gen": map[string]interface{}{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}, "web_search": map[string]interface{}{"num_requests": 0}},
		Store:               false,
		Truncation:          "disabled",
		ToolChoice:          "auto",
	}

	// state
	msgID := "msg_018" + strings.ReplaceAll(uuid.New().String(), "-", "") + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
	var fullText string
	var totalPrompt, totalCompletion int

	// 发期断事䶯: response.created + response.in_progress
	writeSSEEvent(c.Writer, "response.created", map[string]interface{}{"response": respObj})
	flusher.Flush()
	writeSSEEvent(c.Writer, "response.in_progress", map[string]interface{}{"response": respObj})
	flusher.Flush()

	// 发 output_item.added
	initItem := outputItem{
		Type:    "message",
		ID:      msgID,
		Role:    "assistant",
		Status:  "in_progress",
		Content: []contentPart{},
	}
	writeSSEEvent(c.Writer, "response.output_item.added", map[string]interface{}{
		"output_index": 0,
		"item":         initItem,
	})
	flusher.Flush()

	// 发 content_part.added
	writeSSEEvent(c.Writer, "response.content_part.added", map[string]interface{}{
		"item_id":       msgID,
		"output_index":  0,
		"content_index": 0,
		"part": contentPart{
			Type:        "output_text",
			Text:        "",
			Annotations: []interface{}{},
			Logprobs:    []interface{}{},
		},
	})
	flusher.Flush()

	// 读上游 Anthropic SSE
	reader := bufio.NewReaderSize(resp.Body, 65536)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(data) == 0 {
				continue
			}

			var evt map[string]interface{}
			if jerr := json.Unmarshal(data, &evt); jerr != nil {
				continue
			}

			evtType, _ := evt["type"].(string)
			switch evtType {
			case "content_block_delta":
				if delta, ok := evt["delta"].(map[string]interface{}); ok {
					if dType, _ := delta["type"].(string); dType == "text_delta" {
						if text, ok := delta["text"].(string); ok {
							fullText += text
							writeSSEEvent(c.Writer, "response.output_text.delta", map[string]interface{}{
								"item_id":       msgID,
								"output_index":  0,
								"content_index": 0,
								"delta":         text,
								"logprobs":      []interface{}{},
							})
							flusher.Flush()
						}
					}
				}
			case "message_start":
				if msg, ok := evt["message"].(map[string]interface{}); ok {
					if usage, ok := msg["usage"].(map[string]interface{}); ok {
						if v, ok := usage["input_tokens"].(float64); ok {
							totalPrompt = int(v)
						}
					}
				}
			case "message_delta":
				if usage, ok := evt["usage"].(map[string]interface{}); ok {
					if v, ok := usage["output_tokens"].(float64); ok {
						totalCompletion = int(v)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[responses-msgs-stream] read error: %v", err)
			break
		}
	}

	// 发期断事: response.output_text.done + content_part.done + output_item.done
	writeSSEEvent(c.Writer, "response.output_text.done", map[string]interface{}{
		"item_id":       msgID,
		"output_index":  0,
		"content_index": 0,
		"text":          fullText,
		"logprobs":      []interface{}{},
	})
	flusher.Flush()

	finalPart := contentPart{
		Type:        "output_text",
		Text:        fullText,
		Annotations: []interface{}{},
		Logprobs:    []interface{}{},
	}
	writeSSEEvent(c.Writer, "response.content_part.done", map[string]interface{}{
		"item_id":       msgID,
		"output_index":  0,
		"content_index": 0,
		"part":          finalPart,
	})
	flusher.Flush()

	finalItem := outputItem{
		Type:    "message",
		ID:      msgID,
		Role:    "assistant",
		Status:  "completed",
		Phase:   "final_answer",
		Content: []contentPart{finalPart},
	}
	writeSSEEvent(c.Writer, "response.output_item.done", map[string]interface{}{
		"output_index": 0,
		"item":         finalItem,
	})
	flusher.Flush()

	respObj.Output = append(respObj.Output, finalItem)
	respObj.Status = "completed"
	respObj.CompletedAt = time.Now().Unix()
	if totalPrompt > 0 || totalCompletion > 0 {
		respObj.Usage = &responseUsage{
			InputTokens:         totalPrompt,
			OutputTokens:        totalCompletion,
			TotalTokens:         totalPrompt + totalCompletion,
			InputTokensDetails:  map[string]interface{}{"cached_tokens": 0},
			OutputTokensDetails: map[string]interface{}{"reasoning_tokens": 0},
		}
	}

	writeSSEEvent(c.Writer, "response.completed", map[string]interface{}{"response": respObj})
	flusher.Flush()

	log.Printf("[responses-msgs-stream] done user=%s model=%s prompt=%d completion=%d duration=%dms", userIDStr[:8], req.Model, totalPrompt, totalCompletion, time.Since(startTime).Milliseconds())

	if totalPrompt > 0 || totalCompletion > 0 {
		h.bill(userIDStr, *model, ch, totalPrompt, totalCompletion, startTime, 200)
	}
}
