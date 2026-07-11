package handlers

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

type ZhifuxHandler struct {
	db          *gorm.DB
	engine      *billing.Engine
	merchantNum string
	secret      string
	apiBase     string
}

func NewZhifuxHandler(db *gorm.DB, engine *billing.Engine) *ZhifuxHandler {
	return &ZhifuxHandler{
		db:          db,
		engine:      engine,
		merchantNum: os.Getenv("ZHIFUX_MERCHANT_NUM"),
		secret:      os.Getenv("ZHIFUX_SECRET"),
		apiBase:     os.Getenv("ZHIFUX_API_BASE"),
	}
}

func (h *ZhifuxHandler) sign(orderNo, amount, notifyURL string) string {
	raw := h.merchantNum + orderNo + amount + notifyURL + h.secret
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (h *ZhifuxHandler) signNotify(state, orderNo, amount string) string {
	raw := state + h.merchantNum + orderNo + amount + h.secret
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// readSetting reads a value from public.settings table by key
func (h *ZhifuxHandler) readSetting(key string) string {
	var v string
	row := h.db.Raw("SELECT value FROM settings WHERE key = ?", key).Row()
	_ = row.Scan(&v)
	return v
}

type zhifuxCreateReq struct {
	Amount  float64 `json:"amount" binding:"required"`
	PayType string  `json:"pay_type"`
	TierID  string  `json:"tier_id"`
}

type zhifuxAPIResp struct {
	Success   bool   `json:"success"`
	Msg       string `json:"msg"`
	Code      int    `json:"code"`
	Timestamp int64  `json:"timestamp"`
	Data      struct {
		ID     string `json:"id"`
		PayURL string `json:"payUrl"`
	} `json:"data"`
}

func (h *ZhifuxHandler) CreateCheckout(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(string)

	var req zhifuxCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.PayType == "" {
		req.PayType = "aloop"
	}
	if req.Amount < 1 {
		c.JSON(400, gin.H{"error": "amount too small"})
		return
	}

	orderNo := fmt.Sprintf("TX%d", time.Now().UnixNano())
	amountStr := fmt.Sprintf("%.2f", req.Amount)
	notifyURL := "https://transitai.cloud/v1/zhifux/webhook"
	returnURL := "https://transitai.cloud/recharge?payment=success"

	sig := h.sign(orderNo, amountStr, notifyURL)

	parsedUserID, _ := uuid.Parse(userID)
	order := &models.ZhifuxOrder{
		ID:        uuid.New(),
		UserID:    parsedUserID,
		OrderNo:   orderNo,
		Amount:    req.Amount,
		PayType:   req.PayType,
		TierID:    req.TierID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(order).Error; err != nil {
		log.Printf("[zhifux] db insert failed: %v", err)
		c.JSON(500, gin.H{"error": "db failed"})
		return
	}

	form := url.Values{}
	form.Set("merchantNum", h.merchantNum)
	form.Set("orderNo", orderNo)
	form.Set("amount", amountStr)
	form.Set("notifyUrl", notifyURL)
	form.Set("returnUrl", returnURL)
	form.Set("payType", req.PayType)
	form.Set("returnType", "json")
	form.Set("sign", sig)
	form.Set("subject", fmt.Sprintf("TransitAI recharge %s CNY", amountStr))

	apiURL := h.apiBase + "/startOrder?" + form.Encode()
	httpReq, _ := http.NewRequest("POST", apiURL, nil)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[zhifux] http err: %v", err)
		c.JSON(500, gin.H{"error": "provider unreachable"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var apiResp zhifuxAPIResp
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("[zhifux] parse resp err: %v body=%s", err, string(body))
		c.JSON(500, gin.H{"error": "invalid provider response"})
		return
	}

	if !apiResp.Success {
		log.Printf("[zhifux] api failure: %s", apiResp.Msg)
		c.JSON(500, gin.H{"error": apiResp.Msg})
		return
	}

	h.db.Model(&models.ZhifuxOrder{}).Where("order_no = ?", orderNo).Update("platform_order_id", apiResp.Data.ID)

	c.JSON(200, gin.H{
		"pay_url":  apiResp.Data.PayURL,
		"order_no": orderNo,
	})
}

func (h *ZhifuxHandler) Webhook(c *gin.Context) {
	params := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	if len(params) == 0 && c.Request.Method == "POST" {
		if err := c.Request.ParseForm(); err == nil {
			for k, v := range c.Request.PostForm {
				if len(v) > 0 {
					params[k] = v[0]
				}
			}
		}
	}

	orderNo := params["orderNo"]
	if orderNo == "" {
		orderNo = params["mchOrderNo"]
	}
	amountStr := params["amount"]
	if amountStr == "" {
		amountStr = params["actualPayAmount"]
	}

	sigGot := params["sign"]
	sigState := params["state"]
	if sigState == "" {
		sigState = "1"
	}
	sigExpected := h.signNotify(sigState, orderNo, amountStr)

	if sigGot != sigExpected {
		log.Printf("[zhifux-webhook] sign mismatch: got=%s expected=%s params=%v", sigGot, sigExpected, params)
		c.String(400, "sign error")
		return
	}

	state := params["orderState"]
	if state == "" {
		state = params["trade_status"]
	}
	if state != "" && state != "1" && state != "2" && state != "success" && state != "TRADE_SUCCESS" {
		log.Printf("[zhifux-webhook] non-success state: %s", state)
		c.String(200, "success")
		return
	}

	var order models.ZhifuxOrder
	if err := h.db.Where("order_no = ? AND status = ?", orderNo, "pending").First(&order).Error; err != nil {
		log.Printf("[zhifux-webhook] order not found or already processed: %s", orderNo)
		c.String(200, "success")
		return
	}

	if err := h.processRecharge(&order); err != nil {
		log.Printf("[zhifux-webhook] processRecharge failed: %v", err)
		c.String(500, "recharge failed")
		return
	}

	log.Printf("[zhifux-webhook] recharge OK: user=%s amount=CNY%.2f order=%s", order.UserID, order.Amount, order.OrderNo)
	c.String(200, "success")
}

// processRecharge: 参考 Stripe webhook 逻辑, 查 stripeTiers -> 加余额/首充/会员/RPM 
func (h *ZhifuxHandler) processRecharge(order *models.ZhifuxOrder) error {
	userID := order.UserID.String()

	// 1. 反查 stripeTier: 优先用 TierID, 无则按 CNY 金额匹配 tier, 都不匹配走 custom
	var tier stripeTier
	amountCNY := int(order.Amount + 0.5)
	tierID := order.TierID
	if tierID == "" {
		// 兼容: 如果前端没传 tier_id, 按金额猜
		if t, ok := stripeTiers[fmt.Sprintf("%d", amountCNY)]; ok {
			tier = t
			tierID = fmt.Sprintf("%d", amountCNY)
		} else if amountCNY == 99 {
			tier = stripeTiers["pro"]
			tierID = "pro"
		} else if amountCNY == 499 {
			tier = stripeTiers["enterprise"]
			tierID = "enterprise"
		} else {
			tier = computeCustomTier(amountCNY)
			tierID = "custom"
		}
	} else {
		if t, ok := stripeTiers[tierID]; ok {
			tier = t
		} else if tierID == "custom" {
			tier = computeCustomTier(amountCNY)
		} else {
			// 未知 tier_id, 兜底
			tier = computeCustomTier(amountCNY)
		}
	}

	isUpgrade := tier.UpgradesToTier != ""
	paidUSD := float64(amountCNY) // 1 CNY = 1 USD balance (跟 Stripe 一致)
	actualAmount := tier.BalanceUSD
	bonus := 0.0
	intent := "balance"
	if isUpgrade {
		intent = "membership"
		bonus = tier.BalanceUSD - paidUSD
	} else {
		bonus = tier.BonusUSD
		actualAmount += bonus
	}

	tx := h.db.Begin()

	var balanceBefore float64
	var existingTier string
	var existingExpiresAt, existingStartedAt, existingFirstRechargeAt sql.NullTime
	err := tx.Raw("SELECT balance, membership_tier, membership_expires_at, membership_started_at, first_recharge_at FROM users WHERE id = ?::uuid", userID).
		Row().Scan(&balanceBefore, &existingTier, &existingExpiresAt, &existingStartedAt, &existingFirstRechargeAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("read user: %v", err)
	}

	// 首充赠送
	isFirstRecharge := !existingFirstRechargeAt.Valid
	if isFirstRecharge && !isUpgrade {
		minAmount := 0.0
		if minStr := h.readSetting("first_recharge_min_amount"); minStr != "" {
			if v, err := strconv.ParseFloat(minStr, 64); err == nil {
				minAmount = v
			}
		}
		if paidUSD >= minAmount {
			if firstBonusStr := h.readSetting("first_recharge_bonus"); firstBonusStr != "" {
				if firstBonus, err := strconv.ParseFloat(firstBonusStr, 64); err == nil && firstBonus > 0 {
					bonus += firstBonus
					actualAmount += firstBonus
					log.Printf("[zhifux-webhook] first recharge bonus: user=%s +$%.2f (paid $%.2f >= min $%.2f)", userID, firstBonus, paidUSD, minAmount)
				}
			}
		}
	}

	newBalance := balanceBefore + actualAmount

	if err := tx.Exec("UPDATE users SET balance = balance + ? WHERE id = ?::uuid", actualAmount, userID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("add balance: %v", err)
	}

	// 会员升级
	if isUpgrade {
		now := time.Now()
		startTime := now
		if existingExpiresAt.Valid && existingExpiresAt.Time.After(now) && existingTier == tier.UpgradesToTier {
			startTime = existingExpiresAt.Time
		}
		newExpiry := startTime.Add(time.Duration(tier.DurationDays) * 24 * time.Hour)
		if !existingStartedAt.Valid {
			tx.Exec("UPDATE users SET membership_tier = ?, membership_expires_at = ?, membership_started_at = ? WHERE id = ?::uuid",
				tier.UpgradesToTier, newExpiry, now, userID)
		} else {
			tx.Exec("UPDATE users SET membership_tier = ?, membership_expires_at = ? WHERE id = ?::uuid",
				tier.UpgradesToTier, newExpiry, userID)
		}
		rpm, tpm := rpmTpmForTier(tier.UpgradesToTier)
		tx.Exec("UPDATE api_keys SET rpm_limit = ?, tpm_limit = ? WHERE user_id = ?::uuid AND deleted_at IS NULL", rpm, tpm, userID)
		log.Printf("[zhifux-webhook] upgraded user %s to %s, set RPM=%d TPM=%d", userID, tier.UpgradesToTier, rpm, tpm)
	}

	// 更新 zhifux_orders + 写 recharge_orders + billing_records
	if err := tx.Model(order).Updates(map[string]interface{}{
		"status":  "paid",
		"paid_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("update zhifux order: %v", err)
	}

	paymentMethod := "alipay"
	if order.PayType == "tloop" || order.PayType == "wxpay" {
		paymentMethod = "wechat"
	}
	var upgradesToTierVal interface{}
	if isUpgrade {
		upgradesToTierVal = tier.UpgradesToTier
	}
	if err := tx.Exec(
		"INSERT INTO recharge_orders (user_id, order_no, amount, bonus_amount, payment_method, payment_status, payment_id, paid_at, created_at, updated_at, intent, upgrades_to_tier) VALUES (?::uuid, ?, ?, ?, ?, 'paid', ?, NOW(), NOW(), NOW(), ?, ?) ON CONFLICT (order_no) DO NOTHING",
		userID, order.OrderNo, paidUSD, bonus, paymentMethod, order.PlatformOrderID, intent, upgradesToTierVal,
	).Error; err != nil {
		log.Printf("[zhifux-webhook] insert recharge_orders failed (non-fatal): %v", err)
	}

	if isFirstRecharge {
		tx.Exec("UPDATE users SET first_recharge_at = NOW() WHERE id = ?::uuid AND first_recharge_at IS NULL", userID)
	}

	var desc string
	if isUpgrade {
		desc = fmt.Sprintf("Zhifux recharge $%.2f (upgrade %s, received $%.2f)", paidUSD, tier.DisplayName, actualAmount)
	} else {
		desc = fmt.Sprintf("Zhifux recharge $%.2f", actualAmount)
	}
	tx.Exec("INSERT INTO billing_records (user_id, type, amount, balance_before, balance_after, description, created_at) VALUES (?::uuid, 'recharge', ?, ?, ?, ?, NOW())",
		userID, actualAmount, balanceBefore, newBalance, desc)

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit: %v", err)
	}

	if h.engine != nil {
		_ = h.engine.InitBalance(userID)
	}

	log.Printf("[zhifux-webhook] processed: user=%s paid=$%.2f received=$%.2f tier=%s upgrade=%v", userID, paidUSD, actualAmount, tierID, isUpgrade)
	return nil
}

func (h *ZhifuxHandler) QueryOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("order_no"))
	userIDRaw, _ := c.Get("user_id")
	userID := userIDRaw.(string)

	var order models.ZhifuxOrder
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"amount":   order.Amount,
	})
}
