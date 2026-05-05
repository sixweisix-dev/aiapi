package channelmetrics

import (
	"strings"
	"fmt"
	"log"
	"time"

	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/monitoring"
	"gorm.io/gorm"
)

// USD 汇率(简化, 后续可从 env 读)
const cnyPerUSD = 7.2

// 三级阈值
const (
	WarningThreshold  = 0.80 // 80% 预警
	CriticalThreshold = 0.90 // 90% 暂停路由
	ExhaustedThreshold = 1.0 // 100% 禁用
	ErrorStreakLimit  = 5    // 连续 5 次失败自动禁用
)

// Tracker 跟踪渠道指标
type Tracker struct {
	db      *gorm.DB
	alerter *monitoring.TelegramAlerter
	bark    *monitoring.BarkNotifier
}

func New(db *gorm.DB, alerter *monitoring.TelegramAlerter, bark *monitoring.BarkNotifier) *Tracker {
	return &Tracker{db: db, alerter: alerter, bark: bark}
}

// RecordSuccess 记录一次成功请求 (扣费成功后调用)
// costCNY: 本次费用(CNY); cacheReadTokens/cacheTotalTokens: 用于缓存命中率
func (t *Tracker) RecordSuccess(channelID string, costCNY float64, cacheReadTokens, totalInputTokens int, latencyMs int64) {
	if channelID == "" {
		return
	}
	// 1. 重置每日 (每个 UTC 0 点)
	t.checkDailyReset(channelID)

	// 2. 累加成本与额度
	costUSD := costCNY / cnyPerUSD
	updates := map[string]interface{}{
		"daily_cost_cny":       gorm.Expr("daily_cost_cny + ?", costCNY),
		"monthly_cost_cny":     gorm.Expr("monthly_cost_cny + ?", costCNY),
		"quota_used_today_usd": gorm.Expr("quota_used_today_usd + ?", costUSD),
		"used_total_usd":       gorm.Expr("used_total_usd + ?", costUSD),
		"cache_hit_tokens":   gorm.Expr("cache_hit_tokens + ?", cacheReadTokens),
		"cache_total_tokens": gorm.Expr("cache_total_tokens + ?", totalInputTokens),
		"error_streak":       0,
		"total_requests":     gorm.Expr("total_requests + 1"),
	}
	if err := t.db.Model(&models.UpstreamChannel{}).Where("id = ?", channelID).Updates(updates).Error; err != nil {
		log.Printf("[channelmetrics] update success failed: %v", err)
		return
	}

	// 3. 检查阈值
	t.checkQuotaThreshold(channelID)
}

// RecordFailure 记录一次失败 (上游 5xx/timeout)
func (t *Tracker) RecordFailure(channelID string, statusCode int) {
	if channelID == "" {
		return
	}
	var ch models.UpstreamChannel
	if err := t.db.First(&ch, "id = ?", channelID).Error; err != nil {
		return
	}
	newStreak := ch.ErrorStreak + 1
	updates := map[string]interface{}{
		"error_streak": newStreak,
		"error_count":  gorm.Expr("error_count + 1"),
	}
	if newStreak >= ErrorStreakLimit {
		updates["is_enabled"] = false
		updates["health_status"] = "unhealthy"
		t.notify(fmt.Sprintf("🔴 渠道自动禁用: %s\n连续失败 %d 次, 已暂停\n请人工排查", ch.Name, newStreak))
	}
	t.db.Model(&models.UpstreamChannel{}).Where("id = ?", channelID).Updates(updates)
}

// checkDailyReset 每天 00:00 (UTC) 重置 daily_cost / quota_used_today
func (t *Tracker) checkDailyReset(channelID string) {
	var ch models.UpstreamChannel
	if err := t.db.First(&ch, "id = ?", channelID).Error; err != nil {
		return
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if ch.QuotaResetAt == nil || ch.QuotaResetAt.Before(today) {
		t.db.Model(&ch).Updates(map[string]interface{}{
			"daily_cost_cny":       0,
			"quota_used_today_usd": 0,
			"quota_status":         "normal",
			"quota_reset_at":       now,
		})
	}
	// 月初重置 monthly
	if now.Day() == 1 && (ch.QuotaResetAt == nil || ch.QuotaResetAt.Month() != now.Month()) {
		t.db.Model(&ch).Update("monthly_cost_cny", 0)
	}
}

// checkQuotaThreshold 检查渠道额度阈值, 触发告警
func (t *Tracker) checkQuotaThreshold(channelID string) {
	var ch models.UpstreamChannel
	if err := t.db.First(&ch, "id = ?", channelID).Error; err != nil {
		return
	}
	var ratio float64
	switch ch.QuotaType {
	case "daily":
		if ch.DailyQuotaUSD <= 0 {
			return
		}
		ratio = ch.QuotaUsedTodayUSD / ch.DailyQuotaUSD
	case "fixed":
		if ch.TotalQuotaUSD <= 0 {
			return
		}
		ratio = ch.UsedTotalUSD / ch.TotalQuotaUSD
	default:
		return // unlimited
	}
	newStatus := "normal"
	switch {
	case ratio >= ExhaustedThreshold:
		newStatus = "exhausted"
	case ratio >= CriticalThreshold:
		newStatus = "critical"
	case ratio >= WarningThreshold:
		newStatus = "warning"
	}
	if newStatus != ch.QuotaStatus {
		t.db.Model(&ch).Update("quota_status", newStatus)
		switch newStatus {
		case "warning":
			t.notify(fmt.Sprintf("🟡 渠道余额预警: %s\n已用 %.2f / %.2f USD (%.0f%%)", ch.Name, ch.QuotaUsedTodayUSD, ch.DailyQuotaUSD, ratio*100))
		case "critical":
			t.notify(fmt.Sprintf("🟠 渠道余额紧急: %s\n已用 %.2f / %.2f USD (%.0f%%) - 暂停路由", ch.Name, ch.QuotaUsedTodayUSD, ch.DailyQuotaUSD, ratio*100))
		case "exhausted":
			t.notify(fmt.Sprintf("🔴 渠道额度耗尽: %s\n已用 %.2f / %.2f USD - 已禁用", ch.Name, ch.QuotaUsedTodayUSD, ch.DailyQuotaUSD))
			t.db.Model(&ch).Update("is_enabled", false)
		}
	}
}

func (t *Tracker) notify(msg string) {
	if t.alerter != nil {
		t.alerter.Send(msg)
	}
	if t.bark != nil {
		level := "active"
		if strings.Contains(msg, "🔴") {
			level = "timeSensitive" // 锁屏强提醒
		} else if strings.Contains(msg, "🟠") {
			level = "timeSensitive"
		}
		title := "TransitAI 渠道告警"
		t.bark.SendWithLevel(title, msg, level)
	}
	log.Printf("[channelmetrics] %s", msg)
}
