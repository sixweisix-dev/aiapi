package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"ai-api-gateway/internal/models"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
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
}

type UpdateChannelRequest struct {
	Name     *string `json:"name,omitempty"`
	APIKey   *string `json:"api_key,omitempty"`
	BaseURL  *string `json:"base_url,omitempty"`
	Weight   *int    `json:"weight,omitempty"`
	IsEnabled *bool  `json:"is_enabled,omitempty"`
}

type ChannelListItem struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Provider     string     `json:"provider"`
	Weight       int        `json:"weight"`
	IsEnabled    bool       `json:"is_enabled"`
	HealthStatus string     `json:"health_status"`
	LastCheck    *time.Time `json:"last_health_check"`
	TotalReqs    int        `json:"total_requests"`
	TotalTokens  int        `json:"total_tokens"`
	ErrorCount   int        `json:"error_count"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (h *AdminHandler) ListChannels(c *gin.Context) {
	var channels []models.UpstreamChannel
	h.db.Order("created_at DESC").Find(&channels)
	items := make([]ChannelListItem, 0, len(channels))
	for _, ch := range channels {
		items = append(items, ChannelListItem{
			ID:           ch.ID.String(),
			Name:         ch.Name,
			Provider:     ch.Provider,
			Weight:       ch.Weight,
			IsEnabled:    ch.IsEnabled,
			HealthStatus: ch.HealthStatus,
			LastCheck:    ch.LastHealthCheck,
			TotalReqs:    ch.TotalRequests,
			TotalTokens:  ch.TotalTokens,
			ErrorCount:   ch.ErrorCount,
			CreatedAt:    ch.CreatedAt,
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
	if req.Weight != nil {
		channel.Weight = *req.Weight
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

	if len(updates) > 0 {
		h.db.Model(&channel).Updates(updates)
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
	InputPrice    float64  `json:"input_price" binding:"required,min=0"`
	OutputPrice   float64  `json:"output_price" binding:"required,min=0"`
	Multiplier    *float64 `json:"multiplier,omitempty"`
	IsPublic      *bool    `json:"is_public,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

type UpdateModelRequest struct {
	DisplayName   *string  `json:"display_name,omitempty"`
	InputPrice    *float64 `json:"input_price,omitempty"`
	OutputPrice   *float64 `json:"output_price,omitempty"`
	Multiplier    *float64 `json:"multiplier,omitempty"`
	IsEnabled     *bool    `json:"is_enabled,omitempty"`
	IsPublic      *bool    `json:"is_public,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

type ModelListItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Provider      string   `json:"provider"`
	ContextLength int      `json:"context_length"`
	InputPrice    float64  `json:"input_price"`
	OutputPrice   float64  `json:"output_price"`
	Multiplier    float64  `json:"multiplier"`
	IsEnabled     bool     `json:"is_enabled"`
	IsPublic      bool     `json:"is_public"`
	Description   *string  `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *AdminHandler) ListModels(c *gin.Context) {
	var modelsList []models.Model
	h.db.Order("provider ASC, name ASC").Find(&modelsList)
	items := make([]ModelListItem, 0, len(modelsList))
	for _, m := range modelsList {
		items = append(items, ModelListItem{
			ID:            m.ID.String(),
			Name:          m.Name,
			DisplayName:   m.DisplayName,
			Provider:      m.Provider,
			ContextLength: m.ContextLength,
			InputPrice:    m.InputPrice,
			OutputPrice:   m.OutputPrice,
			Multiplier:    m.Multiplier,
			IsEnabled:     m.IsEnabled,
			IsPublic:      m.IsPublic,
			Description:   m.Description,
			CreatedAt:     m.CreatedAt,
		})
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

	if err := h.db.Create(&model).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model"})
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
	c.JSON(http.StatusOK, h.loadAllSettings())
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

	const multiplier = 1.5

	type Summary struct {
		Revenue      float64 `json:"revenue"`        // 平台收入(CNY)
		Cost         float64 `json:"cost"`           // 平台成本(CNY)
		Profit       float64 `json:"profit"`         // 毛利(CNY)
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
	s.Cost = s.Revenue / multiplier
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
	for i := range byModel {
		byModel[i].Cost = byModel[i].Revenue / multiplier
		byModel[i].Profit = byModel[i].Revenue - byModel[i].Cost
		if s.Revenue > 0 {
			byModel[i].Share = byModel[i].Revenue / s.Revenue * 100
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
		trend[i].Cost = trend[i].Revenue / multiplier
		trend[i].Profit = trend[i].Revenue - trend[i].Cost
	}

	c.JSON(http.StatusOK, gin.H{
		"range":      rangeType,
		"start_time": startTime,
		"multiplier": multiplier,
		"summary":    s,
		"by_model":   byModel,
		"top_users":  topUsers,
		"trend_30d":  trend,
	})
}
