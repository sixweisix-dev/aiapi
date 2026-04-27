package monitoring

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"ai-api-gateway/internal/models"
)

// Monitor periodically checks system health and sends alerts.
type Monitor struct {
	db       *gorm.DB
	alerter  *TelegramAlerter
	interval time.Duration
}

func NewMonitor(db *gorm.DB, alerter *TelegramAlerter, interval time.Duration) *Monitor {
	return &Monitor{
		db:       db,
		alerter:  alerter,
		interval: interval,
	}
}

// Start launches the monitoring loop in a background goroutine.
func (m *Monitor) Start(ctx context.Context) {
	if m.alerter == nil {
		log.Println("Monitor: alerter not configured, skipping monitor goroutine")
		return
	}

	go func() {
		// Initial check after services have started
		time.Sleep(30 * time.Second)
		m.runCheck()

		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.runCheck()
			case <-ctx.Done():
				log.Println("Monitor: stopped")
				return
			}
		}
	}()

	log.Printf("Monitor: started (interval: %v)", m.interval)
}

func (m *Monitor) runCheck() {
	// 月度预算重置：把上月以前的 period_start 全部刷成本月，恢复因超额禁用的 key
	result := m.db.Exec(`
		UPDATE api_keys
		SET budget_used = 0,
		    budget_alerted = false,
		    budget_period_start = date_trunc('month', NOW()),
		    is_active = CASE 
		      WHEN is_active = false AND monthly_budget IS NOT NULL AND budget_used >= monthly_budget 
		      THEN true ELSE is_active END
		WHERE budget_period_start < date_trunc('month', NOW())
		  AND monthly_budget IS NOT NULL`)
	if result.RowsAffected > 0 {
		log.Printf("[monitor] reset budget for %d API keys (new month)", result.RowsAffected)
		if m.alerter != nil {
			m.alerter.Send(fmt.Sprintf("📅 <b>月度预算重置</b>\n\n已重置 %d 个 API Key 的预算计数和告警状态", result.RowsAffected))
		}
	}

	// 会员到期自动降级回 free
	expired := m.db.Exec(`
		UPDATE users 
		SET membership_tier = 'free' 
		WHERE membership_tier != 'free' 
		  AND membership_expires_at IS NOT NULL 
		  AND membership_expires_at < NOW()`)
	if expired.RowsAffected > 0 {
		log.Printf("[monitor] downgraded %d expired memberships to free", expired.RowsAffected)
		if m.alerter != nil {
			m.alerter.Send(fmt.Sprintf("⏰ <b>会员到期降级</b>\n\n%d 个会员已到期，自动降级为免费版", expired.RowsAffected))
		}
	}

	alerts := m.collectAlerts()
	if len(alerts) > 0 {
		msg := "<b>🔔 AI Gateway 告警</b>\n"
		for _, a := range alerts {
			msg += "• " + a + "\n"
		}
		if err := m.alerter.Send(msg); err != nil {
			log.Printf("Monitor: send alert failed: %v", err)
		}
	}
}

func (m *Monitor) collectAlerts() []string {
	var alerts []string

	// 1. Unhealthy upstream channels
	var unhealthyChannels []struct {
		Name     string
		Provider string
		BaseURL  string
	}
	m.db.Model(&models.UpstreamChannel{}).
		Where("health_status = ? AND is_enabled = ?", "unhealthy", true).
		Find(&unhealthyChannels)
	for _, ch := range unhealthyChannels {
		alerts = append(alerts, fmt.Sprintf("🔴 上游不可用: [%s] %s (%s)", ch.Provider, ch.Name, ch.BaseURL))
	}

	// 2. All channels for a provider are unhealthy
	type providerHealth struct {
		Provider string
		Total    int64
		Healthy  int64
	}
	var providerStats []providerHealth
	m.db.Model(&models.UpstreamChannel{}).
		Select("provider, COUNT(*) as total, SUM(CASE WHEN health_status = 'healthy' THEN 1 ELSE 0 END) as healthy").
		Where("is_enabled = ?", true).
		Group("provider").
		Having("COUNT(*) = SUM(CASE WHEN health_status = 'healthy' THEN 0 ELSE 1 END)").
		Find(&providerStats)
	for _, ps := range providerStats {
		alerts = append(alerts, fmt.Sprintf("🚨 提供商全面宕机: %s (%d/0 可用)", ps.Provider, ps.Total))
	}

	// 3. Error rate spike in the last 5 minutes
	var errCount, totalCount int64
	cutoff := time.Now().Add(-5 * time.Minute)
	m.db.Model(&models.Request{}).Where("created_at > ? AND status_code >= 500", cutoff).Count(&errCount)
	m.db.Model(&models.Request{}).Where("created_at > ?", cutoff).Count(&totalCount)
	if totalCount > 0 {
		failRate := float64(errCount) / float64(totalCount) * 100
		if failRate > 20 {
			alerts = append(alerts, fmt.Sprintf("⚠️ 错误率过高: %.1f%% (%d/%d) 最近5分钟", failRate, errCount, totalCount))
		}
	}
	if errCount > 50 {
		alerts = append(alerts, fmt.Sprintf("⚠️ 大量5xx错误: %d 次 最近5分钟", errCount))
	}

	// 4. Slow requests (P95 > 30s)
	var slowCount int64
	m.db.Model(&models.Request{}).
		Where("created_at > ? AND duration_ms > ?", cutoff, 30000).
		Count(&slowCount)
	if slowCount > 10 {
		alerts = append(alerts, fmt.Sprintf("🐢 大量慢请求: %d 个请求耗时超过30秒", slowCount))
	}

	return alerts
}
