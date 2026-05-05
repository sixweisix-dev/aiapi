package channelmetrics

import (
	"sync"
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
	mail    *monitoring.MailAlerter
}

func New(db *gorm.DB, alerter *monitoring.TelegramAlerter, bark *monitoring.BarkNotifier, mail *monitoring.MailAlerter) *Tracker {
	return &Tracker{db: db, alerter: alerter, bark: bark, mail: mail}
}

// RecordSuccess 记录一次成功请求 (扣费成功后调用)
// costCNY: 本次费用(CNY); cacheReadTokens/cacheTotalTokens: 用于缓存命中率
func (t *Tracker) RecordSuccess(channelID string, costCNY float64, cacheReadTokens, totalInputTokens int, latencyMs int64) {
	if channelID == "" {
		return
	}
	// 1. 重置每日 (每个北京时间 0 点)
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
	t.checkTotalQuota()
	t.checkSubscriptionExpiry()
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

// checkDailyReset 每天 00:00 (Asia/Shanghai, UTC+8) 重置 daily_cost / quota_used_today
func (t *Tracker) checkDailyReset(channelID string) {
	var ch models.UpstreamChannel
	if err := t.db.First(&ch, "id = ?", channelID).Error; err != nil {
		return
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if ch.QuotaResetAt == nil || ch.QuotaResetAt.Before(today) {
		resetTotalAlert()
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
		if strings.Contains(msg, "🔴") || strings.Contains(msg, "🚨") {
			level = "timeSensitive"
		} else if strings.Contains(msg, "🟠") {
			level = "timeSensitive"
		}
		title := "TransitAI 渠道告警"
		t.bark.SendWithLevel(title, msg, level)
	}
	// 重要告警发邮件 (🔴/🚨/🟠), 黄色 🟡 不发邮件避免噪音
	if t.mail != nil && (strings.Contains(msg, "🔴") || strings.Contains(msg, "🚨") || strings.Contains(msg, "🟠")) {
		subject := "[TransitAI] 渠道告警"
		t.mail.Send(subject, msg)
	}
	log.Printf("[channelmetrics] %s", msg)
}


// 总体额度告警阈值
const (
	TotalCriticalThreshold = 0.80 // 总用量 >= 80% 就告警
	TotalEmergencyThreshold = 0.90 // 总用量 >= 90% 强制告警(timeSensitive)
)

// 进程内防抖: 同一阈值只通知一次, 直到下次重置
var lastTotalAlert string
var lastTotalAlertMu sync.Mutex

// checkTotalQuota 检查所有启用渠道的总余额状况
func (t *Tracker) checkTotalQuota() {
	var channels []models.UpstreamChannel
	if err := t.db.Where("is_enabled = ? AND quota_type != ?", true, "unlimited").Find(&channels).Error; err != nil {
		return
	}
	if len(channels) == 0 {
		return
	}

	var totalQuota, totalUsed float64
	for _, c := range channels {
		switch c.QuotaType {
		case "daily":
			totalQuota += c.DailyQuotaUSD
			totalUsed += c.QuotaUsedTodayUSD
		case "fixed":
			totalQuota += c.TotalQuotaUSD
			totalUsed += c.UsedTotalUSD
		}
	}
	if totalQuota <= 0 {
		return
	}
	ratio := totalUsed / totalQuota
	remainingPct := (1 - ratio) * 100

	var tier string
	switch {
	case ratio >= TotalEmergencyThreshold:
		tier = "emergency" // 剩余 < 10%
	case ratio >= TotalCriticalThreshold:
		tier = "critical"  // 剩余 < 20%
	default:
		tier = "normal"
	}
	if tier == "normal" {
		return
	}

	lastTotalAlertMu.Lock()
	defer lastTotalAlertMu.Unlock()
	if lastTotalAlert == tier {
		return // 防抖
	}
	lastTotalAlert = tier

	var emoji, levelMsg string
	switch tier {
	case "emergency":
		emoji = "🚨"
		levelMsg = "总余额紧急"
	case "critical":
		emoji = "🟠"
		levelMsg = "总余额预警"
	}
	t.notify(fmt.Sprintf("%s %s\n所有上游渠道合计:\n已用 %.2f / %.2f USD\n剩余 %.0f%% (%d 个渠道)",
		emoji, levelMsg, totalUsed, totalQuota, remainingPct, len(channels)))
}

// resetTotalAlert 清空防抖状态(午夜或人工重置 quota 时调用)
func resetTotalAlert() {
	lastTotalAlertMu.Lock()
	defer lastTotalAlertMu.Unlock()
	lastTotalAlert = ""
}


// 订阅到期告警 - 进程内防抖 (key: channelID + tier)
var subAlertSent = make(map[string]bool)
var subAlertMu sync.Mutex

// checkSubscriptionExpiry 检查所有渠道订阅到期情况
// 调用频率: 每天 1 次或扣费时顺便调用
func (t *Tracker) checkSubscriptionExpiry() {
	var channels []models.UpstreamChannel
	if err := t.db.Where("is_enabled = ? AND subscription_end IS NOT NULL", true).Find(&channels).Error; err != nil {
		return
	}
	now := time.Now()
	for _, c := range channels {
		if c.SubscriptionEnd == nil {
			continue
		}
		days := int(c.SubscriptionEnd.Sub(now).Hours() / 24)
		var tier string
		switch {
		case days < 0:
			tier = "expired" // 已过期
		case days <= 1:
			tier = "1day"
		case days <= 3:
			tier = "3days"
		case days <= 7:
			tier = "7days"
		default:
			continue
		}
		key := c.ID.String() + ":" + tier
		subAlertMu.Lock()
		if subAlertSent[key] {
			subAlertMu.Unlock()
			continue
		}
		subAlertSent[key] = true
		subAlertMu.Unlock()

		var msg string
		switch tier {
		case "expired":
			msg = fmt.Sprintf("🚨 渠道订阅已过期: %s\n到期时间: %s\n请尽快续费", c.Name, c.SubscriptionEnd.Format("2006-01-02"))
		case "1day":
			msg = fmt.Sprintf("🔴 渠道明天到期: %s\n到期时间: %s\n请抓紧续费", c.Name, c.SubscriptionEnd.Format("2006-01-02"))
		case "3days":
			msg = fmt.Sprintf("🟠 渠道 3 天后到期: %s\n到期时间: %s", c.Name, c.SubscriptionEnd.Format("2006-01-02"))
		case "7days":
			msg = fmt.Sprintf("🟡 渠道 7 天后到期: %s\n到期时间: %s", c.Name, c.SubscriptionEnd.Format("2006-01-02"))
		}
		t.notify(msg)
	}
}
