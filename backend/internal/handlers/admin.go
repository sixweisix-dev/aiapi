package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/channelmetrics"
	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/upstream"
)

type AdminHandler struct {
	db     *gorm.DB
	engine *billing.Engine
	pool *upstream.Pool
	tracker *channelmetrics.Tracker
}

func NewAdminHandler(db *gorm.DB, engine *billing.Engine, pool *upstream.Pool, tracker *channelmetrics.Tracker) *AdminHandler {
	return &AdminHandler{db: db, engine: engine, pool: pool, tracker: tracker}
}

// ---- Dashboard ----

// DashboardStats returns key metrics for the admin dashboard.
func (h *AdminHandler) DashboardStats(c *gin.Context) {
	type result struct {
		TotalUsers      int64   `json:"total_users"`
		ActiveUsers     int64   `json:"active_users"`
		TotalRequests   int64   `json:"total_requests"`
		TodayRequests   int64   `json:"today_requests"`
		TotalRevenue    float64 `json:"total_revenue"`
		TodayRevenue    float64 `json:"today_revenue"`
		TotalChannels   int64   `json:"total_channels"`
		OnlineChannels  int64   `json:"online_channels"`
		TotalModels     int64   `json:"total_models"`
		PendingOrders   int64   `json:"pending_orders"`
	}

	var r result
	today := time.Now().Truncate(24 * time.Hour)

	h.db.Model(&models.User{}).Count(&r.TotalUsers)
	h.db.Model(&models.User{}).Where("is_active = ?", true).Count(&r.ActiveUsers)
	h.db.Model(&models.Request{}).Count(&r.TotalRequests)
	h.db.Model(&models.Request{}).Where("created_at >= ?", today).Count(&r.TodayRequests)
	h.db.Model(&models.BillingRecord{}).Where("type = ?", "recharge").Select("COALESCE(SUM(amount),0)").Scan(&r.TotalRevenue)
	h.db.Model(&models.BillingRecord{}).Where("type = ? AND created_at >= ?", "recharge", today).Select("COALESCE(SUM(amount),0)").Scan(&r.TodayRevenue)
	h.db.Model(&models.UpstreamChannel{}).Count(&r.TotalChannels)
	h.db.Model(&models.UpstreamChannel{}).Where("health_status = ? AND is_enabled = ?", "healthy", true).Count(&r.OnlineChannels)
	h.db.Model(&models.Model{}).Count(&r.TotalModels)
	h.db.Model(&models.RechargeOrder{}).Where("payment_status = ?", "pending").Count(&r.PendingOrders)

	c.JSON(http.StatusOK, r)
}

// ---- User Management ----

