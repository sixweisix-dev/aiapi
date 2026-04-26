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
	Name       string   `json:"name" binding:"required,min=1,max=100"`
	Models     []string `json:"models,omitempty"`   // allowed model names (empty = all)
	RPMLimit   *int     `json:"rpm_limit,omitempty"` // rate limit per minute
	TPMLimit   *int     `json:"tpm_limit,omitempty"` // token per minute
}

type APIKeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key,omitempty"` // only shown on create
	Prefix    string    `json:"prefix"`
	IsActive  bool      `json:"is_active"`
	LastUsed  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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

	apiKey := models.APIKey{
		UserID:  parsedUserID,
		Name:    req.Name,
		KeyHash: keyHash,
		Prefix:  prefix,
		IsActive: true,
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
		ID:        apiKey.ID.String(),
		Name:      apiKey.Name,
		Key:       rawKey, // only time the full key is returned
		Prefix:    prefix,
		IsActive:  apiKey.IsActive,
		CreatedAt: apiKey.CreatedAt,
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
			ID:        k.ID.String(),
			Name:      k.Name,
			Prefix:    k.Prefix,
			IsActive:  k.IsActive,
			LastUsed:  k.LastUsedAt,
			CreatedAt: k.CreatedAt,
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
