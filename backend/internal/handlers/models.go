package handlers

import (
	"time"

	"ai-api-gateway/internal/adapter"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ModelsHandler struct {
	db *gorm.DB
}

func NewModelsHandler(db *gorm.DB) *ModelsHandler {
	return &ModelsHandler{db: db}
}

func (h *ModelsHandler) List(c *gin.Context) {
	type modelRow struct {
		Name        string
		Provider    string
		IsPublic    bool
	}

	var rows []modelRow
	if err := h.db.Table("models").
		Select("name, provider, is_public").
		Where("is_enabled = ?", true).
		Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed to fetch models"})
		return
	}

	models := make([]adapter.ModelInfo, 0, len(rows))
	now := time.Now().Unix()
	for _, r := range rows {
		if !r.IsPublic {
			continue
		}
		models = append(models, adapter.ModelInfo{
			ID:      r.Name,
			Object:  "model",
			Created: now,
			OwnedBy: r.Provider,
		})
	}

	c.JSON(200, adapter.ModelsListResponse{
		Object: "list",
		Data:   models,
	})
}
