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
}

func NewMessagesHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine) *MessagesHandler {
	return &MessagesHandler{db: db, pool: pool, billingEngine: be}
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

	ch := h.pool.Select(model.Provider)
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
	if ab := c.GetHeader("anthropic-beta"); ab != "" {
		upstreamReq.Header.Set("anthropic-beta", ab)
	}

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(upstreamReq)
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

		var promptTokens, completionTokens int
		buf := make([]byte, 4096)
		flusher, canFlush := c.Writer.(http.Flusher)

		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				c.Writer.Write(chunk)
				if canFlush {
					flusher.Flush()
				}
				if bytes.Contains(chunk, []byte(`"input_tokens"`)) {
					lines := bytes.Split(chunk, []byte("\n"))
					for _, line := range lines {
						line = bytes.TrimPrefix(line, []byte("data: "))
						var ev map[string]interface{}
						if json.Unmarshal(line, &ev) == nil {
							if usage, ok := ev["usage"].(map[string]interface{}); ok {
								if v, ok := usage["input_tokens"].(float64); ok {
									promptTokens = int(v)
								}
								if v, ok := usage["output_tokens"].(float64); ok {
									completionTokens = int(v)
								}
							}
						}
					}
				}
			}
			if readErr == io.EOF || readErr != nil {
				break
			}
		}
		h.bill(userIDStr, model, ch, promptTokens, completionTokens, startTime, resp.StatusCode)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		c.Writer.Write(respBody)

		var anthropicResp struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		promptTokens, completionTokens := 0, 0
		if resp.StatusCode == 200 {
			if json.Unmarshal(respBody, &anthropicResp) == nil {
				promptTokens = anthropicResp.Usage.InputTokens
				completionTokens = anthropicResp.Usage.OutputTokens
			}
		}
		h.bill(userIDStr, model, ch, promptTokens, completionTokens, startTime, resp.StatusCode)
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
