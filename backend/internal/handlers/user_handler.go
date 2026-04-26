package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

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
	h.db.Select("balance, total_spent, request_count").First(&user, "id = ?", parsedID)

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

	c.JSON(http.StatusOK, gin.H{
		"balance":         user.Balance,
		"total_spent":     user.TotalSpent,
		"request_count":   user.RequestCount,
		"month_requests":  monthRequests,
		"month_tokens":    monthTokens,
		"month_spent":     monthSpent,
		"recent_requests": recentReqs,
		"recent_billing":  recentBills,
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

// ExportBilling returns billing records as CSV.
func (h *UserHandler) ExportBilling(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
        }
	userID := userIDRaw.(string)

	parsedID, _ := uuid.Parse(userID)

	var records []models.BillingRecord
	h.db.Where("user_id = ?", parsedID).Order("created_at DESC").Limit(1000).Find(&records)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=billing_%s.csv", time.Now().Format("2006-01-02")))

	// Write BOM for Excel compatibility
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"ID", "类型", "金额", "余额前", "余额后", "描述", "时间"})

	for _, r := range records {
		desc := ""
		if r.Description != nil {
			desc = *r.Description
		}
		writer.Write([]string{
			r.ID.String(),
			r.Type,
			fmt.Sprintf("%.8f", r.Amount),
			fmt.Sprintf("%.8f", r.BalanceBefore),
			fmt.Sprintf("%.8f", r.BalanceAfter),
			desc,
			r.CreatedAt.Format(time.RFC3339),
		})
	}
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
		Order("provider ASC, name ASC").
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
