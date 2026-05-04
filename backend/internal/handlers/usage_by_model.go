package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UsageByModelHandler 按模型分组的每日消耗统计
type UsageByModelHandler struct {
	db *gorm.DB
}

func NewUsageByModelHandler(db *gorm.DB) *UsageByModelHandler {
	return &UsageByModelHandler{db: db}
}

type DailyModelStat struct {
	Date     string  `json:"date"`     // 2026-05-03
	Model    string  `json:"model"`    // claude-opus-4-7
	Cost     float64 `json:"cost"`     // 该日该模型总消耗(CNY)
	Tokens   int64   `json:"tokens"`   // 该日该模型总 token
	Requests int64   `json:"requests"` // 该日该模型请求数
}

// queryDailyByModel 通用查询: 按 user_id 过滤(管理员传空 uuid 表示全平台)
func (h *UsageByModelHandler) queryDailyByModel(userID *uuid.UUID, days int) ([]DailyModelStat, error) {
	if days < 1 || days > 365 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	rows := []DailyModelStat{}
	query := h.db.Table("requests").
		Select(`TO_CHAR(DATE(requests.created_at), 'YYYY-MM-DD') as date,
			models.name as model,
			COALESCE(SUM(requests.cost), 0) as cost,
			COALESCE(SUM(requests.total_tokens), 0) as tokens,
			COUNT(*) as requests`).
		Joins("JOIN models ON models.id = requests.model_id").
		Where("requests.created_at >= ?", since).
		Where("requests.status_code < 500").
		Group("DATE(requests.created_at), models.name").
		Order("date ASC, model ASC")

	if userID != nil {
		query = query.Where("requests.user_id = ?", *userID)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UserUsageByModel GET /v1/user/usage/by-model?days=7
func (h *UsageByModelHandler) UserUsageByModel(c *gin.Context) {
	uidVal, _ := c.Get("user_id")
	uidStr, _ := uidVal.(string)
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	rows, err := h.queryDailyByModel(&uid, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "data": rows})
}

// AdminUsageByModel GET /v1/admin/usage/by-model?days=30
func (h *UsageByModelHandler) AdminUsageByModel(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	rows, err := h.queryDailyByModel(nil, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "data": rows})
}
