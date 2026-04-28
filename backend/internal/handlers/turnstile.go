package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var globalDB *gorm.DB

// SetGlobalDB 由 main.go 注入
func SetGlobalDB(db *gorm.DB) { globalDB = db }

var turnstileClient = &http.Client{Timeout: 5 * time.Second}

func VerifyTurnstile(token, ip string) bool {
	secret := os.Getenv("TURNSTILE_SECRET_KEY")
	if secret == "" {
		log.Println("[Turnstile] WARN: TURNSTILE_SECRET_KEY not set, skipping verification")
		return true
	}
	if token == "" {
		return false
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequest("POST",
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("[Turnstile] new request error: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := turnstileClient.Do(req)
	if err != nil {
		log.Printf("[Turnstile] http error: %v", err)
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[Turnstile] decode error: %v", err)
		return false
	}
	if !result.Success {
		log.Printf("[Turnstile] verification failed: %v", result.ErrorCodes)
	}
	return result.Success
}

// AuthConfig 提供给前端的公开配置(含 Turnstile + 充值赠送规则)
type AuthConfig struct {
	TurnstileSiteKey      string `json:"turnstile_site_key"`
	RechargePromoEnabled  bool   `json:"recharge_promo_enabled"`
	RechargeTiers         string `json:"recharge_tiers"`
	FirstRechargeBonus    string `json:"first_recharge_bonus"`
}

// GetAuthConfig 返回前端公开配置
func GetAuthConfig(c *gin.Context) {
	cfg := AuthConfig{
		TurnstileSiteKey:     os.Getenv("TURNSTILE_SITE_KEY"),
		RechargePromoEnabled: true,
		RechargeTiers:        "[]",
		FirstRechargeBonus:   "0",
	}
	if globalDB != nil {
		cfg.RechargePromoEnabled = GetSettingValue(globalDB, "recharge_promo_enabled", "true") == "true"
		cfg.RechargeTiers = GetSettingValue(globalDB, "recharge_tiers", "[]")
		cfg.FirstRechargeBonus = GetSettingValue(globalDB, "first_recharge_bonus", "0")
	}
	c.JSON(http.StatusOK, cfg)
}