type UserListItem struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Username            *string    `json:"username"`
	Role                string     `json:"role"`
	Balance             float64    `json:"balance"`
	TotalSpent          float64    `json:"total_spent"`
	RequestCount        int        `json:"request_count"`
	IsActive            bool       `json:"is_active"`
	EmailVerified       bool       `json:"email_verified"`
	MembershipTier      string     `json:"membership_tier"`
	MembershipExpiresAt *time.Time `json:"membership_expires_at"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

type UpdateUserRequest struct {
	Role               *string  `json:"role,omitempty" binding:"omitempty,oneof=user admin"`
	IsActive           *bool    `json:"is_active,omitempty"`
	EmailVerified      *bool    `json:"email_verified,omitempty"`
	BalanceAdjust      *float64 `json:"balance_adjust,omitempty"` // positive = add, negative = subtract
	MembershipTier     *string  `json:"membership_tier,omitempty" binding:"omitempty,oneof=free pro enterprise"`
	MembershipDays     *int     `json:"membership_days,omitempty"` // 0=clear, >0=set/extend days from now
}

// ListUsers returns paginated user list with optional search.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "20"), 20)
	search := c.Query("search")
	role := c.Query("role")

	query := h.db.Model(&models.User{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("email ILIKE ? OR CAST(username AS text) ILIKE ?", like, like)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)

	items := make([]UserListItem, 0, len(users))
	for _, u := range users {
		items = append(items, UserListItem{
			ID:                  u.ID.String(),
			Email:               u.Email,
			Username:            u.Username,
			Role:                u.Role,
			Balance:             u.Balance,
			TotalSpent:          u.TotalSpent,
			RequestCount:        u.RequestCount,
			IsActive:            u.IsActive,
			EmailVerified:       u.EmailVerified,
			MembershipTier:      u.MembershipTier,
			MembershipExpiresAt: u.MembershipExpiresAt,
			LastLoginAt:         u.LastLoginAt,
			CreatedAt:           u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// GetUser returns a single user's detail.
func (h *AdminHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := h.db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, UserListItem{
		ID:            user.ID.String(),
		Email:         user.Email,
		Username:      user.Username,
		Role:          user.Role,
		Balance:       user.Balance,
		TotalSpent:    user.TotalSpent,
		RequestCount:  user.RequestCount,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		LastLoginAt:   user.LastLoginAt,
		CreatedAt:     user.CreatedAt,
	})
}

// UserErrorLog 只暴露必要字段, 避免把 request_body / response_body 全推给前端
type UserErrorLog struct {
	ID                string          `json:"id"`
	CreatedAt         time.Time       `json:"created_at"`
	Path              string          `json:"path"`
	StatusCode        int             `json:"status_code"`
	ModelName         string          `json:"model_name"`
	ErrorMessage      string          `json:"error_message"`
	DurationMs        int             `json:"duration_ms"`
	UserAgent         string          `json:"user_agent"`
	IPAddress         string          `json:"ip_address"`
	UpstreamChannelID string          `json:"upstream_channel_id"`
	RequestBody       json.RawMessage `json:"request_body"`
	ResponseBody      json.RawMessage `json:"response_body"`
}

// ListUserErrorLogs returns recent 4xx/5xx or error_message-bearing requests for a user
func (h *AdminHandler) ListUserErrorLogs(c *gin.Context) {
	userID := c.Param("id")
	limit := parseInt(c.DefaultQuery("limit", "50"), 50)
	if limit > 200 {
		limit = 200
	}

	type row struct {
		ID                string
		CreatedAt         time.Time
		Path              string
		StatusCode        int
		ModelName         *string
		ErrorMessage      *string
		DurationMs        int
		UserAgent         *string
		IPAddress         *string
		UpstreamChannelID *string
		RequestBody       []byte
		ResponseBody      []byte
	}
	var rows []row
	err := h.db.Table("requests r").
		Select("r.id, r.created_at, r.path, r.status_code, m.name as model_name, r.error_message, r.duration_ms, r.user_agent, r.ip_address::text as ip_address, r.upstream_channel_id::text as upstream_channel_id, r.request_body, r.response_body").
		Joins("LEFT JOIN models m ON m.id = r.model_id").
		Where("r.user_id = ? AND (r.status_code >= 400 OR r.error_message IS NOT NULL AND r.error_message <> '')", userID).
		Order("r.created_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]UserErrorLog, 0, len(rows))
	for _, r := range rows {
		deref := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		result = append(result, UserErrorLog{
			ID:                r.ID,
			CreatedAt:         r.CreatedAt,
			Path:              r.Path,
			StatusCode:        r.StatusCode,
			ModelName:         deref(r.ModelName),
			ErrorMessage:      deref(r.ErrorMessage),
			DurationMs:        r.DurationMs,
			UserAgent:         deref(r.UserAgent),
			IPAddress:         deref(r.IPAddress),
			UpstreamChannelID: deref(r.UpstreamChannelID),
			RequestBody:       r.RequestBody,
			ResponseBody:      r.ResponseBody,
		})
	}
	c.JSON(http.StatusOK, gin.H{"logs": result, "total": len(result)})
}

// UpdateUser updates user fields. Supports balance adjustment.
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	adminID := c.GetString("user_id")

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.EmailVerified != nil {
		updates["email_verified"] = *req.EmailVerified
	}
	if req.MembershipTier != nil || req.MembershipDays != nil {
		tier := "free"
		if req.MembershipTier != nil {
			tier = *req.MembershipTier
		}
		updates["membership_tier"] = tier
		if tier == "free" || (req.MembershipDays != nil && *req.MembershipDays == 0) {
			updates["membership_expires_at"] = nil
		} else if req.MembershipDays != nil && *req.MembershipDays > 0 {
			expiry := time.Now().AddDate(0, 0, *req.MembershipDays)
			updates["membership_expires_at"] = expiry
		}
	}

	// Balance adjustment in transaction
	if req.BalanceAdjust != nil && *req.BalanceAdjust != 0 {
		err := h.db.Transaction(func(tx *gorm.DB) error {
			var before float64
			tx.Model(&models.User{}).Select("balance").Where("id = ?", id).Scan(&before)

			if err := tx.Model(&models.User{}).Where("id = ?", id).
				Update("balance", gorm.Expr("balance + ?", *req.BalanceAdjust)).Error; err != nil {
				return err
			}
			after := before + *req.BalanceAdjust
			desc := "管理员调整余额"
			if *req.BalanceAdjust > 0 {
				desc = "管理员增加余额"
			} else {
				desc = "管理员扣除余额"
			}
			billingType := "adjustment"
			record := &models.BillingRecord{
				UserID:        user.ID,
				Type:          billingType,
				Amount:        *req.BalanceAdjust,
				BalanceBefore: before,
				BalanceAfter:  after,
				Description:   &desc,
			}
			return tx.Create(record).Error
		})
		if err != nil {
			log.Printf("UpdateUser balance error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update balance"})
			return
		}
		// 同步 Redis 热余额: 防止 playground/API 读旧 redis 值覆盖 admin 修改
		if h.engine != nil {
			if syncErr := h.engine.InitBalance(id); syncErr != nil {
				log.Printf("UpdateUser: redis balance sync failed for %s: %v", id, syncErr)
			}
		}
	}

	if len(updates) > 0 {
		if err := h.db.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
			return
		}
	}

	// Audit log
	h.createAuditLog(adminID, "update_user", "users", id, nil, updates)

	c.JSON(http.StatusOK, gin.H{"message": "user updated"})
}

// ---- Upstream Channel Management ----

type CreateChannelRequest struct {
	Name    string  `json:"name" binding:"required,min=1,max=100"`
	Provider string `json:"provider" binding:"required,oneof=openai anthropic google qwen deepseek"`
	APIKey  string  `json:"api_key" binding:"required"`
	BaseURL *string `json:"base_url,omitempty"`
	Weight  *int    `json:"weight,omitempty"`
	SupportedModels string `json:"supported_models,omitempty"`
	FallbackChannelIDs string `json:"fallback_channel_ids,omitempty"`
}

type UpdateChannelRequest struct {
	Name              *string  `json:"name,omitempty"`
	Provider          *string  `json:"provider,omitempty" binding:"omitempty,oneof=openai anthropic google multi_aggregator vertex_ai"`
	APIKey            *string  `json:"api_key,omitempty"`
	BaseURL           *string  `json:"base_url,omitempty"`
	Weight            *int     `json:"weight,omitempty"`
	IsEnabled         *bool    `json:"is_enabled,omitempty"`
	QuotaType         *string  `json:"quota_type,omitempty"` // unlimited/daily/fixed
	DailyQuotaUSD     *float64 `json:"daily_quota_usd,omitempty"`
	TotalQuotaUSD     *float64 `json:"total_quota_usd,omitempty"`
	SubscriptionStart *string  `json:"subscription_start,omitempty"` // YYYY-MM-DD
	SubscriptionEnd   *string  `json:"subscription_end,omitempty"`
	IsDedicated       *bool    `json:"is_dedicated,omitempty"`
	DedicatedUserIDs  *string  `json:"dedicated_user_ids,omitempty"`
	ReconcileMultiplier *float64 `json:"reconcile_multiplier,omitempty"`
	BillingMode         *string  `json:"billing_mode,omitempty"`
	MonthlyFeeCNY       *float64 `json:"monthly_fee_cny,omitempty"`
	EnableCache1hBeta   *bool    `json:"enable_cache_1h_beta,omitempty"`
	AutoInjectCache     *bool    `json:"auto_inject_cache,omitempty"`
	GroupID             *uint    `json:"group_id,omitempty"`
	GroupIDNull         bool     `json:"-"` // true = 前端传了 null，清空分组
	ResetQuota        *bool    `json:"reset_quota,omitempty"` // 手动重置今日额度
	SupportedModelsP    *string    `json:"supported_models,omitempty"`
	FallbackChannelIDs  *string    `json:"fallback_channel_ids,omitempty"`
}

type ChannelListItem struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Provider            string     `json:"provider"`
	Weight              int        `json:"weight"`
	IsEnabled           bool       `json:"is_enabled"`
	HealthStatus        string     `json:"health_status"`
	LastCheck           *time.Time `json:"last_health_check"`
	TotalReqs           int        `json:"total_requests"`
	TotalTokens         int        `json:"total_tokens"`
	ErrorCount          int        `json:"error_count"`
	CreatedAt           time.Time  `json:"created_at"`

	// 额度
	QuotaType           string     `json:"quota_type"`
	DailyQuotaUSD       float64    `json:"daily_quota_usd"`
	QuotaUsedTodayUSD   float64    `json:"quota_used_today_usd"`
	TotalQuotaUSD       float64    `json:"total_quota_usd"`
	UsedTotalUSD        float64    `json:"used_total_usd"`
	QuotaStatus         string     `json:"quota_status"`
	SubscriptionStart   *time.Time `json:"subscription_start"`
	SubscriptionEnd     *time.Time `json:"subscription_end"`
	ErrorStreak         int        `json:"error_streak"`

	// 成本
	DailyCostCNY        float64    `json:"daily_cost_cny"`
	MonthlyCostCNY      float64    `json:"monthly_cost_cny"`

	// 缓存命中率
	CacheHitTokens      int64      `json:"cache_hit_tokens"`
	CacheTotalTokens    int64      `json:"cache_total_tokens"`
	CacheHitRate        float64    `json:"cache_hit_rate"`

	// 延迟
	AvgLatencyMs        int        `json:"avg_latency_ms"`
	P95LatencyMs        int        `json:"p95_latency_ms"`

	// 专属
	IsDedicated         bool       `json:"is_dedicated"`
	DedicatedUserIDs    string     `json:"dedicated_user_ids"`
	DedicatedUserIDsAuto string    `json:"dedicated_user_ids_auto"`
	ReconcileMultiplier float64    `json:"reconcile_multiplier"`
	BillingMode         string     `json:"billing_mode"`
	MonthlyFeeCNY       float64    `json:"monthly_fee_cny"`
	EnableCache1hBeta   bool       `json:"enable_cache_1h_beta"`
	AutoInjectCache     bool       `json:"auto_inject_cache"`
	GroupID             *uint      `json:"group_id,omitempty"`
	GroupName           string     `json:"group_name,omitempty"`
	SupportedModels     string     `json:"supported_models"`
	FallbackChannelIDs  string     `json:"fallback_channel_ids"`
	Errors1h            int64      `json:"errors_1h"`
}

func (h *AdminHandler) ListChannels(c *gin.Context) {
	// load all channel_groups for name lookup
	var allGroups []models.ChannelGroup
	h.db.Find(&allGroups)
	groupNameMap := make(map[uint]string, len(allGroups))
	for _, g := range allGroups {
		groupNameMap[g.ID] = g.Name
	}
	var channels []models.UpstreamChannel
	h.db.Order("created_at DESC").Find(&channels)
	items := make([]ChannelListItem, 0, len(channels))
	for _, ch := range channels {
		hitRate := float64(0)
		if ch.CacheTotalTokens > 0 {
			hitRate = float64(ch.CacheHitTokens) / float64(ch.CacheTotalTokens)
		}
		items = append(items, ChannelListItem{
			ID:                ch.ID.String(),
			Name:              ch.Name,
			Provider:          ch.Provider,
			Weight:            ch.Weight,
			IsEnabled:         ch.IsEnabled,
			HealthStatus:      ch.HealthStatus,
			LastCheck:         ch.LastHealthCheck,
			TotalReqs:         ch.TotalRequests,
			TotalTokens:       ch.TotalTokens,
			ErrorCount:        ch.ErrorCount,
			CreatedAt:         ch.CreatedAt,
			QuotaType:         ch.QuotaType,
			DailyQuotaUSD:     ch.DailyQuotaUSD,
			QuotaUsedTodayUSD: ch.QuotaUsedTodayUSD,
			TotalQuotaUSD:     ch.TotalQuotaUSD,
			UsedTotalUSD:      ch.UsedTotalUSD,
			QuotaStatus:       ch.QuotaStatus,
			SubscriptionStart: ch.SubscriptionStart,
			SubscriptionEnd:   ch.SubscriptionEnd,
			ErrorStreak:       ch.ErrorStreak,
			DailyCostCNY:      ch.DailyCostCNY,
			MonthlyCostCNY:    ch.MonthlyCostCNY,
			CacheHitTokens:    ch.CacheHitTokens,
			CacheTotalTokens:  ch.CacheTotalTokens,
			CacheHitRate:      hitRate,
			AvgLatencyMs:      ch.AvgLatencyMs,
			P95LatencyMs:      ch.P95LatencyMs,
			IsDedicated:       ch.IsDedicated,
			DedicatedUserIDs:  ch.DedicatedUserIDs,
			DedicatedUserIDsAuto: ch.DedicatedUserIDsAuto,
			ReconcileMultiplier: ch.ReconcileMultiplier,
			BillingMode:         ch.BillingMode,
			MonthlyFeeCNY:       ch.MonthlyFeeCNY,
			EnableCache1hBeta:   ch.EnableCache1hBeta,
			AutoInjectCache:     ch.AutoInjectCache,
			GroupID:             ch.GroupID,
		SupportedModels:     ch.SupportedModels,
			GroupName:           func() string {
				if ch.GroupID != nil {
					if n, ok := groupNameMap[*ch.GroupID]; ok {
						return n
					}
				}
				return ""
			}(),
			Errors1h: func() int64 {
				if h.pool != nil {
					return h.pool.GetErrorsLastHour(ch.ID.String())
				}
				return 0
			}(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminHandler) CreateChannel(c *gin.Context) {
	adminID := c.GetString("user_id")
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel := models.UpstreamChannel{
		Name:            req.Name,
		Provider:        req.Provider,
		APIKeyEncrypted: req.APIKey,
		Weight:          1,
		IsEnabled:       true,
		HealthStatus:    "unknown",
	}
	if req.BaseURL != nil {
		channel.BaseURL = req.BaseURL
	}
	if req.SupportedModels != "" {
		channel.SupportedModels = req.SupportedModels
	}
	if req.Weight != nil {
		channel.Weight = *req.Weight
	}
	if req.FallbackChannelIDs != "" {
		channel.FallbackChannelIDs = req.FallbackChannelIDs
	}

	if err := h.db.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create channel"})
		return
	}

	h.createAuditLog(adminID, "create_channel", "upstream_channels", channel.ID.String(), nil, map[string]interface{}{
		"name": req.Name, "provider": req.Provider,
	})

	c.JSON(http.StatusCreated, ChannelListItem{
		ID: channel.ID.String(), Name: channel.Name, Provider: channel.Provider,
		Weight: channel.Weight, IsEnabled: channel.IsEnabled, HealthStatus: channel.HealthStatus,
		CreatedAt: channel.CreatedAt,
	})
}

func (h *AdminHandler) UpdateChannel(c *gin.Context) {
	adminID := c.GetString("user_id")
	id := c.Param("id")

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var channel models.UpstreamChannel
	if err := h.db.First(&channel, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Provider != nil {
		updates["provider"] = *req.Provider
	}
	if req.APIKey != nil {
		updates["api_key_encrypted"] = *req.APIKey
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.Weight != nil {
		updates["weight"] = *req.Weight
	}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}
	if req.QuotaType != nil {
		updates["quota_type"] = *req.QuotaType
	}
	if req.DailyQuotaUSD != nil {
		updates["daily_quota_usd"] = *req.DailyQuotaUSD
	}
	if req.TotalQuotaUSD != nil {
		updates["total_quota_usd"] = *req.TotalQuotaUSD
		// 修改总额时同步清零 used_total_usd
		if *req.TotalQuotaUSD != channel.TotalQuotaUSD {
			updates["used_total_usd"] = 0
		}
	}
	if req.SubscriptionStart != nil {
		if t, err := time.Parse("2006-01-02", *req.SubscriptionStart); err == nil {
			updates["subscription_start"] = t
		}
	}
	if req.SubscriptionEnd != nil {
		if t, err := time.Parse("2006-01-02", *req.SubscriptionEnd); err == nil {
			updates["subscription_end"] = t
		}
	}
	if req.IsDedicated != nil {
		updates["is_dedicated"] = *req.IsDedicated
	}
	if req.DedicatedUserIDs != nil {
		updates["dedicated_user_ids"] = *req.DedicatedUserIDs
	}
	if req.ReconcileMultiplier != nil && *req.ReconcileMultiplier > 0 {
		updates["reconcile_multiplier"] = *req.ReconcileMultiplier
	}
	if req.BillingMode != nil {
		updates["billing_mode"] = *req.BillingMode
	}
	if req.MonthlyFeeCNY != nil && *req.MonthlyFeeCNY >= 0 {
		updates["monthly_fee_cny"] = *req.MonthlyFeeCNY
	}
	if req.EnableCache1hBeta != nil {
		updates["enable_cache_1h_beta"] = *req.EnableCache1hBeta
	}
	if req.AutoInjectCache != nil {
		updates["auto_inject_cache"] = *req.AutoInjectCache
	}
	if req.SupportedModelsP != nil {
		updates["supported_models"] = *req.SupportedModelsP
	}
	if req.FallbackChannelIDs != nil {
		updates["fallback_channel_ids"] = *req.FallbackChannelIDs
	}
	if req.GroupID != nil {
		if *req.GroupID == 0 {
			updates["group_id"] = nil
		} else {
			updates["group_id"] = *req.GroupID
		}
	}
	if req.ResetQuota != nil && *req.ResetQuota {
		updates["quota_used_today_usd"] = 0
		updates["daily_cost_cny"] = 0
		updates["quota_status"] = "normal"
		updates["error_streak"] = 0
	}

	if len(updates) > 0 {
		h.db.Model(&channel).Updates(updates)
		// 改完 quota 字段强制重算 quota_status (避免 critical 卡死)
		if h.tracker != nil {
			h.tracker.RecheckQuota(id)
		}
	}

	h.createAuditLog(adminID, "update_channel", "upstream_channels", id, nil, updates)
	c.JSON(http.StatusOK, gin.H{"message": "channel updated"})
}

func (h *AdminHandler) DeleteChannel(c *gin.Context) {
	adminID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Delete(&models.UpstreamChannel{}, "id = ?", id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	h.createAuditLog(adminID, "delete_channel", "upstream_channels", id, nil, nil)
	c.JSON(http.StatusOK, gin.H{"message": "channel deleted"})
}

// TestChannel performs a simple connectivity test by pinging the upstream provider.
func (h *AdminHandler) TestChannel(c *gin.Context) {
	id := c.Param("id")
	var channel models.UpstreamChannel
	if err := h.db.First(&channel, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	// Health check: try a simple request to the provider's model list endpoint
	healthy := testProviderConnection(channel)
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	h.db.Model(&channel).Updates(map[string]interface{}{
		"health_status":    status,
		"last_health_check": time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"healthy": healthy,
		"status":  status,
	})
}

// ---- Model Management ----

type CreateModelRequest struct {
	Name          string   `json:"name" binding:"required"`
	DisplayName   string   `json:"display_name" binding:"required"`
	Provider      string   `json:"provider" binding:"required,oneof=openai anthropic google qwen deepseek"`
	ContextLength *int     `json:"context_length,omitempty"`
	InputPrice    float64  `json:"input_price" binding:"gte=0"`
	OutputPrice   float64  `json:"output_price" binding:"gte=0"`
	CostPerCall   *float64 `json:"cost_per_call,omitempty"`
	UpstreamChannelID *string `json:"upstream_channel_id,omitempty"`
	Multiplier    *float64 `json:"multiplier,omitempty"`
	IsPublic      *bool    `json:"is_public,omitempty"`
	Description   *string  `json:"description,omitempty"`
	GroupID       *uint    `json:"group_id,omitempty"`
	UpstreamName  *string  `json:"upstream_name,omitempty"`
}

type UpdateModelRequest struct {
	DisplayName   *string  `json:"display_name,omitempty"`
	InputPrice    *float64 `json:"input_price,omitempty"`
	OutputPrice   *float64 `json:"output_price,omitempty"`
	CostPerCall   *float64 `json:"cost_per_call,omitempty"`
	UpstreamChannelID *string `json:"upstream_channel_id,omitempty"`
	UpstreamChannelIDNull bool `json:"-"` // true = 前端传 null 清空绑定
	Multiplier    *float64 `json:"multiplier,omitempty"`
	IsEnabled     *bool    `json:"is_enabled,omitempty"`
	IsPublic      *bool    `json:"is_public,omitempty"`
	Description   *string  `json:"description,omitempty"`
	GroupID       *uint    `json:"group_id,omitempty"`
	UpstreamName  *string  `json:"upstream_name,omitempty"`
}

type ModelListItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Provider      string   `json:"provider"`
	ContextLength int      `json:"context_length"`
	InputPrice    float64  `json:"input_price"`
	OutputPrice   float64  `json:"output_price"`
	CostPerCall   float64  `json:"cost_per_call"`
	UpstreamChannelID *string `json:"upstream_channel_id,omitempty"`
	UpstreamChannelName string `json:"upstream_channel_name,omitempty"`
	Multiplier    float64  `json:"multiplier"`
	IsEnabled     bool     `json:"is_enabled"`
	IsPublic      bool     `json:"is_public"`
	GroupID       *uint    `json:"group_id,omitempty"`
	GroupName     string   `json:"group_name,omitempty"`
	UpstreamName  *string  `json:"upstream_name,omitempty"`
	Description   *string  `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *AdminHandler) ListModels(c *gin.Context) {
	type rowType struct {
		models.Model
		GroupName           string `gorm:"column:group_name"`
		UpstreamChannelName string `gorm:"column:upstream_channel_name"`
	}
	var rows []rowType
	h.db.Table("models AS m").
		Select("m.*, COALESCE(cg.name,'') AS group_name, COALESCE(uc.name,'') AS upstream_channel_name").
		Joins("LEFT JOIN channel_groups AS cg ON cg.id = m.group_id AND cg.deleted_at IS NULL").
		Joins("LEFT JOIN upstream_channels AS uc ON uc.id = m.upstream_channel_id AND uc.deleted_at IS NULL").
		Where("m.deleted_at IS NULL").
		Order("m.provider ASC, m.name ASC").
		Scan(&rows)
	items := make([]ModelListItem, 0, len(rows))
	for _, r := range rows {
		item := ModelListItem{
			ID:            r.ID.String(),
			Name:          r.Name,
			DisplayName:   r.DisplayName,
			Provider:      r.Provider,
			ContextLength: r.ContextLength,
			InputPrice:    r.InputPrice,
			OutputPrice:   r.OutputPrice,
			CostPerCall:   r.CostPerCall,
			Multiplier:    r.Multiplier,
			IsEnabled:     r.IsEnabled,
			IsPublic:      r.IsPublic,
			Description:   r.Description,
			GroupName:     r.GroupName,
			CreatedAt:     r.CreatedAt,
		}
		if r.GroupID != nil {
			item.GroupID = r.GroupID
		}
		if r.UpstreamName != nil {
			item.UpstreamName = r.UpstreamName
		}
		if r.UpstreamChannelID != nil {
			idStr := r.UpstreamChannelID.String()
			item.UpstreamChannelID = &idStr
			item.UpstreamChannelName = r.UpstreamChannelName
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminHandler) CreateModel(c *gin.Context) {
	adminID := c.GetString("user_id")
	var req CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	model := models.Model{
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		Provider:      req.Provider,
		ContextLength: 4096,
		InputPrice:    req.InputPrice,
		OutputPrice:   req.OutputPrice,
		Multiplier:    1.0,
		IsEnabled:     true,
		IsPublic:      true,
	}
	if req.ContextLength != nil {
		model.ContextLength = *req.ContextLength
	}
	if req.Multiplier != nil {
		model.Multiplier = *req.Multiplier
	}
	if req.IsPublic != nil {
		model.IsPublic = *req.IsPublic
	}
	if req.Description != nil {
		model.Description = req.Description
	}
	if req.CostPerCall != nil {
		model.CostPerCall = *req.CostPerCall
	}
	if req.UpstreamChannelID != nil && *req.UpstreamChannelID != "" {
		if uid, uerr := uuid.Parse(*req.UpstreamChannelID); uerr == nil {
			model.UpstreamChannelID = &uid
		}
	}

	if err := h.db.Create(&model).Error; err != nil {
		msg := err.Error()
		if strings.Contains(msg, "duplicate key") && strings.Contains(msg, "idx_models_name") {
			c.JSON(http.StatusConflict, gin.H{"error": "模型名已存在 (可能被软删除, 请到 DB 直接恢复或换名字)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model: " + msg})
		return
	}

	h.createAuditLog(adminID, "create_model", "models", model.ID.String(), nil, map[string]interface{}{
		"name": req.Name, "provider": req.Provider,
	})

	c.JSON(http.StatusCreated, ModelListItem{
		ID: model.ID.String(), Name: model.Name, DisplayName: model.DisplayName,
		Provider: model.Provider, InputPrice: model.InputPrice, OutputPrice: model.OutputPrice,
		Multiplier: model.Multiplier, IsEnabled: model.IsEnabled, IsPublic: model.IsPublic,
		CreatedAt: model.CreatedAt,
	})
}

func (h *AdminHandler) UpdateModel(c *gin.Context) {
	adminID := c.GetString("user_id")
	id := c.Param("id")
	var req UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var model models.Model
	if err := h.db.First(&model, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.InputPrice != nil {
		updates["input_price"] = *req.InputPrice
	}
	if req.OutputPrice != nil {
		updates["output_price"] = *req.OutputPrice
	}
	if req.CostPerCall != nil {
		updates["cost_per_call"] = *req.CostPerCall
	}
	if req.UpstreamChannelID != nil {
		if *req.UpstreamChannelID == "" {
			updates["upstream_channel_id"] = nil
		} else if uid, uerr := uuid.Parse(*req.UpstreamChannelID); uerr == nil {
			updates["upstream_channel_id"] = uid
		}
	}
	if req.UpstreamName != nil {
		if *req.UpstreamName == "" {
			updates["upstream_name"] = nil
		} else {
			updates["upstream_name"] = *req.UpstreamName
		}
	}
	if req.Multiplier != nil {
		updates["multiplier"] = *req.Multiplier
	}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.GroupID != nil {
		if *req.GroupID == 0 {
			updates["group_id"] = nil
		} else {
			updates["group_id"] = *req.GroupID
		}
	}

	if len(updates) > 0 {
		h.db.Model(&model).Updates(updates)
	}

	h.createAuditLog(adminID, "update_model", "models", id, nil, updates)
	c.JSON(http.StatusOK, gin.H{"message": "model updated"})
}

func (h *AdminHandler) DeleteModel(c *gin.Context) {
	adminID := c.GetString("user_id")
	id := c.Param("id")
	result := h.db.Delete(&models.Model{}, "id = ?", id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}
	h.createAuditLog(adminID, "delete_model", "models", id, nil, nil)
	c.JSON(http.StatusOK, gin.H{"message": "model deleted"})
}

// ---- Request Logs ----

type LogQuery struct {
	UserID     string `form:"user_id"`
	ModelName  string `form:"model"`
	StatusCode int    `form:"status_code"`
	StartDate  string `form:"start_date"`
	EndDate    string `form:"end_date"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

type LogListItem struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	UserEmail        string    `json:"user_email,omitempty"`
	ModelName        string    `json:"model_name"`
	Path             string    `json:"path"`
	StatusCode       int       `json:"status_code"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	Cost             float64   `json:"cost"`
	DurationMs       int       `json:"duration_ms"`
	IPAddress        *string   `json:"ip_address,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

func (h *AdminHandler) ListLogs(c *gin.Context) {
	var q LogQuery
	c.ShouldBindQuery(&q)

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}

	query := h.db.Table("requests").
		Select("requests.id, requests.user_id, users.email AS user_email, models.name AS model_name, requests.path, requests.status_code, requests.prompt_tokens, requests.completion_tokens, requests.total_tokens, requests.cost, requests.duration_ms, requests.ip_address, requests.created_at").
		Joins("LEFT JOIN users ON users.id = requests.user_id").
		Joins("LEFT JOIN models ON models.id = requests.model_id")

	if q.UserID != "" {
		query = query.Where("requests.user_id = ?", q.UserID)
	}
	if q.ModelName != "" {
		query = query.Where("models.name = ?", q.ModelName)
	}
	if q.StatusCode > 0 {
		query = query.Where("requests.status_code = ?", q.StatusCode)
	}
	if q.StartDate != "" {
		query = query.Where("requests.created_at >= ?", q.StartDate)
	}
	if q.EndDate != "" {
		query = query.Where("requests.created_at <= ?", q.EndDate)
	}

	var total int64
	query.Count(&total)

	type logRow struct {
		ID               uuid.UUID
		UserID           uuid.UUID
		UserEmail        string
		ModelName        string
		Path             string
		StatusCode       int
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
		Cost             float64
		DurationMs       int
		IPAddress        *string
		CreatedAt        time.Time
	}

	var rows []logRow
	query.Order("requests.created_at DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).
		Scan(&rows)

	items := make([]LogListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, LogListItem{
			ID:               r.ID.String(),
			UserID:           r.UserID.String(),
			UserEmail:        r.UserEmail,
			ModelName:        r.ModelName,
			Path:             r.Path,
			StatusCode:       r.StatusCode,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			TotalTokens:      r.TotalTokens,
			Cost:             r.Cost,
			DurationMs:       r.DurationMs,
			IPAddress:        r.IPAddress,
			CreatedAt:        r.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  q.Page,
		"size":  q.PageSize,
	})
}

// ---- Audit Logs ----

type AuditLogListItem struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	Action       string    `json:"action"`
	ResourceType *string   `json:"resource_type,omitempty"`
	ResourceID   *string   `json:"resource_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "30"), 30)

	var total int64
	h.db.Model(&models.AuditLog{}).Count(&total)

	var logs []models.AuditLog
	h.db.Order("created_at DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&logs)

	items := make([]AuditLogListItem, 0, len(logs))
	for _, l := range logs {
		var uid *string
		if l.UserID != nil {
			s := l.UserID.String()
			uid = &s
		}
		items = append(items, AuditLogListItem{
			ID:           l.ID.String(),
			UserID:       uid,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceID:   l.ResourceID,
			CreatedAt:    l.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// ---- Settings ----

type SystemSettings struct {
	AllowRegistration          bool   `json:"allow_registration"`
	RequireEmailVerification   bool   `json:"require_email_verification"`
	DefaultUserQuota           int    `json:"default_user_quota"`
	MaintenanceMode            bool   `json:"maintenance_mode"`
	Announcement               string `json:"announcement"`
}

// settingsDefaults 默认值, 用于初始化或缺失时回退
var settingsDefaults = map[string]string{
	"signup_bonus":             "5",
	"allow_registration":       "true",
	"announcement":              "",
	"recharge_promo_enabled":   "true",
	"promo_start":               "",
	"promo_end":                 "",
	"recharge_tiers":            `[{"min":100,"bonus":8},{"min":300,"bonus":30},{"min":500,"bonus":75},{"min":1000,"bonus":200},{"min":3000,"bonus":750}]`,
	"first_recharge_bonus":      "50",
	"alert_email":               "",
	"alert_warn_threshold":      "100",
	"alert_critical_threshold":  "500",
	// 闲管家集成
	"goofish_app_key":                "",
	"goofish_app_secret":             "",
	"goofish_seller_id":              "",
	"goofish_webhook_url":            "https://transitai.cloud/v1/integrations/goofish/order",
	"goofish_stock_alert_threshold":  "5",
	"goofish_enabled":                "false",
	"goofish_mch_id":                 "",
	"goofish_mch_secret":             "",
	// 支付FM (zhifux) 额度监控
	"zhifux_quota_remaining":  "0",   // 当前剩余通用额度 (每收款 CNY 1 元扣 1 额度), 手动填入充值后的余额
	"zhifux_quota_threshold":  "500", // 剩余低于此值触发 Bark 告警
	"zhifux_bark_url":          "",   // Bark 推送 URL, 空则不告警
}

func (h *AdminHandler) loadAllSettings() map[string]string {
	var rows []models.Setting
	h.db.Find(&rows)
	out := make(map[string]string, len(settingsDefaults))
	for k, v := range settingsDefaults {
		out[k] = v
	}
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out
}

func (h *AdminHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.loadAllSettings())
}

func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for k, v := range req {
		if _, ok := settingsDefaults[k]; !ok {
			continue // ignore unknown keys
		}
		now := time.Now()
		row := models.Setting{Key: k, Value: v, UpdatedAt: now}
		if err := h.db.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	// 保存后如果 zhifux 额度设置变了, 立刻检查是否要 Bark 告警
	if _, quotaChanged := req["zhifux_quota_remaining"]; quotaChanged {
		go h.checkZhifuxQuotaAndAlert()
	} else if _, thChanged := req["zhifux_quota_threshold"]; thChanged {
		go h.checkZhifuxQuotaAndAlert()
	} else if _, urlChanged := req["zhifux_bark_url"]; urlChanged {
		go h.checkZhifuxQuotaAndAlert()
	}

	c.JSON(http.StatusOK, h.loadAllSettings())
}

// checkZhifuxQuotaAndAlert 保存 settings 时的即时检查: 剩余 <= 阈值就 Bark 一次
func (h *AdminHandler) checkZhifuxQuotaAndAlert() {
	remaining, _ := strconv.ParseFloat(GetSettingValue(h.db, "zhifux_quota_remaining", "0"), 64)
	threshold, _ := strconv.ParseFloat(GetSettingValue(h.db, "zhifux_quota_threshold", "0"), 64)
	if threshold <= 0 || remaining > threshold {
		return
	}
	barkURL := GetSettingValue(h.db, "zhifux_bark_url", "")
	if barkURL == "" {
		return
	}
	title := url.QueryEscape("支付FM 额度告警")
	body := url.QueryEscape(fmt.Sprintf("剩余额度 %.2f 已低于阈值 %.2f, 请尽快充值", remaining, threshold))
	fullURL := strings.TrimRight(barkURL, "/") + "/" + title + "/" + body + "?group=TransitAI&level=timeSensitive"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		log.Printf("[admin-quota] bark failed: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[admin-quota] bark sent, remaining=%.2f threshold=%.2f status=%d", remaining, threshold, resp.StatusCode)
}

// GetSettingValue 全局读取(注册赠送等地方用), 不存在返回 default
func GetSettingValue(db *gorm.DB, key, defaultValue string) string {
	var row models.Setting
	if err := db.Where("key = ?", key).First(&row).Error; err == nil {
		return row.Value
	}
	return defaultValue
}

// ---- Recharge Orders ----
func (h *AdminHandler) ListRechargeOrders(c *gin.Context) {
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "20"), 20)
	status := c.Query("status")

	query := h.db.Model(&models.RechargeOrder{})
	if status != "" {
		query = query.Where("payment_status = ?", status)
	}

	var total int64
	query.Count(&total)

	var orders []models.RechargeOrder
	query.Order("created_at DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&orders)

	type orderItem struct {
		ID            string     `json:"id"`
		UserID        string     `json:"user_id"`
		OrderNo       string     `json:"order_no"`
		Amount        float64    `json:"amount"`
		PaymentMethod string     `json:"payment_method"`
		PaymentStatus string     `json:"payment_status"`
		PaidAt        *time.Time `json:"paid_at,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
	}

	items := make([]orderItem, 0, len(orders))
	for _, o := range orders {
		items = append(items, orderItem{
			ID: o.ID.String(), UserID: o.UserID.String(), OrderNo: o.OrderNo,
			Amount: o.Amount, PaymentMethod: o.PaymentMethod,
			PaymentStatus: o.PaymentStatus, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": pageSize})
}

// ---- Internal Helpers ----

func (h *AdminHandler) createAuditLog(adminID, action, resourceType, resourceID string, oldV, newV interface{}) {
	parsedID, err := uuid.Parse(adminID)
	if err != nil {
		return
	}
	al := models.AuditLog{
		UserID:       &parsedID,
		Action:       action,
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
	}
	h.db.Create(&al)
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v < 1 {
		return defaultVal
	}
	return v
}

// testProviderConnection performs a basic health check.
func testProviderConnection(ch models.UpstreamChannel) bool {
	client := &http.Client{Timeout: 15 * time.Second}

	if ch.Provider == "anthropic" {
		baseURL := "https://api.anthropic.com"
		if ch.BaseURL != nil && *ch.BaseURL != "" {
			baseURL = *ch.BaseURL
		}
		req, err := http.NewRequest("GET", baseURL+"/v1/models", nil)
		if err != nil {
			return false
		}
		req.Header.Set("x-api-key", ch.APIKeyEncrypted)
		req.Header.Set("anthropic-version", "2023-06-01")
		log.Printf("[TestChannel] testing url: %s", baseURL+"/v1/models")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[TestChannel] error: %v", err)
			return false
		}
		defer resp.Body.Close()
		log.Printf("[TestChannel] status: %d", resp.StatusCode)
		return resp.StatusCode == 200
	}

	baseURL := "https://api.openai.com"
	if ch.BaseURL != nil && *ch.BaseURL != "" {
		baseURL = *ch.BaseURL
	}
	url := baseURL + "/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+ch.APIKeyEncrypted)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[TestChannel] anthropic request error: %v", err)
		return false
	}
	defer resp.Body.Close()
	log.Printf("[TestChannel] anthropic status: %d url: %s", resp.StatusCode, baseURL)
	return resp.StatusCode == 200
}


// ---- Profit Dashboard ----
// ProfitStats 返回平台收入、成本、毛利统计。
// 假设倍率 1.5x（成本 = 用户扣费 / 1.5），如果以后调整倍率需要同步修改。
// query 参数: range = today | month | year（默认 month）
func (h *AdminHandler) ProfitStats(c *gin.Context) {
	rangeType := c.DefaultQuery("range", "month")
	now := time.Now()
	var startTime time.Time
	switch rangeType {
	case "today":
		startTime = now.Truncate(24 * time.Hour)
	case "year":
		startTime = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default: // month
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}

	// rangeDays: 当前 range 跨度（用于摊销 subscription 月费）
	rangeDays := now.Sub(startTime).Hours() / 24
	if rangeDays < 1 {
		rangeDays = 1
	}

	type Summary struct {
		Revenue      float64 `json:"revenue"`        // 平台收入(USD)
		Cost         float64 `json:"cost"`           // 平台成本(USD)
		Profit       float64 `json:"profit"`         // 毛利(USD)
		ProfitMargin float64 `json:"profit_margin"`  // 毛利率(%)
		RequestCount int64   `json:"request_count"`  // 请求数
		PromptTokens int64   `json:"prompt_tokens"`
		OutputTokens int64   `json:"output_tokens"`
	}

	var s Summary
	h.db.Model(&models.Request{}).
		Where("status_code = 200 AND created_at >= ?", startTime).
		Select(`
			COALESCE(SUM(cost), 0) AS revenue,
			COUNT(*) AS request_count,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS output_tokens
		`).
		Scan(&s)

	// === 成本计算: 分 billing_mode ===
	// 1. Subscription 渠道: 固定成本 = monthly_fee × rangeDays / 30 (无论是否有流量)
	var subChannels []models.UpstreamChannel
	h.db.Where("is_enabled = ? AND billing_mode = ?", true, "subscription").Find(&subChannels)
	var subCost float64
	for _, ch := range subChannels {
		subCost += ch.MonthlyFeeCNY * rangeDays / 30
	}

	// 2. Pay-as-you-go 渠道: 按 reconcile_multiplier 反算 (revenue_per_request / multiplier)
	var paygCost float64
	h.db.Table("requests r").
		Joins("LEFT JOIN upstream_channels c ON c.id = r.upstream_channel_id").
		Where("r.status_code = 200 AND r.created_at >= ? AND (c.billing_mode IS NULL OR c.billing_mode = ?)", startTime, "pay_as_you_go").
		Select("COALESCE(SUM(r.cost / NULLIF(c.reconcile_multiplier, 0)), 0)").
		Scan(&paygCost)

	s.Cost = subCost + paygCost
	s.Profit = s.Revenue - s.Cost
	if s.Revenue > 0 {
		s.ProfitMargin = (s.Profit / s.Revenue) * 100
	}

	// 分模型统计
	type ModelBreakdown struct {
		ModelName    string  `json:"model_name"`
		RequestCount int64   `json:"request_count"`
		Revenue      float64 `json:"revenue"`
		Cost         float64 `json:"cost"`
		Profit       float64 `json:"profit"`
		Share        float64 `json:"share"` // 占总收入百分比
	}
	var byModel []ModelBreakdown
	h.db.Table("requests r").
		Select(`m.name AS model_name, COUNT(*) AS request_count, COALESCE(SUM(r.cost),0) AS revenue`).
		Joins("LEFT JOIN models m ON m.id = r.model_id").
		Where("r.status_code = 200 AND r.created_at >= ?", startTime).
		Group("m.name").
		Order("revenue DESC").
		Scan(&byModel)
	// byModel 成本用全局有效倍率估算 (近似, 不含 subscription 精确摊销)
	effectiveMult := 1.0
	if s.Cost > 0 {
		effectiveMult = s.Revenue / s.Cost
	}
	for i := range byModel {
		byModel[i].Cost = byModel[i].Revenue / effectiveMult
		byModel[i].Profit = byModel[i].Revenue - byModel[i].Cost
		if s.Revenue > 0 {
			byModel[i].Share = byModel[i].Revenue / s.Revenue * 100
		}
	}

	// 分分组收益
	type GroupBreakdown struct {
		GroupName    string  `json:"group_name"`
		GroupSlug    string  `json:"group_slug"`
		Multiplier   float64 `json:"multiplier"`
		RequestCount int64   `json:"request_count"`
		Revenue      float64 `json:"revenue"`
		Cost         float64 `json:"cost"`
		Profit       float64 `json:"profit"`
		Share        float64 `json:"share"`
	}
	var byGroup []GroupBreakdown
	h.db.Table("requests r").
		Select(`COALESCE(g.name, '未分组') AS group_name, COALESCE(g.slug, '') AS group_slug, COALESCE(g.multiplier, 1) AS multiplier, COUNT(*) AS request_count, COALESCE(SUM(r.cost),0) AS revenue`).
		Joins("LEFT JOIN models m ON m.id = r.model_id").
		Joins("LEFT JOIN channel_groups g ON g.id = m.group_id").
		Where("r.status_code = 200 AND r.created_at >= ?", startTime).
		Group("g.name, g.slug, g.multiplier").
		Order("revenue DESC").
		Scan(&byGroup)
	for i := range byGroup {
		byGroup[i].Cost = byGroup[i].Revenue / effectiveMult
		byGroup[i].Profit = byGroup[i].Revenue - byGroup[i].Cost
		if s.Revenue > 0 {
			byGroup[i].Share = byGroup[i].Revenue / s.Revenue * 100
		}
	}

	// TOP 10 消费用户
	type TopUser struct {
		Email        string  `json:"email"`
		RequestCount int64   `json:"request_count"`
		Revenue      float64 `json:"revenue"`
	}
	var topUsers []TopUser
	h.db.Table("requests r").
		Select(`u.email, COUNT(*) AS request_count, COALESCE(SUM(r.cost),0) AS revenue`).
		Joins("LEFT JOIN users u ON u.id = r.user_id").
		Where("r.status_code = 200 AND r.created_at >= ?", startTime).
		Group("u.email").
		Order("revenue DESC").
		Limit(10).
		Scan(&topUsers)

	// 最近 30 天每日趋势（无论 range 如何，趋势图固定 30 天）
	type DailyPoint struct {
		Date    string  `json:"date"`
		Revenue float64 `json:"revenue"`
		Cost    float64 `json:"cost"`
		Profit  float64 `json:"profit"`
	}
	var trend []DailyPoint
	thirtyDaysAgo := now.AddDate(0, 0, -29).Truncate(24 * time.Hour)
	h.db.Raw(`
		SELECT TO_CHAR(date_trunc('day', created_at), 'YYYY-MM-DD') AS date,
			COALESCE(SUM(cost), 0) AS revenue
		FROM requests
		WHERE status_code = 200 AND created_at >= ?
		GROUP BY date
		ORDER BY date ASC
	`, thirtyDaysAgo).Scan(&trend)
	for i := range trend {
		trend[i].Cost = trend[i].Revenue / effectiveMult
		trend[i].Profit = trend[i].Revenue - trend[i].Cost
	}

	c.JSON(http.StatusOK, gin.H{
		"range":      rangeType,
		"start_time": startTime,
		"multiplier": effectiveMult,
		"summary":    s,
		"by_model":   byModel,
		"by_group":   byGroup,
		"top_users":  topUsers,
		"trend_30d":  trend,
	})
}


// ============================================================
// Channel Group CRUD
// ============================================================

type ChannelGroupItem struct {
	ID            uint    `json:"id"`
	Name          string  `json:"name"`
	NameEn        string  `json:"name_en"`
	Slug          string  `json:"slug"`
	Multiplier    float64 `json:"multiplier"`
	Description   string  `json:"description"`
	DescriptionEn string  `json:"description_en"`
	SortOrder     int     `json:"sort_order"`
	IsDefault     bool    `json:"is_default"`
	Channels      int     `json:"channels"`
	Models        int     `json:"models"`
}

type CreateChannelGroupRequest struct {
	Name          string  `json:"name" binding:"required"`
	Slug          string  `json:"slug" binding:"required"`
	Multiplier    float64 `json:"multiplier" binding:"required,min=0.01"`
	Description   string  `json:"description"`
	NameEn        string  `json:"name_en"`
	DescriptionEn string  `json:"description_en"`
	SortOrder     int     `json:"sort_order"`
	IsDefault     bool    `json:"is_default"`
}

type UpdateChannelGroupRequest struct {
	Name          *string  `json:"name,omitempty"`
	Slug          *string  `json:"slug,omitempty"`
	Multiplier    *float64 `json:"multiplier,omitempty"`
	Description   *string  `json:"description,omitempty"`
	NameEn        *string  `json:"name_en,omitempty"`
	DescriptionEn *string  `json:"description_en,omitempty"`
	SortOrder     *int     `json:"sort_order,omitempty"`
	IsDefault     *bool    `json:"is_default,omitempty"`
}

func (h *AdminHandler) ListChannelGroups(c *gin.Context) {
	var groups []models.ChannelGroup
	if err := h.db.Order("sort_order ASC").Find(&groups).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	items := make([]ChannelGroupItem, 0, len(groups))
	for _, g := range groups {
		var chCount, mdCount int64
		h.db.Model(&models.UpstreamChannel{}).Where("group_id = ? AND deleted_at IS NULL", g.ID).Count(&chCount)
		h.db.Model(&models.Model{}).Where("group_id = ? AND deleted_at IS NULL", g.ID).Count(&mdCount)
		desc := ""
		if g.Description != nil {
			desc = *g.Description
		}
		nameEn := ""
		if g.NameEn != nil {
			nameEn = *g.NameEn
		}
		descEn := ""
		if g.DescriptionEn != nil {
			descEn = *g.DescriptionEn
		}
		items = append(items, ChannelGroupItem{
			ID: g.ID, Name: g.Name, NameEn: nameEn, Slug: g.Slug, Multiplier: g.Multiplier,
			Description: desc, DescriptionEn: descEn, SortOrder: g.SortOrder, IsDefault: g.IsDefault,
			Channels: int(chCount), Models: int(mdCount),
		})
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *AdminHandler) CreateChannelGroup(c *gin.Context) {
	var req CreateChannelGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	g := models.ChannelGroup{
		Name: req.Name, Slug: req.Slug, Multiplier: req.Multiplier,
		SortOrder: req.SortOrder, IsDefault: req.IsDefault,
	}
	if req.Description != "" {
		g.Description = &req.Description
	}
	if req.NameEn != "" {
		g.NameEn = &req.NameEn
	}
	if req.DescriptionEn != "" {
		g.DescriptionEn = &req.DescriptionEn
	}
	if err := h.db.Create(&g).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": g.ID, "message": "created"})
}

func (h *AdminHandler) UpdateChannelGroup(c *gin.Context) {
	id := c.Param("id")
	var req UpdateChannelGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.Multiplier != nil {
		updates["multiplier"] = *req.Multiplier
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.NameEn != nil {
		updates["name_en"] = *req.NameEn
	}
	if req.DescriptionEn != nil {
		updates["description_en"] = *req.DescriptionEn
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"error": "no fields to update"})
		return
	}
	if err := h.db.Model(&models.ChannelGroup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "updated"})
}

func (h *AdminHandler) DeleteChannelGroup(c *gin.Context) {
	id := c.Param("id")
	// 检查是否有关联 channel/model
	var chCount, mdCount int64
	h.db.Model(&models.UpstreamChannel{}).Where("group_id = ? AND deleted_at IS NULL", id).Count(&chCount)
	h.db.Model(&models.Model{}).Where("group_id = ? AND deleted_at IS NULL", id).Count(&mdCount)
	if chCount > 0 || mdCount > 0 {
		c.JSON(400, gin.H{"error": fmt.Sprintf("group has %d channels and %d models, please reassign first", chCount, mdCount)})
		return
	}
	if err := h.db.Delete(&models.ChannelGroup{}, id).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}
