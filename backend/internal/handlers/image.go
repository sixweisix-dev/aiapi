package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"log"
	"net/http"
	"net/textproto"
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

// HandleEdit POST /v1/images/edits (multipart/form-data 或 JSON via playground)
func (h *ImageHandler) HandleEdit(c *gin.Context) {
	isMultipart := true
	if v, exists := c.Get("playground_edits_json"); exists {
		if b, ok := v.(bool); ok && b {
			isMultipart = false  // playground JSON path
		}
	}
	h.handleImage(c, "/v1/images/edits", isMultipart)
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
	// 如果是 playground JSON edits (isMultipart=false 但 endpoint 是 edits), 也读 JSON body
	isPlaygroundJSONEdits := false
	if _, exists := c.Get("playground_edits_json"); exists {
		isPlaygroundJSONEdits = true
	}
	if !isMultipart || isPlaygroundJSONEdits {
		var err error
		reqBodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil && len(reqBodyBytes) > 0 {
			var tmp struct{ Model string `json:"model"` }
			if json.Unmarshal(reqBodyBytes, &tmp) == nil && tmp.Model != "" {
				modelName = tmp.Model
			}
		}
	} else if isMultipart {
		// multipart edits: 读 form 取 model 字段, 然后重新构造 body
		var err error
		reqBodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil && len(reqBodyBytes) > 0 {
			// 解析 multipart 取 model
			ctType := c.Request.Header.Get("Content-Type")
			if idx := strings.Index(ctType, "boundary="); idx >= 0 {
				boundary := ctType[idx+9:]
				if semi := strings.Index(boundary, ";"); semi >= 0 {
					boundary = boundary[:semi]
				}
				boundary = strings.Trim(boundary, "\"")
				mpReader := multipart.NewReader(bytes.NewReader(reqBodyBytes), boundary)
				for {
					part, e := mpReader.NextPart()
					if e != nil {
						break
					}
					if part.FormName() == "model" {
						vbuf, _ := io.ReadAll(part)
						if len(vbuf) > 0 {
							modelName = string(vbuf)
						}
						part.Close()
						break
					}
					part.Close()
				}
			}
		}
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
	ch := h.pool.Select(providerForChannel, modelName, modelRec.GroupID)
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

	playgroundMultipartCT := ""
	if isPlaygroundJSONEdits {
		// playground JSON edits -> 转成 multipart 发给上游
		var jsonBody map[string]interface{}
		if err := json.Unmarshal(reqBodyBytes, &jsonBody); err != nil {
			c.JSON(400, gin.H{"error": gin.H{"message": "invalid JSON body"}})
			return
		}
		imgDataURL, _ := jsonBody["image"].(string)
		if imgDataURL == "" {
			c.JSON(400, gin.H{"error": gin.H{"message": "image field missing"}})
			return
		}
		b64Part := imgDataURL
		if idx := strings.Index(imgDataURL, "base64,"); idx >= 0 {
			b64Part = imgDataURL[idx+7:]
		}
		imgBytes, err := base64.StdEncoding.DecodeString(b64Part)
		if err != nil {
			c.JSON(400, gin.H{"error": gin.H{"message": "invalid base64 image: " + err.Error()}})
			return
		}
		var mpBuf bytes.Buffer
		mpWriter := multipart.NewWriter(&mpBuf)
		// 从 data URL 提取真实 mime type (Playground 可能上传 jpg/png/webp)
		imgMime := "image/png"
		imgFilename := "image.png"
		if strings.HasPrefix(imgDataURL, "data:") {
			if semi := strings.Index(imgDataURL, ";"); semi > 5 {
				imgMime = imgDataURL[5:semi]
				switch imgMime {
				case "image/jpeg", "image/jpg":
					imgFilename = "image.jpg"
				case "image/webp":
					imgFilename = "image.webp"
				case "image/gif":
					imgFilename = "image.gif"
				}
			}
		}
		log.Printf("[image] playground upload mime=%s filename=%s bytes=%d", imgMime, imgFilename, len(imgBytes))
		imgHdr := make(textproto.MIMEHeader)
		imgHdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename=%q`, imgFilename))
		imgHdr.Set("Content-Type", imgMime)
		imgPart, _ := mpWriter.CreatePart(imgHdr)
		imgPart.Write(imgBytes)
		// 用重写后的 model name (yyapi upstream_name 已应用)
		modelToSend := modelName
		if modelRec.UpstreamName != nil && *modelRec.UpstreamName != "" {
			modelToSend = *modelRec.UpstreamName
		}
		mpWriter.WriteField("model", modelToSend)
		if p, ok := jsonBody["prompt"].(string); ok && p != "" {
			mpWriter.WriteField("prompt", p)
		}
		if s, ok := jsonBody["size"].(string); ok && s != "" {
			mpWriter.WriteField("size", s)
		}
		if q, ok := jsonBody["quality"].(string); ok && q != "" {
			mpWriter.WriteField("quality", q)
		}
		if n, ok := jsonBody["n"].(float64); ok {
			mpWriter.WriteField("n", fmt.Sprintf("%d", int(n)))
		}
		mpWriter.Close()
		reqBody = &mpBuf
		playgroundMultipartCT = mpWriter.FormDataContentType()
		log.Printf("[image] playground JSON->multipart, CT=%s body=%d bytes", playgroundMultipartCT, mpBuf.Len())
	} else if isMultipart {
		// multipart edits: 重新构造 body, 把 model 字段重写为 upstream_name
		if modelRec.UpstreamName != nil && *modelRec.UpstreamName != "" && *modelRec.UpstreamName != modelRec.Name && len(reqBodyBytes) > 0 {
			ctType := c.Request.Header.Get("Content-Type")
			if idx := strings.Index(ctType, "boundary="); idx >= 0 {
				boundary := ctType[idx+9:]
				if semi := strings.Index(boundary, ";"); semi >= 0 {
					boundary = boundary[:semi]
				}
				boundary = strings.Trim(boundary, "\"")
				mpReader := multipart.NewReader(bytes.NewReader(reqBodyBytes), boundary)
				var mpBuf bytes.Buffer
				mpWriter := multipart.NewWriter(&mpBuf)
				for {
					part, e := mpReader.NextPart()
					if e != nil {
						break
					}
					name := part.FormName()
					if name == "model" {
						mpWriter.WriteField("model", *modelRec.UpstreamName)
						part.Close()
						continue
					}
					if fname := part.FileName(); fname != "" {
						// file field — 保留原 Content-Type (yyapi 严格要求, 默认 octet-stream 会 500)
						fileHdr := make(textproto.MIMEHeader)
						fileHdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, name, fname))
						ct := part.Header.Get("Content-Type")
						if ct == "" {
							// 根据 fname 后缀猜
							lower := strings.ToLower(fname)
							switch {
							case strings.HasSuffix(lower, ".png"):
								ct = "image/png"
							case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
								ct = "image/jpeg"
							case strings.HasSuffix(lower, ".webp"):
								ct = "image/webp"
							default:
								ct = "image/png"
							}
						}
						fileHdr.Set("Content-Type", ct)
						newPart, _ := mpWriter.CreatePart(fileHdr)
						io.Copy(newPart, part)
					} else {
						vbuf, _ := io.ReadAll(part)
						mpWriter.WriteField(name, string(vbuf))
					}
					part.Close()
				}
				mpWriter.Close()
				reqBody = &mpBuf
				playgroundMultipartCT = mpWriter.FormDataContentType()
				log.Printf("[image] multipart model rewritten %s -> %s body=%d bytes", modelName, *modelRec.UpstreamName, mpBuf.Len())
			} else {
				reqBody = bytes.NewReader(reqBodyBytes)
			}
		} else {
			reqBody = bytes.NewReader(reqBodyBytes)
		}
	} else {
		reqBody = bytes.NewReader(reqBodyBytes)
	}

	upstreamReq, err := http.NewRequest("POST", upstreamURL, reqBody)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{"message": "build upstream request failed"}})
		return
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+ch.APIKey)
	if playgroundMultipartCT != "" {
		upstreamReq.Header.Set("Content-Type", playgroundMultipartCT)
	} else if isMultipart {
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
