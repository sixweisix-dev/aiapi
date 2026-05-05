package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WidgetHandler 提供 iOS Scriptable 锁屏组件用的精简 dashboard API。
// 鉴权方式: ?key=sk-xxx (admin 用户的 API key)
type WidgetHandler struct {
	db *gorm.DB
}

func NewWidgetHandler(db *gorm.DB) *WidgetHandler {
	return &WidgetHandler{db: db}
}

type widgetChannel struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`        // normal / warning / critical / exhausted
	QuotaPct     float64 `json:"quota_pct"`     // 已用百分比 (0-100)
	TodayCNY     float64 `json:"today_cny"`
	RemainingUSD float64 `json:"remaining_usd"` // 剩余配额 USD
}

type widgetDashboardResp struct {
	Channels       []widgetChannel `json:"channels"`
	TotalTodayCNY  float64         `json:"total_today_cny"`
	TotalMonthCNY  float64         `json:"total_month_cny"`
	TotalQuotaPct  float64         `json:"total_quota_pct"`  // 所有渠道中最高的 quota_pct（最告警的渠道）
	TotalRemainingUSD float64      `json:"total_remaining_usd"` // 所有渠道剩余配额 USD 总和
	AlertsCount    int             `json:"alerts_count"`     // 非 normal 状态的渠道数量
	UpdatedAt      string          `json:"updated_at"`
}

// Dashboard returns a compact dashboard JSON for the iOS widget.
// Cache: public, max-age=60 (Cloudflare 边缘缓存 60 秒减压)
func (h *WidgetHandler) Dashboard(c *gin.Context) {
	key := c.Query("key")
	if !strings.HasPrefix(key, "sk-") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid key"})
		return
	}

	sum := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(sum[:])

	// 查 api_key + user，要求 role == admin
	type authRow struct {
		UserID   string `gorm:"column:user_id"`
		UserRole string `gorm:"column:user_role"`
		IsActive bool   `gorm:"column:is_active"`
	}
	var row authRow
	err := h.db.Table("api_keys").
		Select("api_keys.user_id, users.role AS user_role, api_keys.is_active").
		Joins("JOIN users ON users.id = api_keys.user_id").
		Where("api_keys.key_hash = ?", keyHash).
		First(&row).Error
	if err != nil || !row.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid key"})
		return
	}
	if row.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	// 查所有启用的渠道
	var channels []models.UpstreamChannel
	if err := h.db.Where("is_enabled = ?", true).Order("name").Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	resp := widgetDashboardResp{
		Channels:  make([]widgetChannel, 0, len(channels)),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	var maxPct float64
	for _, ch := range channels {
		pct := computeQuotaPct(ch)
		rem := computeRemainingUSD(ch)
		resp.Channels = append(resp.Channels, widgetChannel{
			Name:         ch.Name,
			Status:       ch.QuotaStatus,
			QuotaPct:     pct,
			TodayCNY:     ch.DailyCostCNY,
			RemainingUSD: rem,
		})
		resp.TotalRemainingUSD += rem
		resp.TotalTodayCNY += ch.DailyCostCNY
		resp.TotalMonthCNY += ch.MonthlyCostCNY
		if pct > maxPct {
			maxPct = pct
		}
		if ch.QuotaStatus != "normal" && ch.QuotaStatus != "" {
			resp.AlertsCount++
		}
	}
	resp.TotalQuotaPct = round1(maxPct)
	resp.TotalTodayCNY = round2(resp.TotalTodayCNY)
	resp.TotalMonthCNY = round2(resp.TotalMonthCNY)
	resp.TotalRemainingUSD = round4(resp.TotalRemainingUSD)

	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, resp)
}

func computeQuotaPct(ch models.UpstreamChannel) float64 {
	switch ch.QuotaType {
	case "daily":
		if ch.DailyQuotaUSD > 0 {
			return round1(ch.QuotaUsedTodayUSD / ch.DailyQuotaUSD * 100)
		}
	case "fixed":
		if ch.TotalQuotaUSD > 0 {
			return round1(ch.UsedTotalUSD / ch.TotalQuotaUSD * 100)
		}
	}
	return 0
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}

func computeRemainingUSD(ch models.UpstreamChannel) float64 {
	// reconcile_multiplier: 实际比例(quota_used_today_usd / 上游真实消耗)
	// 反算上游真实消耗 = quota_used_today_usd / reconcile_multiplier
	mult := ch.ReconcileMultiplier
	if mult <= 0 {
		mult = 1.0
	}
	switch ch.QuotaType {
	case "daily":
		upstreamUsed := ch.QuotaUsedTodayUSD / mult
		r := ch.DailyQuotaUSD - upstreamUsed
		if r < 0 {
			r = 0
		}
		return round4(r)
	case "fixed":
		upstreamUsed := ch.UsedTotalUSD / mult
		r := ch.TotalQuotaUSD - upstreamUsed
		if r < 0 {
			r = 0
		}
		return round4(r)
	}
	return 0
}
