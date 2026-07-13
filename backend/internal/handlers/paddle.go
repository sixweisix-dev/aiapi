package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/models"
)

type PaddleHandler struct {
	db            *gorm.DB
	engine        *billing.Engine
	webhookSecret string
	clientToken   string
	apiKey        string
}

func NewPaddleHandler(db *gorm.DB, engine *billing.Engine) *PaddleHandler {
	return &PaddleHandler{
		db:            db,
		engine:        engine,
		webhookSecret: os.Getenv("PADDLE_WEBHOOK_SECRET"),
		clientToken:   os.Getenv("PADDLE_CLIENT_TOKEN"),
		apiKey:        os.Getenv("PADDLE_API_KEY"),
	}
}

var paddlePriceMap = map[string]string{
	"pri_01kxa9gsx0gqdkk0mf923g8mf1": "100",
	"pri_01kxa9gtr66vf94b4z3tdke9pe": "300",
	"pri_01kxa9gvqfp21x2bc088kfwky2": "500",
	"pri_01kxa9gweqyeqfaw7jxhkwp9a9": "1000",
	"pri_01kxa9gy3a9t12g3zgtexzf81r": "3000",
	"pri_01kxa9gyvm4pmnztyse607p714": "pro",
	"pri_01kxa9gzmzmyyvesy5kj97bx9t": "enterprise",
}

const topUpPriceID = "pri_01kxamkng7rn6ae47gp5m12ctn"

func (h *PaddleHandler) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"client_token":    h.clientToken,
		"environment":     os.Getenv("PADDLE_ENV"),
		"top_up_price_id": topUpPriceID,
		"tier_map": map[string]map[string]interface{}{
			"100":        {"price_id": "pri_01kxa9gsx0gqdkk0mf923g8mf1", "amount_cny": 100, "balance_usd": 108, "name": "Starter"},
			"300":        {"price_id": "pri_01kxa9gtr66vf94b4z3tdke9pe", "amount_cny": 300, "balance_usd": 330, "name": "Growth"},
			"500":        {"price_id": "pri_01kxa9gvqfp21x2bc088kfwky2", "amount_cny": 500, "balance_usd": 575, "name": "Pro Pack"},
			"1000":       {"price_id": "pri_01kxa9gweqyeqfaw7jxhkwp9a9", "amount_cny": 1000, "balance_usd": 1200, "name": "Scale"},
			"3000":       {"price_id": "pri_01kxa9gy3a9t12g3zgtexzf81r", "amount_cny": 3000, "balance_usd": 3750, "name": "Enterprise"},
			"pro":        {"price_id": "pri_01kxa9gyvm4pmnztyse607p714", "amount_cny": 99, "balance_usd": 120, "name": "Pro Membership"},
			"enterprise": {"price_id": "pri_01kxa9gzmzmyyvesy5kj97bx9t", "amount_cny": 499, "balance_usd": 600, "name": "Enterprise Membership"},
		},
	})
}

type paddleCreateOrderReq struct {
	TierID    string  `json:"tier_id"`
	PriceID   string  `json:"price_id"`
	AmountCNY float64 `json:"amount_cny"`
}

func (h *PaddleHandler) CreateOrder(c *gin.Context) {
	userIDIface, _ := c.Get("user_id")
	userIDStr, _ := userIDIface.(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req paddleCreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TierID != "custom" {
		if pid, ok := paddlePriceMap[req.PriceID]; !ok || pid != req.TierID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tier_id/price_id"})
			return
		}
		if t, ok := stripeTiers[req.TierID]; ok {
			req.AmountCNY = float64(t.AmountCNY)
		}
	} else {
		if req.AmountCNY <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "amount required for custom"})
			return
		}
		req.PriceID = topUpPriceID
	}

	orderNo := fmt.Sprintf("PD%d", time.Now().UnixNano())
	order := &models.PaddleOrder{
		ID:        uuid.New(),
		UserID:    userID,
		OrderNo:   orderNo,
		PriceID:   req.PriceID,
		TierID:    req.TierID,
		AmountCNY: req.AmountCNY,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[paddle] create order: user=%s tier=%s price=%s amount_cny=%.2f order_no=%s", userIDStr, req.TierID, req.PriceID, req.AmountCNY, orderNo)

	c.JSON(http.StatusOK, gin.H{
		"order_no":   orderNo,
		"price_id":   req.PriceID,
		"amount_cny": req.AmountCNY,
	})
}

