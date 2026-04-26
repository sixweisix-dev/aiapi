package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"ai-api-gateway/internal/models"
)

type PaymentHandler struct {
	db        *gorm.DB
	alipayCli *alipay.Client
	alipayCfg AlipayConfig
}

type AlipayConfig struct {
	NotifyURL     string
	ReturnURL     string
	Sandbox       bool
}

// ---- Request / Response types ----

type CreateOrderRequest struct {
	Amount        float64 `json:"amount" binding:"required,min=0.01"`
	PaymentMethod string  `json:"payment_method" binding:"required,oneof=alipay"`
}

type OrderResponse struct {
	ID            string  `json:"id"`
	OrderNo       string  `json:"order_no"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	PaymentStatus string  `json:"payment_status"`
	PayURL        string  `json:"pay_url,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// ---- Constructor ----

func NewPaymentHandler(db *gorm.DB, cfg AlipayConfig, appID, privateKey, publicKey string) (*PaymentHandler, error) {
	if appID == "" || privateKey == "" {
		log.Println("WARN: Alipay not configured, payment endpoints will return 503")
		return &PaymentHandler{db: db, alipayCfg: cfg}, nil
	}

	client, err := alipay.New(appID, privateKey, !cfg.Sandbox)
	if err != nil {
		return nil, fmt.Errorf("failed to create alipay client: %w", err)
	}

	if err := client.LoadAliPayPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("failed to load alipay public key: %w", err)
	}

	log.Printf("Alipay client initialized (sandbox=%v, app_id=%s)", cfg.Sandbox, appID)
	return &PaymentHandler{db: db, alipayCli: client, alipayCfg: cfg}, nil
}

// ---- Handlers ----

// CreateOrder creates a recharge order and returns an Alipay payment URL.
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	if h.alipayCli == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment not configured"})
		return
	}

	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
        }
	userID := userIDRaw.(string)

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Round amount to 2 decimal places (Alipay only supports up to 2)
	amount := math.Round(req.Amount*100) / 100

	// Generate order number: timestamp + 8 random hex digits
	orderNo := fmt.Sprintf("RECHARGE%d%s", time.Now().UnixMilli(), uuid.New().String()[:8])

	// Create order in pending status
	parsedUserID, _ := uuid.Parse(userID)
	order := &models.RechargeOrder{
		UserID:        parsedUserID,
		OrderNo:       orderNo,
		Amount:        amount,
		PaymentMethod: "alipay",
		PaymentStatus: "pending",
	}
	if err := h.db.Create(order).Error; err != nil {
		log.Printf("Create order error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	// Build Alipay trade page pay request
	p := alipay.TradePagePay{}
	p.NotifyURL = h.alipayCfg.NotifyURL
	p.ReturnURL = h.alipayCfg.ReturnURL
	p.Subject = fmt.Sprintf("API 充值 - ¥%.2f", amount)
	p.OutTradeNo = orderNo
	p.TotalAmount = fmt.Sprintf("%.2f", amount)
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"

	payURL, err := h.alipayCli.TradePagePay(p)
	if err != nil {
		log.Printf("Alipay TradePagePay error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}

	c.JSON(http.StatusOK, OrderResponse{
		ID:            order.ID.String(),
		OrderNo:       orderNo,
		Amount:        amount,
		PaymentMethod: "alipay",
		PaymentStatus: "pending",
		PayURL:        payURL.String(),
		CreatedAt:     order.CreatedAt.Format(time.RFC3339),
	})
}

// AlipayNotify handles the async notification from Alipay.
// IMPORTANT: This endpoint should be exposed WITHOUT auth middleware (Alipay sends the request directly).
func (h *PaymentHandler) AlipayNotify(c *gin.Context) {
	if h.alipayCli == nil {
		c.String(http.StatusOK, "fail")
		return
	}

	noti, err := h.alipayCli.GetTradeNotification(c.Request)
	if err != nil {
		log.Printf("Alipay notification verification failed: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	// Only process successful payments
	if noti.TradeStatus != "TRADE_SUCCESS" && noti.TradeStatus != "TRADE_FINISHED" {
		log.Printf("Alipay notify skipped: non-success status %s for order %s", noti.TradeStatus, noti.OutTradeNo)
		c.String(http.StatusOK, "success")
		return
	}

	// Process the payment in a transaction
	if err := h.processSuccessfulPayment(noti); err != nil {
		log.Printf("Payment processing failed for order %s: %v", noti.OutTradeNo, err)
		c.String(http.StatusOK, "fail")
		return
	}

	log.Printf("Payment processed: order=%s, trade_no=%s, amount=%s", noti.OutTradeNo, noti.TradeNo, noti.TotalAmount)
	c.String(http.StatusOK, "success")
}

// AlipayReturn handles the synchronous redirect from Alipay (user lands here after payment).
func (h *PaymentHandler) AlipayReturn(c *gin.Context) {
	if h.alipayCli == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment not configured"})
		return
	}

	noti, err := h.alipayCli.GetTradeNotification(c.Request)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "fail", "message": "verification failed"})
		return
	}

	// Async notify will handle the actual balance update; this is just a redirect page.
	c.JSON(http.StatusOK, gin.H{
		"code":       "success",
		"message":   "payment received, your balance will be updated shortly",
		"order_no":  noti.OutTradeNo,
		"trade_no":  noti.TradeNo,
		"amount":    noti.TotalAmount,
	})
}

// ListOrders returns the current user's recharge orders.
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(401, gin.H{"error": "unauthorized"})
                return
	}
	userID := userIDRaw.(string)

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user"})
		return
	}

	var orders []models.RechargeOrder
	if err := h.db.Where("user_id = ?", parsedUserID).Order("created_at DESC").Limit(50).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch orders"})
		return
	}

	result := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		result = append(result, OrderResponse{
			ID:            o.ID.String(),
			OrderNo:       o.OrderNo,
			Amount:        o.Amount,
			PaymentMethod: o.PaymentMethod,
			PaymentStatus: o.PaymentStatus,
			CreatedAt:     o.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{"orders": result})
}

