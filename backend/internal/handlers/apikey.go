package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"ai-api-gateway/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type APIKeyHandler struct {
	db *gorm.DB
}

type CreateAPIKeyRequest struct {
	Name           string   `json:"name" binding:"required,min=1,max=100"`
	Models         []string `json:"models,omitempty"`     // allowed model names (empty = all)
	RPMLimit       *int     `json:"rpm_limit,omitempty"`  // 每分钟请求数限制
	TPMLimit       *int     `json:"tpm_limit,omitempty"`  // 每分钟 token 限制
	ProjectName    *string  `json:"project_name,omitempty"`    // 项目名（B 端用）
	MonthlyBudget  *float64 `json:"monthly_budget,omitempty"`  // 月预算（CNY）
	BudgetAlertPct *int     `json:"budget_alert_pct,omitempty"` // 预算告警阈值（默认 80%）
}

type APIKeyResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Key            string     `json:"key,omitempty"` // 只在创建时返回完整 key
	Prefix         string     `json:"prefix"`
	IsActive       bool       `json:"is_active"`
	LastUsed       *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	// 项目 + 限制 + 预算
	ProjectName    *string    `json:"project_name,omitempty"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	MonthlyBudget  *float64   `json:"monthly_budget,omitempty"`
	BudgetUsed     float64    `json:"budget_used"`
	BudgetAlertPct int        `json:"budget_alert_pct"`
	TotalUsed      int        `json:"total_used"`
}

func NewAPIKeyHandler(db *gorm.DB) *APIKeyHandler {
	return &APIKeyHandler{db: db}
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate sk-xxx API key
	rawKey, prefix := generateAPIKey()

	// Hash the key for storage
	keyHash := hashKey(rawKey)

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user"})
		return
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	alertPct := 80
	if req.BudgetAlertPct != nil {
		alertPct = *req.BudgetAlertPct
	}
	apiKey := models.APIKey{
		UserID:            parsedUserID,
		Name:              req.Name,
		KeyHash:           keyHash,
		Prefix:            prefix,
		IsActive:          true,
		ProjectName:       req.ProjectName,
		MonthlyBudget:     req.MonthlyBudget,
		BudgetAlertPct:    alertPct,
		BudgetPeriodStart: &periodStart,
	}

	if req.RPMLimit != nil {
		apiKey.RPMLimit = req.RPMLimit
	}
	if req.TPMLimit != nil {
		apiKey.TPMLimit = req.TPMLimit
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&apiKey).Error; err != nil {
			return fmt.Errorf("failed to create API key: %w", err)
		}

		// Handle allowed models
		if len(req.Models) > 0 {
			var allowedModels []models.Model
			if err := tx.Where("name IN ? AND is_enabled = ?", req.Models, true).Find(&allowedModels).Error; err != nil {
				return fmt.Errorf("failed to find models: %w", err)
			}
			for _, m := range allowedModels {
				link := models.APIKeyAllowedModel{
					APIKeyID: apiKey.ID,
					ModelID:  m.ID,
				}
				if err := tx.Create(&link).Error; err != nil {
					return fmt.Errorf("failed to link model: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Create API key error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, APIKeyResponse{
		ID:             apiKey.ID.String(),
		Name:           apiKey.Name,
		Key:            rawKey,
		Prefix:         prefix,
		IsActive:       apiKey.IsActive,
		CreatedAt:      apiKey.CreatedAt,
		ProjectName:    apiKey.ProjectName,
		RPMLimit:       apiKey.RPMLimit,
		TPMLimit:       apiKey.TPMLimit,
		MonthlyBudget:  apiKey.MonthlyBudget,
		BudgetAlertPct: apiKey.BudgetAlertPct,
	})
}

func (h *APIKeyHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user"})
		return
	}

	var keys []models.APIKey
	if err := h.db.Where("user_id = ?", parsedUserID).Order("created_at desc").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch API keys"})
		return
	}

	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, APIKeyResponse{
			ID:             k.ID.String(),
			Name:           k.Name,
			Prefix:         k.Prefix,
			IsActive:       k.IsActive,
			LastUsed:       k.LastUsedAt,
			CreatedAt:      k.CreatedAt,
			ProjectName:    k.ProjectName,
			RPMLimit:       k.RPMLimit,
			TPMLimit:       k.TPMLimit,
			MonthlyBudget:  k.MonthlyBudget,
			BudgetUsed:     k.BudgetUsed,
			BudgetAlertPct: k.BudgetAlertPct,
			TotalUsed:      k.TotalUsed,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	if userID == "" || keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	result := h.db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&models.APIKey{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}

func (h *APIKeyHandler) Toggle(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	if userID == "" || keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var key models.APIKey
	if err := h.db.Where("id = ? AND user_id = ?", keyID, userID).First(&key).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	// Disallow disabling the last active key
	if key.IsActive {
		var count int64
		h.db.Model(&models.APIKey{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&count)
		if count <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disable the last active API key"})
			return
		}
	}

	h.db.Model(&key).Update("is_active", !key.IsActive)
	c.JSON(http.StatusOK, gin.H{"is_active": !key.IsActive})
}

// generateAPIKey creates an sk-xxx formatted key with 48 chars of hex.
func generateAPIKey() (raw, prefix string) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// fallback — should never happen
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	hexStr := hex.EncodeToString(bytes) // 64 hex chars
	prefix = hexStr[:8]
	raw = "sk-" + hexStr
	return
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Update modifies an API key's project_name / monthly_budget / limits / alert_pct
type UpdateAPIKeyRequest struct {
	Name           *string  `json:"name,omitempty"`
	ProjectName    *string  `json:"project_name,omitempty"`
	RPMLimit       *int     `json:"rpm_limit,omitempty"`
	TPMLimit       *int     `json:"tpm_limit,omitempty"`
	MonthlyBudget  *float64 `json:"monthly_budget,omitempty"`
	BudgetAlertPct *int     `json:"budget_alert_pct,omitempty"`
}

func (h *APIKeyHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")
	if userID == "" || keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var req UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ProjectName != nil {
		updates["project_name"] = *req.ProjectName
	}
	if req.RPMLimit != nil {
		updates["rpm_limit"] = *req.RPMLimit
	}
	if req.TPMLimit != nil {
		updates["tpm_limit"] = *req.TPMLimit
	}
	if req.MonthlyBudget != nil {
		updates["monthly_budget"] = *req.MonthlyBudget
		// 改预算时重置告警状态，避免新预算又卡在已告警状态
		updates["budget_alerted"] = false
	}
	if req.BudgetAlertPct != nil {
		updates["budget_alert_pct"] = *req.BudgetAlertPct
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	result := h.db.Model(&models.APIKey{}).
		Where("id = ? AND user_id = ?", keyID, userID).
		Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}