func (h *PaddleHandler) Webhook(c *gin.Context) {
	sig := c.GetHeader("Paddle-Signature")
	if sig == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing signature"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	if !h.verifySignature(body, sig) {
		log.Printf("[paddle-webhook] signature verification failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var event struct {
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	log.Printf("[paddle-webhook] event=%s", event.EventType)

	switch event.EventType {
	case "transaction.completed", "transaction.paid":
		if err := h.handleTransactionPaid(event.Data); err != nil {
			log.Printf("[paddle-webhook] handleTransactionPaid err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case "transaction.canceled":
		var tx struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data, &tx); err == nil {
			h.db.Model(&models.PaddleOrder{}).
				Where("paddle_transaction_id = ?", tx.ID).
				Update("status", "cancelled")
		}
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *PaddleHandler) verifySignature(body []byte, sigHeader string) bool {
	if h.webhookSecret == "" {
		log.Println("[paddle] WARN: PADDLE_WEBHOOK_SECRET not set, skipping verification")
		return true
	}
	parts := strings.Split(sigHeader, ";")
	var ts, h1 string
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "h1":
			h1 = kv[1]
		}
	}
	if ts == "" || h1 == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write([]byte(ts + ":" + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(h1))
}

func (h *PaddleHandler) handleTransactionPaid(dataRaw json.RawMessage) error {
	var tx struct {
		ID         string          `json:"id"`
		Status     string          `json:"status"`
		CustomerID string          `json:"customer_id"`
		CustomData json.RawMessage `json:"custom_data"`
		Items      []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
			Quantity int `json:"quantity"`
		} `json:"items"`
		Details struct {
			Totals struct {
				Total string `json:"total"`
			} `json:"totals"`
		} `json:"details"`
		Currency string `json:"currency_code"`
	}
	if err := json.Unmarshal(dataRaw, &tx); err != nil {
		return fmt.Errorf("unmarshal tx: %v", err)
	}

	var custom struct {
		OrderNo string `json:"order_no"`
	}
	_ = json.Unmarshal(tx.CustomData, &custom)

	var order models.PaddleOrder
	if custom.OrderNo != "" {
		if err := h.db.Where("order_no = ?", custom.OrderNo).First(&order).Error; err != nil {
			log.Printf("[paddle-webhook] order not found by order_no=%s", custom.OrderNo)
			return nil
		}
	} else {
		if err := h.db.Where("paddle_transaction_id = ?", tx.ID).First(&order).Error; err != nil {
			log.Printf("[paddle-webhook] no matching order for tx=%s", tx.ID)
			return nil
		}
	}

	if order.Status == "paid" {
		log.Printf("[paddle-webhook] order %s already paid, skipping", order.OrderNo)
		return nil
	}

	now := time.Now()
	totalCents, _ := strconv.ParseInt(tx.Details.Totals.Total, 10, 64)
	amountUSDPaid := float64(totalCents) / 100.0

	payloadStr := string(dataRaw)
	h.db.Model(&order).Updates(map[string]interface{}{
		"paddle_transaction_id": tx.ID,
		"paddle_customer_id":    tx.CustomerID,
		"status":                "paid",
		"paid_at":               &now,
		"amount_usd_paid":       amountUSDPaid,
		"raw_payload":           payloadStr,
	})

	h.db.Where("id = ?", order.ID).First(&order)
	if err := h.processRecharge(&order); err != nil {
		return fmt.Errorf("processRecharge: %v", err)
	}

	log.Printf("[paddle-webhook] OK: user=%s order=%s tier=%s amount_cny=%.2f amount_usd_paid=%.2f", order.UserID, order.OrderNo, order.TierID, order.AmountCNY, amountUSDPaid)
	return nil
}

func (h *PaddleHandler) processRecharge(order *models.PaddleOrder) error {
	userID := order.UserID.String()
	amountCNY := int(order.AmountCNY + 0.5)
	tierID := order.TierID

	var tier stripeTier
	if tierID == "" || tierID == "custom" {
		tier = computeCustomTier(amountCNY)
		tierID = "custom"
	} else {
		if t, ok := stripeTiers[tierID]; ok {
			tier = t
		} else {
			tier = computeCustomTier(amountCNY)
			tierID = "custom"
		}
	}

	isUpgrade := tier.UpgradesToTier != ""
	paidUSD := float64(amountCNY)
	balanceGain := tier.BalanceUSD
	bonus := tier.BonusUSD

	minStr := h.readSetting("first_recharge_min_amount")
	bonusStr := h.readSetting("first_recharge_bonus")
	firstRechargeMin, _ := strconv.ParseFloat(minStr, 64)
	firstRechargeBonus, _ := strconv.ParseFloat(bonusStr, 64)

	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user models.User
	if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("find user: %v", err)
	}

	var firstBonusUSD float64
	if !isUpgrade && firstRechargeMin > 0 && firstRechargeBonus > 0 && user.FirstRechargeAt == nil && paidUSD >= firstRechargeMin {
		firstBonusUSD = firstRechargeBonus
	}

	totalGain := balanceGain + bonus + firstBonusUSD
	if isUpgrade {
		totalGain = tier.BalanceUSD
	}

	balanceBefore := user.Balance
	balanceAfter := balanceBefore + totalGain

	updates := map[string]interface{}{
		"balance":    balanceAfter,
		"updated_at": time.Now(),
	}
	if user.FirstRechargeAt == nil {
		now := time.Now()
		updates["first_recharge_at"] = &now
	}
	if isUpgrade {
		mLevel := tier.UpgradesToTier
		expires := time.Now().AddDate(0, 0, tier.DurationDays)
		updates["membership_tier"] = mLevel
		updates["membership_expires_at"] = &expires
	}
	if err := tx.Model(&user).Updates(updates).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("update user: %v", err)
	}

	note := fmt.Sprintf("Paddle recharge $%.2f tier=%s bonus=$%.2f first=$%.2f", paidUSD, tierID, bonus, firstBonusUSD)
	rec := &models.BillingRecord{
		UserID:        user.ID,
		Type:          "recharge",
		Amount:        totalGain,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Description:   &note,
	}
	if err := tx.Create(rec).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("create billing: %v", err)
	}

	// 写 recharge_orders (Paddle 走 stripe payment_method 复用现有前端 UI)
	var upgradesToTier *string
	if isUpgrade {
		v := tier.UpgradesToTier
		upgradesToTier = &v
	}
	intent := "balance"
	if isUpgrade {
		intent = "membership"
	}
	if err := tx.Exec(
		"INSERT INTO recharge_orders (user_id, order_no, amount, bonus_amount, payment_method, payment_status, payment_id, paid_at, created_at, updated_at, intent, upgrades_to_tier) VALUES (?::uuid, ?, ?, ?, 'stripe', 'paid', ?, NOW(), NOW(), NOW(), ?, ?) ON CONFLICT (order_no) DO NOTHING",
		userID, order.OrderNo, totalGain, bonus+firstBonusUSD, order.PaddleTransactionID, intent, upgradesToTier,
	).Error; err != nil {
		log.Printf("[paddle-webhook] insert recharge_orders failed (non-fatal): %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit: %v", err)
	}

	if h.engine != nil {
		_ = h.engine.InitBalance(userID)
	}

	log.Printf("[paddle-webhook] processed: user=%s paid=$%.2f received=$%.2f tier=%s upgrade=%v firstBonus=$%.2f",
		userID, paidUSD, totalGain, tierID, isUpgrade, firstBonusUSD)
	return nil
}

func (h *PaddleHandler) readSetting(key string) string {
	var v string
	row := h.db.Raw("SELECT value FROM settings WHERE key = ?", key).Row()
	_ = row.Scan(&v)
	return v
}
