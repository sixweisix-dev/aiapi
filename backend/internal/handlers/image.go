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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/channelmetrics"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"
)

// 单张图固定成本(CNY): aitechflux $0.038 × 7 (汇率) × 2.5 (倍率) = ¥0.665
const ImageCostPerCallCNY = 0.665

type ImageHandler struct {
	db            *gorm.DB
	pool          *upstream.Pool
	billingEngine *billing.Engine
	tracker       *channelmetrics.Tracker
}

func NewImageHandler(db *gorm.DB, pool *upstream.Pool, be *billing.Engine, tracker *channelmetrics.Tracker) *ImageHandler {
	return &ImageHandler{db: db, pool: pool, billingEngine: be, tracker: tracker}
}

// HandleGenerate POST /v1/images/generations
func (h *ImageHandler) HandleGenerate(c *gin.Context) {
	h.handleImage(c, "/v1/images/generations", false)
}

// HandleEdit POST /v1/images/edits (multipart/form-data)
func (h *ImageHandler) HandleEdit(c *gin.Context) {
	h.handleImage(c, "/v1/images/edits", true)
}

func (h *ImageHandler) handleImage(c *gin.Context, endpoint string, isMultipart bool) {
	userIDRaw, _ := c.Get("user_id")
	userIDStr, _ := userIDRaw.(string)
	if userIDStr == "" {
		c.JSON(401, gin.H{"error": gin.H{"message": "authentication required", "type": "auth_error"}})
		return
	}

	// 0. 解析请求 body 拿 model 字段(只读, 不消费 body)
	var reqBodyBytes []byte
	modelName := "gpt-image-2"
	if !isMultipart {
		var err error
		reqBodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil && len(reqBodyBytes) > 0 {
			var tmp struct{ Model string `json:"model"` }
			if json.Unmarshal(reqBodyBytes, &tmp) == nil && tmp.Model != "" {
				modelName = tmp.Model
			}
		}
	} else {
		// multipart edits 端点 - model 在 form data 里, 不解析了, 默认 gpt-image-2
	}

	// 1. 查 model 拿单价
	var modelRec models.Model
	if err := h.db.Where("name = ? AND is_enabled = ?", modelName, true).First(&modelRec).Error; err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "model not found or disabled: " + modelName}})
		return
	}
	costPerCall := modelRec.CostPerCall
	if costPerCall <= 0 {
		costPerCall = ImageCostPerCallCNY // fallback
	}

	// 2. 先检查余额
	parsedUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"message": "invalid user id"}})
		return
	}
	var user models.User
	if err := h.db.First(&user, parsedUserID).Error; err != nil {
		c.JSON(401, gin.H{"error": gin.H{"message": "user not found"}})
		return
	}
	if user.Balance < costPerCall {
		c.JSON(402, gin.H{"error": gin.H{"message": fmt.Sprintf("insufficient balance: need %.4f CNY, have %.4f", costPerCall, user.Balance), "type": "balance_error"}})
		return
	}

	// 2. 选 channel - 根据 model.Provider 选 (yyapi=openai, aitechflux=multi_aggregator)
	providerForChannel := modelRec.Provider
	if providerForChannel == "" {
		providerForChannel = "multi_aggregator"
	}
	ch := h.pool.Select(providerForChannel, modelRec.GroupID)
	log.Printf("[image] model=%s provider=%s -> selecting channel", modelName, providerForChannel)
	if ch == nil {
		c.JSON(503, gin.H{"error": gin.H{"message": "no upstream available for image generation"}})
		return
	}

	// 3. 构造上游 URL
	upstreamURL := strings.TrimRight(ch.BaseURL, "/") + endpoint
	log.Printf("[image] route user=%s -> ch=%s url=%s", userIDStr, ch.Name, upstreamURL)

	// 4. 转发请求
	reqStart := time.Now()
	var upstreamResp *http.Response
	var reqBody io.Reader

	// 替换 body 中的 model 字段为 upstream_name (yyapi 等用别名)
	if !isMultipart && modelRec.UpstreamName != nil && *modelRec.UpstreamName != "" && *modelRec.UpstreamName != modelRec.Name {
		var bodyMap map[string]interface{}
		if json.Unmarshal(reqBodyBytes, &bodyMap) == nil {
			bodyMap["model"] = *modelRec.UpstreamName
			if newBody, err := json.Marshal(bodyMap); err == nil {
				reqBodyBytes = newBody
				log.Printf("[image] body model rewritten: %s -> %s", modelName, *modelRec.UpstreamName)
			}
		}
	}

	if isMultipart {
		// edits 端点用 multipart, 直接传 raw body
		reqBody = c.Request.Body
	} else {
		// generations 端点 - body 已在前面读过, 复用
		reqBody = bytes.NewReader(reqBodyBytes)
	}

	upstreamReq, err := http.NewRequest("POST", upstreamURL, reqBody)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "build upstream request failed"}})
		return
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+ch.APIKey)
	if isMultipart {
		// 透传原 Content-Type (含 boundary)
		upstreamReq.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	} else {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	upstreamResp, err = client.Do(upstreamReq)
	if err != nil {
		log.Printf("[image] upstream call error: %v", err)
		c.JSON(502, gin.H{"error": gin.H{"message": "upstream call failed: " + err.Error()}})
		return
	}
	defer upstreamResp.Body.Close()

	respBody, _ := io.ReadAll(upstreamResp.Body)
	latencyMs := time.Since(reqStart).Milliseconds()

	// 5. 检查响应
	if upstreamResp.StatusCode != 200 {
		log.Printf("[image] upstream %d body=%s", upstreamResp.StatusCode, string(respBody)[:min(300, len(respBody))])
		c.Data(upstreamResp.StatusCode, "application/json", respBody)
		return
	}

	// 6. 扣费(成功才扣)
	if _, err := h.billingEngine.DeductBalance(userIDStr, costPerCall); err != nil {
		log.Printf("[image] deduct balance failed: %v", err)
	}

	// 7. 记录账单
	if _, err := h.billingEngine.RecordImageBilling(userIDStr, modelName, costPerCall, 1); err != nil {
		log.Printf("[image] record billing failed: %v", err)
	}

	// 扣上游 channel quota
	if h.tracker != nil {
		h.tracker.RecordSuccess(ch.ID, costPerCall, 0, 0, latencyMs)
	}

	// 写 requests 记录 (账单页/用量柱状图依赖此表)
	var apiKeyUUIDPtr *uuid.UUID
	if v, ok := c.Get("api_key_id"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			if u, e := uuid.Parse(s); e == nil {
				apiKeyUUIDPtr = &u
			}
		}
	}
	var chIDPtr *uuid.UUID
	if chID, e := uuid.Parse(ch.ID); e == nil {
		chIDPtr = &chID
	}
	reqLog := &models.Request{
		UserID:            parsedUserID,
		APIKeyID:          apiKeyUUIDPtr,
		ModelID:           modelRec.ID,
		UpstreamChannelID: chIDPtr,
		Path:              endpoint,
		Method:            "POST",
		StatusCode:        200,
		PromptTokens:      0,
		CompletionTokens:  0,
		TotalTokens:       0,
		Cost:              costPerCall,
		DurationMs:        int(latencyMs),
	}
	if err := h.db.Create(reqLog).Error; err != nil {
		log.Printf("[image] log request failed: %v", err)
	}

	log.Printf("[image] success user=%s ch=%s endpoint=%s cost=%.4f latency=%dms",
		userIDStr, ch.Name, endpoint, costPerCall, latencyMs)

	// 9. 透传响应
	for k, v := range upstreamResp.Header {
		if strings.HasPrefix(k, "X-") || k == "Content-Type" {
			for _, val := range v {
				c.Header(k, val)
			}
		}
	}
	c.Data(200, upstreamResp.Header.Get("Content-Type"), respBody)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
