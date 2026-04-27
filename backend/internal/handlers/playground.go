package handlers

import (
"net/http"

"ai-api-gateway/internal/models"
"github.com/gin-gonic/gin"
"github.com/google/uuid"
"gorm.io/gorm"
)

// PlaygroundHandler bridges JWT-authenticated users to chat completions
// without requiring them to handle raw sk-xxx API keys in the browser.
type PlaygroundHandler struct {
db          *gorm.DB
chatHandler *ChatHandler
}

func NewPlaygroundHandler(db *gorm.DB, chatHandler *ChatHandler) *PlaygroundHandler {
return &PlaygroundHandler{db: db, chatHandler: chatHandler}
}

// PlaygroundChat: JWT 认证 → 查指定 / 第一个 active 的 API key → 塞 context → 转发到 chat handler
// 前端只需传 api_key_id（可选）+ 标准 chat completions body
func (h *PlaygroundHandler) PlaygroundChat(c *gin.Context) {
userID := c.GetString("user_id")
if userID == "" {
c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "unauthorized", "type": "auth_error"}})
return
}

parsedUID, err := uuid.Parse(userID)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "invalid user id", "type": "internal_error"}})
return
}

// 优先用前端指定的 api_key_id；否则取第一个 active 的
requestedKeyID := c.Query("api_key_id")
var key models.APIKey
q := h.db.Where("user_id = ? AND is_active = true AND deleted_at IS NULL", parsedUID)
if requestedKeyID != "" {
q = q.Where("id = ?", requestedKeyID)
}
if err := q.Order("created_at ASC").First(&key).Error; err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
"message": "no active API key found, please create one in 'API Keys' page first",
"type":    "no_api_key",
}})
return
}

// 把 APIKeyAuth 中间件该塞的字段全部塞进 context（让 chatHandler.Handle 跑得跟正常 sk-xxx 一样）
c.Set("api_key_hash", key.KeyHash)
c.Set("api_key_id", key.ID.String())
c.Set("api_key_rpm", key.RPMLimit)
c.Set("api_key_tpm", key.TPMLimit)
c.Set("auth_method", "playground")

// 直接转发到现有 chat handler（共享所有上游、内容过滤、计费、预算逻辑）
h.chatHandler.Handle(c)
}
