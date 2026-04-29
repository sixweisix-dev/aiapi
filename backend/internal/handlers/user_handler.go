package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"ai-api-gateway/internal/membership"
	"ai-api-gateway/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

// ---- Dashboard ----

// Dashboard returns stats for the current user.
func (h *UserHandler) Dashboard(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
        }
	userID := userIDRaw.(string)

	parsedID, _ := uuid.Parse(userID)

	// Current balance
	var user models.User
	h.db.Select("balance, total_spent, request_count, membership_tier, membership_expires_at, membership_started_at").First(&user, "id = ?", parsedID)

	// This month stats
	monthStart := time.Now().Truncate(24 * time.Hour).AddDate(0, 0, -time.Now().Day()+1)

	var monthRequests int64
	h.db.Model(&models.Request{}).Where("user_id = ? AND created_at >= ?", parsedID, monthStart).Count(&monthRequests)

	var monthTokens int64
	h.db.Model(&models.Request{}).Where("user_id = ? AND created_at >= ?", parsedID, monthStart).Select("COALESCE(SUM(total_tokens),0)").Scan(&monthTokens)

	var monthSpent float64
	h.db.Model(&models.BillingRecord{}).Where("user_id = ? AND type = ? AND created_at >= ?", parsedID, "chat_completion", monthStart).Select("COALESCE(SUM(ABS(amount)),0)").Scan(&monthSpent)

	// Recent requests (last 5)
	type recentReq struct {
		ID               string    `json:"id"`
		ModelName        string    `json:"model_name"`
		PromptTokens     int       `json:"prompt_tokens"`
		CompletionTokens int       `json:"completion_tokens"`
		TotalTokens      int       `json:"total_tokens"`
		Cost             float64   `json:"cost"`
		StatusCode       int       `json:"status_code"`
		CreatedAt        time.Time `json:"created_at"`
	}

	var recentReqs []recentReq
	h.db.Table("requests").
		Select("requests.id, models.name AS model_name, requests.prompt_tokens, requests.completion_tokens, requests.total_tokens, requests.cost, requests.status_code, requests.created_at").
		Joins("LEFT JOIN models ON models.id = requests.model_id").
		Where("requests.user_id = ?", parsedID).
		Order("requests.created_at DESC").
		Limit(5).
		Scan(&recentReqs)

	// Recent billing (last 5)
	type recentBill struct {
		ID          string    `json:"id"`
		Type        string    `json:"type"`
		Amount      float64   `json:"amount"`
		Description *string   `json:"description,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
	}

	var recentBills []recentBill
	h.db.Model(&models.BillingRecord{}).
		Select("id, type, amount, description, created_at").
		Where("user_id = ?", parsedID).
		Order("created_at DESC").
		Limit(5).
		Scan(&recentBills)

	effectiveTier := membership.EffectiveTier(membership.Tier(user.MembershipTier), user.MembershipExpiresAt)
	tierLimits := membership.TierLimits[effectiveTier]

	c.JSON(http.StatusOK, gin.H{
		"balance":         user.Balance,
		"total_spent":     user.TotalSpent,
		"request_count":   user.RequestCount,
		"month_requests":  monthRequests,
		"month_tokens":    monthTokens,
		"month_spent":     monthSpent,
		"recent_requests": recentReqs,
		"recent_billing":  recentBills,
		"membership": gin.H{
			"tier":         user.MembershipTier,
			"effective":    string(effectiveTier),
			"display_name": tierLimits.DisplayName,
			"expires_at":   user.MembershipExpiresAt,
			"started_at":   user.MembershipStartedAt,
			"limits":       tierLimits,
		},
	})
}

// ---- Billing Records ----

type billingQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Start    string `form:"start"`
	End      string `form:"end"`
}

func (h *UserHandler) ListBilling(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
        }
	userID := userIDRaw.(string)

	var q billingQuery
	c.ShouldBindQuery(&q)
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}

	parsedID, _ := uuid.Parse(userID)
	query := h.db.Model(&models.BillingRecord{}).Where("user_id = ?", parsedID)

	if q.Type != "" {
		query = query.Where("type = ?", q.Type)
	}
	if q.Start != "" {
		query = query.Where("created_at >= ?", q.Start)
	}
	if q.End != "" {
		query = query.Where("created_at <= ?", q.End)
	}

	var total int64
	query.Count(&total)

	var records []models.BillingRecord
	query.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&records)

	type item struct {
		ID            string    `json:"id"`
		Type          string    `json:"type"`
		Amount        float64   `json:"amount"`
		BalanceBefore float64   `json:"balance_before"`
		BalanceAfter  float64   `json:"balance_after"`
		Description   *string   `json:"description,omitempty"`
		CreatedAt     time.Time `json:"created_at"`
	}

	items := make([]item, 0, len(records))
	for _, r := range records {
		items = append(items, item{
			ID:            r.ID.String(),
			Type:          r.Type,
			Amount:        r.Amount,
			BalanceBefore: r.BalanceBefore,
			BalanceAfter:  r.BalanceAfter,
			Description:   r.Description,
			CreatedAt:     r.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  q.Page,
		"size":  q.PageSize,
	})
}

// ExportBilling returns request-level billing details as CSV.
// 支持时间范围 / API key 筛选，输出 Excel 兼容的 UTF-8 BOM CSV。
func (h *UserHandler) ExportBilling(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDRaw.(string)
	parsedID, _ := uuid.Parse(userID)

	startStr := c.Query("start_date")
	endStr := c.Query("end_date")
	apiKeyID := c.Query("api_key_id")

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now
	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end = t.Add(24*time.Hour - time.Second)
		}
	}

	type row struct {
		CreatedAt        time.Time
		APIKeyName       string
		ProjectName      *string
		ModelName        string
		Provider         string
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
		Cost             float64
		StatusCode       int
		DurationMs       int
		IPAddress        *string
		BalanceAfter     *float64
	}

	var rows []row
	query := h.db.Table("requests AS r").
		Select("r.created_at, COALESCE(k.name, '-') AS api_key_name, k.project_name, COALESCE(m.name, '-') AS model_name, COALESCE(m.provider, '-') AS provider, r.prompt_tokens, r.completion_tokens, r.total_tokens, r.cost, r.status_code, r.duration_ms, r.ip_address, br.balance_after").
		Joins("LEFT JOIN api_keys k ON k.id = r.api_key_id").
		Joins("LEFT JOIN models m ON m.id = r.model_id").
		Joins("LEFT JOIN billing_records br ON br.request_id = r.id").
		Where("r.user_id = ? AND r.created_at BETWEEN ? AND ?", parsedID, start, end).
		Order("r.created_at DESC").
		Limit(50000)

	if apiKeyID != "" {
		query = query.Where("r.api_key_id = ?", apiKeyID)
	}
	query.Scan(&rows)

	filename := fmt.Sprintf("billing_%s_to_%s.csv", start.Format("2006-01-02"), end.Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"时间", "项目", "API Key 名称", "模型", "提供商", "输入 Token", "输出 Token", "总 Token", "费用 (CNY)", "扣费后余额 (CNY)", "状态码", "耗时(ms)", "IP"})

	var totalCost float64
	var totalTokens int
	for _, r := range rows {
		proj := "-"
		if r.ProjectName != nil && *r.ProjectName != "" {
			proj = *r.ProjectName
		}
		ip := "-"
		if r.IPAddress != nil {
			ip = *r.IPAddress
		}
		balanceStr := "-"
		if r.BalanceAfter != nil {
			balanceStr = fmt.Sprintf("%.4f", *r.BalanceAfter)
		}
		writer.Write([]string{
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			proj, r.APIKeyName, r.ModelName, r.Provider,
			fmt.Sprintf("%d", r.PromptTokens),
			fmt.Sprintf("%d", r.CompletionTokens),
			fmt.Sprintf("%d", r.TotalTokens),
			fmt.Sprintf("%.4f", r.Cost),
			balanceStr,
			fmt.Sprintf("%d", r.StatusCode),
			fmt.Sprintf("%d", r.DurationMs),
			ip,
		})
		totalCost += r.Cost
		totalTokens += r.TotalTokens
	}

	writer.Write([]string{})
	writer.Write([]string{"汇总", fmt.Sprintf("共 %d 条记录", len(rows)), "", "", "", "", "", fmt.Sprintf("%d", totalTokens), fmt.Sprintf("%.4f", totalCost), "", "", "", ""})
	writer.Flush()
}


// ---- Public Models with Pricing ----

type publicModelItem struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	DisplayName   string  `json:"display_name"`
	Provider      string  `json:"provider"`
	ContextLength int     `json:"context_length"`
	InputPrice    float64 `json:"input_price"`
	OutputPrice   float64 `json:"output_price"`
	Multiplier    float64 `json:"multiplier"`
	Description   *string `json:"description,omitempty"`
}

func (h *UserHandler) ListPublicModels(c *gin.Context) {
	var modelsList []models.Model
	h.db.Where("is_enabled = ? AND is_public = ?", true, true).
		Order("CASE WHEN name LIKE '%sonnet%' THEN 1 WHEN name LIKE '%haiku%' THEN 2 WHEN name LIKE '%opus%' THEN 4 ELSE 3 END, provider ASC, name ASC").
		Find(&modelsList)

	items := make([]publicModelItem, 0, len(modelsList))
	for _, m := range modelsList {
		items = append(items, publicModelItem{
			ID:            m.ID.String(),
			Name:          m.Name,
			DisplayName:   m.DisplayName,
			Provider:      m.Provider,
			ContextLength: m.ContextLength,
			InputPrice:    m.InputPrice,
			OutputPrice:   m.OutputPrice,
			Multiplier:    m.Multiplier,
			Description:   m.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ---- Usage Stats (for API key usage) ----

type usageStatsResponse struct {
	TotalRequests int64   `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	TodayRequests int64   `json:"today_requests"`
	TodayTokens   int64   `json:"today_tokens"`
}

func (h *UserHandler) UsageStats(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
        }
	userID := userIDRaw.(string)

	parsedID, _ := uuid.Parse(userID)
	today := time.Now().Truncate(24 * time.Hour)

	var resp usageStatsResponse
	h.db.Model(&models.Request{}).Where("user_id = ?", parsedID).Select("COALESCE(COUNT(*),0)").Scan(&resp.TotalRequests)
	h.db.Model(&models.Request{}).Where("user_id = ?", parsedID).Select("COALESCE(SUM(total_tokens),0)").Scan(&resp.TotalTokens)
	h.db.Model(&models.BillingRecord{}).Where("user_id = ? AND type = ?", parsedID, "chat_completion").Select("COALESCE(SUM(ABS(amount)),0)").Scan(&resp.TotalCost)
	h.db.Model(&models.Request{}).Where("user_id = ? AND created_at >= ?", parsedID, today).Select("COALESCE(COUNT(*),0)").Scan(&resp.TodayRequests)
	h.db.Model(&models.Request{}).Where("user_id = ? AND created_at >= ?", parsedID, today).Select("COALESCE(SUM(total_tokens),0)").Scan(&resp.TodayTokens)

	c.JSON(http.StatusOK, resp)
}