// ---- Internal ----

// processSuccessfulPayment updates the order status and adds balance to user.
func (h *PaymentHandler) processSuccessfulPayment(noti *alipay.Notification) error {
	// Use transaction for atomicity
	return h.db.Transaction(func(tx *gorm.DB) error {
		// Find the order
		var order models.RechargeOrder
		if err := tx.Where("order_no = ?", noti.OutTradeNo).First(&order).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}

		// Idempotency check: skip if already processed
		if order.PaymentStatus == "paid" {
			log.Printf("Order %s already processed, skipping", noti.OutTradeNo)
			return nil
		}

		// Status machine: only process pending orders
		if order.PaymentStatus != "pending" {
			return fmt.Errorf("order %s has invalid status for processing: %s", noti.OutTradeNo, order.PaymentStatus)
		}

		// Amount verification: Alipay amount should match order amount
		var paidAmount float64
		if _, err := fmt.Sscanf(noti.TotalAmount, "%f", &paidAmount); err != nil {
			return fmt.Errorf("failed to parse paid amount: %w", err)
		}
		// Allow 0.01 tolerance for floating point
		if math.Abs(paidAmount-order.Amount) > 0.01 {
			return fmt.Errorf("amount mismatch: order=%.2f, paid=%.2f", order.Amount, paidAmount)
		}

		// Update order status
		now := time.Now()
		paymentID := noti.TradeNo
		updates := map[string]interface{}{
			"payment_status": "paid",
			"payment_id":     &paymentID,
			"paid_at":        &now,
		}
		if err := tx.Model(&order).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}

		// Atomic balance increment (prevents race conditions from concurrent recharges)
		var balanceBefore float64
		if err := tx.Model(&models.User{}).Select("balance").
			Where("id = ?", order.UserID).Scan(&balanceBefore).Error; err != nil {
			return fmt.Errorf("failed to read balance: %w", err)
		}

		newBalance := balanceBefore + order.Amount
		if err := tx.Model(&models.User{}).Where("id = ?", order.UserID).
			Update("balance", gorm.Expr("balance + ?", order.Amount)).Error; err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		// Record billing record for the recharge
		desc := fmt.Sprintf("支付宝充值 ¥%.2f", order.Amount)
		billingRecord := &models.BillingRecord{
			UserID:        order.UserID,
			Type:          "recharge",
			Amount:        order.Amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  newBalance,
			Description:   &desc,
		}
		if err := tx.Create(billingRecord).Error; err != nil {
			return fmt.Errorf("failed to create billing record: %w", err)
		}

		return nil
	})
}
