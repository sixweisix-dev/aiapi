package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
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
	AmountCNY  int
	PriceCents int64
	BalanceUSD float64
	BonusUSD   float64
	Name       string
}

var stripeTiers = map[string]stripeTier{
	"100":  {100, 1429, 100, 8, "TransitAI Recharge 100"},
	"300":  {300, 4286, 300, 30, "TransitAI Recharge 300"},
	"500":  {500, 7143, 500, 75, "TransitAI Recharge 500"},
	"1000": {1000, 14286, 1000, 200, "TransitAI Recharge 1000"},
	"3000": {3000, 42857, 3000, 750, "TransitAI Recharge 3000"},
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

	tier, ok := stripeTiers[req.TierID]
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

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("payment_method_types[]", "card")
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", "usd")
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(tier.PriceCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", tier.Name)
	form.Set("line_items[0][price_data][product_data][description]",
		fmt.Sprintf("Adds $%.0f balance (CNY %d tier)", tier.BalanceUSD+tier.BonusUSD, tier.AmountCNY))
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("metadata[user_id]", userIDStr)
	form.Set("metadata[tier_id]", req.TierID)

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

	if event.Type == "checkout.session.completed" {
		var wrap checkoutSessionObject
		if err := json.Unmarshal(event.Data, &wrap); err == nil && wrap.Object.PaymentStatus == "paid" {
			sess := wrap.Object
			userID := sess.Metadata["user_id"]
			tierID := sess.Metadata["tier_id"]
			tier, ok := stripeTiers[tierID]
			if !ok || userID == "" {
				log.Printf("[stripe-webhook] invalid metadata user=%s tier=%s", userID, tierID)
				c.JSON(200, gin.H{"received": true, "warning": "invalid metadata"})
				return
			}

			promo := h.readSetting("recharge_promo_enabled") == "true"
			balance := tier.BalanceUSD
			bonus := 0.0
			if promo {
				bonus = tier.BonusUSD
				balance += bonus
			}

			orderNo := "stripe_" + sess.ID
			tx := h.db.Begin()
			if err := tx.Exec("UPDATE users SET balance = balance + ? WHERE id = ?::uuid", balance, userID).Error; err != nil {
				tx.Rollback()
				log.Printf("[stripe-webhook] update balance failed: %v", err)
				c.JSON(500, gin.H{"error": "add balance failed"})
				return
			}
			insertErr := tx.Exec("INSERT INTO recharge_orders (user_id, order_no, amount, bonus_amount, payment_method, payment_status, payment_id, paid_at, created_at, updated_at, intent) VALUES (?::uuid, ?, ?, ?, 'stripe', 'paid', ?, NOW(), NOW(), NOW(), 'balance') ON CONFLICT (order_no) DO NOTHING",
				userID, orderNo, float64(sess.AmountTotal)/100.0, bonus, sess.ID).Error
			if insertErr != nil {
				log.Printf("[stripe-webhook] insert order failed (non-fatal): %v", insertErr)
			}
			tx.Commit()

			if h.engine != nil {
				_ = h.engine.InitBalance(userID)
			}

			log.Printf("[stripe-webhook] balance added: user=%s amount=$%.2f balance+=$%.2f session=%s",
				userID, float64(sess.AmountTotal)/100.0, balance, sess.ID)
		}
	}

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
