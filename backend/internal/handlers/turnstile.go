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
)

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

func GetAuthConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"turnstile_site_key": os.Getenv("TURNSTILE_SITE_KEY"),
	})
}
