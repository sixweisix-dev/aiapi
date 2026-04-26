package middleware

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContentFilter 提供基于关键词的内容过滤
type ContentFilter struct {
	db        *gorm.DB
	mu        sync.RWMutex
	keywords  []sensitiveKeyword
	lastLoad  time.Time
	reloadTTL time.Duration
}

type sensitiveKeyword struct {
	ID       uuid.UUID
	Keyword  string
	Category string
	Severity int
}

// FilterResult 过滤结果
type FilterResult struct {
	Blocked       bool   // 是否拦截
	ShouldBlacklist bool // 是否应拉黑
	Category      string // 命中类别
	MatchedKeyword string // 命中关键词
	Severity      int    // 严重程度 1=警告 2=拦截 3=拉黑
}

func NewContentFilter(db *gorm.DB) *ContentFilter {
	cf := &ContentFilter{
		db:        db,
		reloadTTL: 5 * time.Minute,
	}
	cf.reload()
	go cf.periodicReload()
	return cf
}

func (cf *ContentFilter) periodicReload() {
	ticker := time.NewTicker(cf.reloadTTL)
	defer ticker.Stop()
	for range ticker.C {
		cf.reload()
	}
}

func (cf *ContentFilter) reload() {
	var rows []sensitiveKeyword
	err := cf.db.Table("sensitive_keywords").
		Select("id, keyword, category, severity").
		Where("is_enabled = ?", true).
		Find(&rows).Error
	if err != nil {
		log.Printf("[content_filter] reload failed: %v", err)
		return
	}
	cf.mu.Lock()
	cf.keywords = rows
	cf.lastLoad = time.Now()
	cf.mu.Unlock()
	log.Printf("[content_filter] loaded %d keywords", len(rows))
}

// Check 检查文本是否含敏感词（不区分大小写）
func (cf *ContentFilter) Check(text string) *FilterResult {
	if text == "" {
		return &FilterResult{Blocked: false}
	}
	lowerText := strings.ToLower(text)

	cf.mu.RLock()
	defer cf.mu.RUnlock()

	// 优先匹配高 severity（按 severity 降序遍历）
	var bestHit *sensitiveKeyword
	for i := range cf.keywords {
		k := &cf.keywords[i]
		if strings.Contains(lowerText, strings.ToLower(k.Keyword)) {
			if bestHit == nil || k.Severity > bestHit.Severity {
				bestHit = k
			}
		}
	}

	if bestHit == nil {
		return &FilterResult{Blocked: false}
	}

	return &FilterResult{
		Blocked:         bestHit.Severity >= 2,
		ShouldBlacklist: bestHit.Severity >= 3,
		Category:        bestHit.Category,
		MatchedKeyword:  bestHit.Keyword,
		Severity:        bestHit.Severity,
	}
}

// LogViolation 记录违规日志（异步，避免阻塞请求）
func (cf *ContentFilter) LogViolation(userID uuid.UUID, apiKeyID *uuid.UUID, violationType, matchedKeyword, snippet, ipAddress string) {
	go func() {
		// 截断 snippet 避免存太长
		if len(snippet) > 500 {
			snippet = snippet[:500] + "..."
		}
		cf.db.Exec(`
			INSERT INTO violation_logs (user_id, api_key_id, violation_type, matched_keyword, request_snippet, ip_address, created_at)
			VALUES (?, ?, ?, ?, ?, ?::inet, NOW())`,
			userID, apiKeyID, violationType, matchedKeyword, snippet, ipAddress,
		)
	}()
}

// IncrementViolation 增加用户违规计数，达到阈值自动拉黑
// 返回当前违规次数和是否被拉黑
func (cf *ContentFilter) IncrementViolation(userID uuid.UUID, reason string) (count int64, blacklisted bool) {
	const threshold = 5

	// 原子增加 violation_count
	cf.db.Exec("UPDATE users SET violation_count = violation_count + 1 WHERE id = ?", userID)

	// 读取当前次数
	cf.db.Raw("SELECT violation_count FROM users WHERE id = ?", userID).Scan(&count)

	// 达到阈值自动拉黑
	if count >= threshold {
		cf.db.Exec(`
			UPDATE users 
			SET is_active = false, blacklist_reason = ?, blacklisted_at = NOW() 
			WHERE id = ? AND is_active = true`,
			fmt.Sprintf("累计违规 %d 次: %s", count, reason), userID)
		return count, true
	}
	return count, false
}

// MarkBlacklist 直接拉黑用户（高严重度 severity=3 直接调用）
func (cf *ContentFilter) MarkBlacklist(userID uuid.UUID, reason string) {
	cf.db.Exec(`
		UPDATE users 
		SET is_active = false, 
		    violation_count = violation_count + 10, 
		    blacklist_reason = ?, 
		    blacklisted_at = NOW() 
		WHERE id = ?`,
		reason, userID)
}

// IsBlacklisted 检查用户是否在黑名单
func (cf *ContentFilter) IsBlacklisted(userID uuid.UUID) (bool, string) {
	var u struct {
		IsActive        bool
		BlacklistReason *string
	}
	err := cf.db.Raw("SELECT is_active, blacklist_reason FROM users WHERE id = ?", userID).Scan(&u).Error
	if err != nil {
		return false, ""
	}
	if !u.IsActive {
		reason := ""
		if u.BlacklistReason != nil {
			reason = *u.BlacklistReason
		}
		return true, reason
	}
	return false, ""
}

// 让 GORM 知道这个结构对应哪张表
func (sensitiveKeyword) TableName() string {
	return "sensitive_keywords"
}

var _ = context.Background // keep import
