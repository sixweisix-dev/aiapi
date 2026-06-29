package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ai-api-gateway/internal/billing"
)

const stripeAPIBase = "https://api.stripe.com/v1"

type StripeHandler struct {
	db     *gorm.DB
	engine *billing.Engine
}

func NewStripeHandler(db *gorm.DB, engine *billing.Engine) *StripeHandler {
	return &StripeHandler{db: db, engine: engine}
}

type stripeTier struct {
	AmountCNY      int
	PriceCents     int64
	BalanceUSD     float64
	BonusUSD       float64
	UpgradesToTier string
	DurationDays   int
	Name           string
	DisplayName    string
}

// USD → HKD 换算汇率 (HK Stripe 账户必须用 HKD 才能显示 wechat_pay)
// 实时汇率参考 https://www.xe.com/currencyconverter/convert/?From=USD&To=HKD
const usdToHKDx100 = 784 // 1 USD = 7.84 HKD; 用整数 x100 避免浮点

var stripeTiers = map[string]stripeTier{
	"100":        {100, 1429, 100, 8, "", 0, "TransitAI Recharge 100", "$100 Balance"},
	"300":        {300, 4286, 300, 30, "", 0, "TransitAI Recharge 300", "$300 Balance"},
	"500":        {500, 7143, 500, 75, "", 0, "TransitAI Recharge 500", "$500 Balance"},
	"1000":       {1000, 14286, 1000, 200, "", 0, "TransitAI Recharge 1000", "$1000 Balance"},
	"3000":       {3000, 42857, 3000, 750, "", 0, "TransitAI Recharge 3000", "$3000 Balance"},
	"pro":        {99, 1414, 120, 0, "pro", 30, "TransitAI Pro Membership (30 days)", "Pro Membership"},
	"enterprise": {499, 7129, 600, 0, "enterprise", 30, "TransitAI Enterprise Membership (30 days)", "Enterprise Membership"},
}


// computeCustomTier: 根据自定义 CNY 金额计算 stripeTier (线性插值赠送比例)
// 规则: <100 无赠送; 100-300=8%~10%; 300-500=10%~15%; 500-1000=15%~20%;
// 1000-3000=20%~25%; >=3000 固定 25% (与 3000 档对齐, 最高赠送)
func computeCustomTier(amountCNY int) stripeTier {
	var pct float64
	switch {
	case amountCNY < 100:
		pct = 0
	case amountCNY < 300:
		pct = 8 + float64(amountCNY-100)/200.0*(10-8)
	case amountCNY < 500:
		pct = 10 + float64(amountCNY-300)/200.0*(15-10)
	case amountCNY < 1000:
		pct = 15 + float64(amountCNY-500)/500.0*(20-15)
	case amountCNY < 3000:
		pct = 20 + float64(amountCNY-1000)/2000.0*(25-20)
	default:
		pct = 25
	}
	balanceUSD := float64(amountCNY)
	bonusUSD := balanceUSD * pct / 100.0
	priceCents := int64(float64(amountCNY)*100.0/7.0 + 0.5)
	return stripeTier{
		AmountCNY:   amountCNY,
		PriceCents:  priceCents,
		BalanceUSD:  balanceUSD,
		BonusUSD:    bonusUSD,
		Name:        fmt.Sprintf("TransitAI Recharge %d (Custom)", amountCNY),
		DisplayName: fmt.Sprintf("$%.0f Balance", balanceUSD+bonusUSD),
	}
}


func (h *StripeHandler) readSetting(key string) string {
	var v string
	h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", key).Scan(&v)
	return v
}

func (h *StripeHandler) stripeEnabled() bool {
	return h.readSetting("stripe_enabled") == "true" && h.readSetting("stripe_secret_key") != ""
}

func (h *StripeHandler) GetStatus(c *gin.Context) {
	c.JSON(200, gin.H{"enabled": h.stripeEnabled()})
}

type checkoutRequest struct {
	TierID string `json:"tier_id"`
	AmountCNY int    `json:"amount_cny"`
}

type stripeSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func (h *StripeHandler) CreateCheckoutSession(c *gin.Context) {
	if !h.stripeEnabled() {
		c.JSON(503, gin.H{"error": "Stripe payment not enabled"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if userIDStr == "" {
		c.JSON(401, gin.H{"error": "auth required"})
		return
	}

	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TierID == "" {
		c.JSON(400, gin.H{"error": "missing tier_id"})
		return
	}

	var tier stripeTier
	var ok bool
	if req.TierID == "custom" {
		if req.AmountCNY < 10 {
			c.JSON(400, gin.H{"error": "custom amount must be >= 10 CNY"})
			return
		}
		if req.AmountCNY > 10000 {
			c.JSON(400, gin.H{"error": "custom amount must be <= 10000 CNY"})
			return
		}
		tier = computeCustomTier(req.AmountCNY)
		ok = true
	} else {
		tier, ok = stripeTiers[req.TierID]
	}
	if !ok {
		c.JSON(400, gin.H{"error": "invalid tier_id"})
		return
	}

	secretKey := h.readSetting("stripe_secret_key")
	successURL := h.readSetting("stripe_success_url")
	if successURL == "" {
		successURL = "https://transitai.cloud/recharge?stripe=success&session={CHECKOUT_SESSION_ID}"
	}
	cancelURL := h.readSetting("stripe_cancel_url")
	if cancelURL == "" {
		cancelURL = "https://transitai.cloud/recharge?stripe=cancel"
	}

	var productDesc string
	if tier.UpgradesToTier != "" {
		productDesc = fmt.Sprintf("%d-day %s + $%.0f balance included", tier.DurationDays, tier.DisplayName, tier.BalanceUSD)
	} else {
		productDesc = fmt.Sprintf("Adds $%.0f balance (CNY %d tier)", tier.BalanceUSD+tier.BonusUSD, tier.AmountCNY)
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Add("payment_method_types[]", "card")
	form.Add("payment_method_types[]", "alipay")
	form.Add("payment_method_types[]", "wechat_pay")
	form.Set("payment_method_options[wechat_pay][client]", "web")
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "hkd")
	hkdCents := tier.PriceCents * usdToHKDx100 / 100
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(hkdCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", tier.Name)
	form.Set("line_items[0][price_data][product_data][description]", productDesc)
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("metadata[user_id]", userIDStr)
	form.Set("metadata[tier_id]", req.TierID)
	if req.TierID == "custom" {
		form.Set("metadata[amount_cny]", strconv.Itoa(req.AmountCNY))
	}

	httpReq, _ := http.NewRequest("POST", stripeAPIBase+"/checkout/sessions", strings.NewReader(form.Encode()))
	httpReq.SetBasicAuth(secretKey, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[stripe] checkout request failed: %v", err)
		c.JSON(502, gin.H{"error": "Stripe request failed"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Printf("[stripe] checkout failed [%d]: %s", resp.StatusCode, string(body))
		c.JSON(resp.StatusCode, gin.H{"error": "Stripe API error", "details": string(body)})
		return
	}

	var session stripeSession
	if err := json.Unmarshal(body, &session); err != nil {
		c.JSON(500, gin.H{"error": "parse Stripe response failed"})
		return
	}

	log.Printf("[stripe] checkout session created: user=%s tier=%s session=%s", userIDStr, req.TierID, session.ID)
	c.JSON(200, gin.H{"url": session.URL, "session_id": session.ID})
}

type stripeEvent struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type checkoutSessionObject struct {
	Object struct {
		ID            string            `json:"id"`
		PaymentStatus string            `json:"payment_status"`
		AmountTotal   int64             `json:"amount_total"`
		Currency      string            `json:"currency"`
		Metadata      map[string]string `json:"metadata"`
	} `json:"object"`
}


// rpmTpmForTier 返回不同会员等级对应的 RPM/TPM 限制
func rpmTpmForTier(tier string) (int, int) {
	switch tier {
	case "pro":
		return 60, 100000
	case "enterprise":
		return 600, 1000000
	default:
		return 10, 30000 // free
	}
}

func (h *StripeHandler) HandleWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read body failed"})
		return
	}

	webhookSecret := h.readSetting("stripe_webhook_secret")
	sigHeader := c.GetHeader("Stripe-Signature")
	if webhookSecret == "" {
		log.Printf("[stripe-webhook] webhook secret not configured")
		c.JSON(503, gin.H{"error": "webhook not configured"})
		return
	}
	if err := verifyStripeSignature(payload, sigHeader, webhookSecret); err != nil {
		log.Printf("[stripe-webhook] signature verification failed: %v", err)
		c.JSON(400, gin.H{"error": "signature verification failed"})
		return
	}

	var event stripeEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		c.JSON(400, gin.H{"error": "parse event failed"})
		return
	}
	log.Printf("[stripe-webhook] received event %s type=%s", event.ID, event.Type)

	if event.Type != "checkout.session.completed" {
		c.JSON(200, gin.H{"received": true, "ignored": true})
		return
	}

	var wrap checkoutSessionObject
	if err := json.Unmarshal(event.Data, &wrap); err != nil {
		c.JSON(200, gin.H{"received": true, "warning": "unparseable data"})
		return
	}
	if wrap.Object.PaymentStatus != "paid" {
		c.JSON(200, gin.H{"received": true, "ignored": "not paid"})
		return
	}

	sess := wrap.Object
	userID := sess.Metadata["user_id"]
	tierID := sess.Metadata["tier_id"]
	var tier stripeTier
	var ok bool
	if tierID == "custom" {
		amt, _ := strconv.Atoi(sess.Metadata["amount_cny"])
		if amt >= 10 {
			tier = computeCustomTier(amt)
			ok = true
		}
	} else {
		tier, ok = stripeTiers[tierID]
	}
	if !ok || userID == "" {
		log.Printf("[stripe-webhook] invalid metadata user=%s tier=%s", userID, tierID)
		c.JSON(200, gin.H{"received": true, "warning": "invalid metadata"})
		return
	}

	isUpgrade := tier.UpgradesToTier != ""
	promo := h.readSetting("recharge_promo_enabled") == "true"

	paidUSD := float64(sess.AmountTotal) / 100.0
	actualAmount := tier.BalanceUSD
	var bonus float64
	intent := "balance"
	if isUpgrade {
		intent = "membership"
		bonus = tier.BalanceUSD - paidUSD
	} else if promo {
		bonus = tier.BonusUSD
		actualAmount += bonus
	}

	orderNo := "stripe_" + sess.ID
	tx := h.db.Begin()

	var balanceBefore float64
	var existingTier string
	var existingExpiresAt, existingStartedAt, existingFirstRechargeAt sql.NullTime
	err = tx.Raw("SELECT balance, membership_tier, membership_expires_at, membership_started_at, first_recharge_at FROM users WHERE id = ?::uuid", userID).
		Row().Scan(&balanceBefore, &existingTier, &existingExpiresAt, &existingStartedAt, &existingFirstRechargeAt)
	if err != nil {
		tx.Rollback()
		log.Printf("[stripe-webhook] read user failed: %v", err)
		c.JSON(500, gin.H{"error": "read user failed"})
		return
	}

	// 首充赠送 (非 membership 升级, 充值金额 >= first_recharge_min_amount)
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
					log.Printf("[stripe-webhook] first recharge bonus: user=%s +$%.2f (paid $%.2f >= min $%.2f)", userID, firstBonus, paidUSD, minAmount)
				}
			}
		} else {
			log.Printf("[stripe-webhook] first recharge user=%s paid $%.2f < min $%.2f, no bonus", userID, paidUSD, minAmount)
		}
	}

	newBalance := balanceBefore + actualAmount

	if err := tx.Exec("UPDATE users SET balance = balance + ? WHERE id = ?::uuid", actualAmount, userID).Error; err != nil {
		tx.Rollback()
		log.Printf("[stripe-webhook] update balance failed: %v", err)
		c.JSON(500, gin.H{"error": "add balance failed"})
		return
	}

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
		// 同步升级该用户所有 API Key 的 RPM/TPM 限制
		rpm, tpm := rpmTpmForTier(tier.UpgradesToTier)
		tx.Exec("UPDATE api_keys SET rpm_limit = ?, tpm_limit = ? WHERE user_id = ?::uuid AND deleted_at IS NULL", rpm, tpm, userID)
		log.Printf("[stripe-webhook] upgraded user %s to %s, set RPM=%d TPM=%d on all API keys", userID, tier.UpgradesToTier, rpm, tpm)
	}

	var upgradesToTierVal interface{}
	if isUpgrade {
		upgradesToTierVal = tier.UpgradesToTier
	}
	insertErr := tx.Exec("INSERT INTO recharge_orders (user_id, order_no, amount, bonus_amount, payment_method, payment_status, payment_id, paid_at, created_at, updated_at, intent, upgrades_to_tier) VALUES (?::uuid, ?, ?, ?, 'stripe', 'paid', ?, NOW(), NOW(), NOW(), ?, ?) ON CONFLICT (order_no) DO NOTHING",
		userID, orderNo, paidUSD, bonus, sess.ID, intent, upgradesToTierVal).Error
	if insertErr != nil {
		log.Printf("[stripe-webhook] insert order failed (non-fatal): %v", insertErr)
	}

	// 标记首充时间 (仅当 first_recharge_at 还是 NULL 时)
	if isFirstRecharge {
		tx.Exec("UPDATE users SET first_recharge_at = NOW() WHERE id = ?::uuid AND first_recharge_at IS NULL", userID)
	}

	var desc string
	if isUpgrade {
		desc = fmt.Sprintf("Stripe recharge $%.2f (upgrade %s, received $%.2f)", paidUSD, tier.DisplayName, actualAmount)
	} else {
		desc = fmt.Sprintf("Stripe recharge $%.2f", actualAmount)
	}
	tx.Exec("INSERT INTO billing_records (user_id, type, amount, balance_before, balance_after, description, created_at) VALUES (?::uuid, 'recharge', ?, ?, ?, ?, NOW())",
		userID, actualAmount, balanceBefore, newBalance, desc)

	tx.Commit()

	if h.engine != nil {
		_ = h.engine.InitBalance(userID)
	}

	log.Printf("[stripe-webhook] processed: user=%s paid=$%.2f received=$%.2f tier=%s upgrade=%v session=%s",
		userID, paidUSD, actualAmount, tierID, isUpgrade, sess.ID)

	c.JSON(200, gin.H{"received": true})
}

func verifyStripeSignature(payload []byte, sigHeader, secret string) error {
	if sigHeader == "" {
		return errors.New("missing signature header")
	}
	parts := strings.Split(sigHeader, ",")
	var ts, sig string
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sig = kv[1]
		}
	}
	if ts == "" || sig == "" {
		return errors.New("malformed signature")
	}
	signedPayload := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return errors.New("signature mismatch")
	}
	return nil
}
