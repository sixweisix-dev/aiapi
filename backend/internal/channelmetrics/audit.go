package channelmetrics

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-api-gateway/internal/models"
	"github.com/google/uuid"
)

// 审计阈值常量
const (
	BigCostThresholdCNY     = 50.0   // 单笔超 ¥50 记审计
	HighRPMWindowMinutes    = 5      // 5 分钟窗口
	HighRPMThreshold        = 100    // 5 分钟内 > 100 次请求记审计
	HighFailureRateMinutes  = 5
	HighFailureRateMin      = 0.5    // 失败率 >= 50% 记审计
	HighFailureMinSamples   = 10     // 至少 10 次请求才计算失败率
)

// 用户级滑窗计数 (per-user, 进程内, 简化版)
type userWindow struct {
	requests []time.Time
	failures []time.Time
}

// 注: 进程重启会丢失计数, 简化实现; 生产可用 Redis
var (
	userWindows = make(map[string]*userWindow)
	windowMu    sync.Mutex
)

// AuditBigCost 单笔扣费 > 50 元
func (t *Tracker) AuditBigCost(userID, modelName, channelID string, costCNY float64, promptTokens, completionTokens int) {
	if costCNY < BigCostThresholdCNY {
		return
	}
	t.writeAudit(userID, "big_cost_request", map[string]interface{}{
		"cost_cny":          costCNY,
		"model":             modelName,
		"channel_id":        channelID,
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"threshold":         BigCostThresholdCNY,
	})
	log.Printf("[audit] big_cost user=%s cost=%.2f model=%s", userID, costCNY, modelName)
}

// AuditHighRPM 单用户 5 分钟 RPM > 阈值
func (t *Tracker) AuditHighRPM(userID string) {
	now := time.Now()
	windowMu.Lock()
	w, ok := userWindows[userID]
	if !ok {
		w = &userWindow{}
		userWindows[userID] = w
	}
	w.requests = append(w.requests, now)
	// 修剪 5 分钟前的
	cutoff := now.Add(-HighRPMWindowMinutes * time.Minute)
	trimmed := w.requests[:0]
	for _, ts := range w.requests {
		if ts.After(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	w.requests = trimmed
	count := len(trimmed)
	windowMu.Unlock()

	if count >= HighRPMThreshold {
		t.writeAudit(userID, "high_rpm", map[string]interface{}{
			"rpm_5min":  count,
			"threshold": HighRPMThreshold,
		})
		log.Printf("[audit] high_rpm user=%s 5min_count=%d", userID, count)
	}
}

// AuditFailureRate 失败率高
func (t *Tracker) AuditFailureRate(userID string, statusCode int) {
	now := time.Now()
	windowMu.Lock()
	w, ok := userWindows[userID]
	if !ok {
		w = &userWindow{}
		userWindows[userID] = w
	}
	if statusCode >= 400 {
		w.failures = append(w.failures, now)
	}
	cutoff := now.Add(-HighFailureRateMinutes * time.Minute)
	trimmedF := w.failures[:0]
	for _, ts := range w.failures {
		if ts.After(cutoff) {
			trimmedF = append(trimmedF, ts)
		}
	}
	w.failures = trimmedF

	total := len(w.requests)
	failures := len(trimmedF)
	windowMu.Unlock()

	if total >= HighFailureMinSamples && total > 0 {
		rate := float64(failures) / float64(total)
		if rate >= HighFailureRateMin {
			t.writeAudit(userID, "high_failure_rate", map[string]interface{}{
				"rate":      fmt.Sprintf("%.2f", rate),
				"failures":  failures,
				"total":     total,
				"threshold": HighFailureRateMin,
			})
			log.Printf("[audit] high_failure_rate user=%s rate=%.2f", userID, rate)
		}
	}
}

func (t *Tracker) writeAudit(userIDStr, action string, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	userUUID, err := uuid.Parse(userIDStr)
	var userIDPtr *uuid.UUID
	if err == nil {
		userIDPtr = &userUUID
	}
	detailsBytes := []byte(detailsJSON)
	record := &models.AuditLog{
		UserID:    userIDPtr,
		Action:    action,
		NewValues: &detailsBytes,
	}
	if err := t.db.Create(record).Error; err != nil {
		log.Printf("[audit] write failed: %v", err)
	}
}
