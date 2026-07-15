package handlers

import (
	"strings"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"ai-api-gateway/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type OAuthHandler struct {
	db        *gorm.DB
	rdb       *redis.Client
	jwtSecret string
}

func NewOAuthHandler(db *gorm.DB, rdb *redis.Client, jwtSecret string) *OAuthHandler {
	return &OAuthHandler{db: db, rdb: rdb, jwtSecret: jwtSecret}
}

// -------- 通用 helpers --------

func randState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 存 state 到 Redis, 10 分钟有效
func (h *OAuthHandler) saveState(provider, state string) {
	h.rdb.Set(context.Background(), "oauth:state:"+provider+":"+state, "1", 10*time.Minute)
}

// 消费 state (一次性)
func (h *OAuthHandler) consumeState(provider, state string) bool {
	if state == "" {
		return false
	}
	ctx := context.Background()
	key := "oauth:state:" + provider + ":" + state
	n, _ := h.rdb.Exists(ctx, key).Result()
	if n == 0 {
		return false
	}
	h.rdb.Del(ctx, key)
	return true
}

// 用 email+provider_id 查/建用户, 返回 user 和是否新建
func (h *OAuthHandler) upsertUser(provider, providerID, email, username string) (*models.User, bool, error) {
	if email == "" {
		return nil, false, fmt.Errorf("email required")
	}

	var user models.User
	var isNew bool

	// 1. 先按 provider_id 查
	var col string
	switch provider {
	case "github":
		col = "github_id"
	case "google":
		col = "google_id"
	default:
		return nil, false, fmt.Errorf("unknown provider")
	}

	err := h.db.Where(col+" = ?", providerID).First(&user).Error
	if err == nil {
		return &user, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	// 2. 按 email 查(自动关联)
	err = h.db.Where("email = ?", email).First(&user).Error
	if err == nil {
		// 已有邮箱账号, 关联 provider_id
		upd := map[string]interface{}{col: providerID}
		if err := h.db.Model(&user).Updates(upd).Error; err != nil {
			return nil, false, err
		}
		return &user, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	// 3. 全新用户
	isNew = true
	user = models.User{
		Email:    email,
		Role:     "user",
		IsActive: true,
		EmailVerified: true, // OAuth 提供商已验证
	}
	if provider == "github" {
		user.GithubID = &providerID
	}
	if provider == "google" {
		user.GoogleID = &providerID
	}
	if username != "" {
		user.Username = &username
	}
	if err := h.db.Create(&user).Error; err != nil {
		return nil, false, err
	}

	// 送试用余额(同 IP 当日 1 次)
	bonusStr := GetSettingValue(h.db, "signup_bonus", "0")
	if bonusF, perr := strconv.ParseFloat(bonusStr, 64); perr == nil && bonusF > 0 {
		if err := h.db.Model(&user).Update("balance", gorm.Expr("balance + ?", bonusF)).Error; err == nil {
			user.Balance += bonusF
			log.Printf("[SignupBonus] oauth user=%s amount=%.2f provider=%s", user.ID, bonusF, provider)
		}
	}

	return &user, isNew, nil
}

// 生成 JWT 并跳转前端
func (h *OAuthHandler) issueTokenAndRedirect(c *gin.Context, user *models.User) {
	token, err := generateJWTWithSecret(user.ID.String(), user.Email, user.Role, h.jwtSecret)
	if err != nil {
		c.String(http.StatusInternalServerError, "token generation failed")
		return
	}
	// 跳到前端 /oauth-callback?token=xxx  前端处理 token 存储
	redirect := "/oauth-callback?token=" + url.QueryEscape(token)
	c.Redirect(http.StatusFound, redirect)
}

// -------- GitHub --------

func (h *OAuthHandler) GithubStart(c *gin.Context) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		c.String(http.StatusServiceUnavailable, "GitHub OAuth not configured")
		return
	}
	state := randState()
	h.saveState("github", state)
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://" + c.Request.Host
	}
	redirectURI := baseURL + "/v1/auth/github/callback"
	authURL := "https://github.com/login/oauth/authorize?client_id=" + clientID +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&scope=" + url.QueryEscape("read:user user:email") +
		"&state=" + state
	c.Redirect(http.StatusFound, authURL)
}

func (h *OAuthHandler) GithubCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if code == "" || !h.consumeState("github", state) {
		c.String(http.StatusBadRequest, "invalid state or code")
		return
	}

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	// 交换 access_token
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", nil)
	req.URL.RawQuery = form.Encode()
	req.Header.Set("Accept", "application/json")
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "github token exchange failed")
		return
	}
	defer resp.Body.Close()
	var tokResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil || tokResp.AccessToken == "" {
		log.Printf("[GitHub OAuth] token exchange error: %v resp=%+v", err, tokResp)
		c.String(http.StatusBadGateway, "invalid github response")
		return
	}

	// 拿用户信息
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")
	userResp, err := httpClient.Do(userReq)
	if err != nil {
		c.String(http.StatusBadGateway, "github user fetch failed")
		return
	}
	defer userResp.Body.Close()
	var gu struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&gu); err != nil {
		c.String(http.StatusBadGateway, "invalid github user data")
		return
	}

	// email 可能是 null(用户设为私密), 需要再拉 /user/emails
	email := gu.Email
	if email == "" {
		emailReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
		emailReq.Header.Set("Accept", "application/vnd.github+json")
		emailResp, err := httpClient.Do(emailReq)
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
			}
		}
	}
	if email == "" {
		c.String(http.StatusBadRequest, "无法获取 GitHub 邮箱, 请在 GitHub 设置里公开主邮箱或换用邮箱注册")
		return
	}

	user, _, err := h.upsertUser("github", strconv.FormatInt(gu.ID, 10), email, gu.Login)
	if err != nil {
		log.Printf("[GitHub OAuth] upsert error: %v", err)
		c.String(http.StatusInternalServerError, "user creation failed")
		return
	}

	h.issueTokenAndRedirect(c, user)
}

// -------- Google --------

func (h *OAuthHandler) GoogleStart(c *gin.Context) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		c.String(http.StatusServiceUnavailable, "Google OAuth not configured")
		return
	}
	state := randState()
	h.saveState("google", state)
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://" + c.Request.Host
	}
	redirectURI := baseURL + "/v1/auth/google/callback"
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" +
		"client_id=" + url.QueryEscape(clientID) +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&response_type=code" +
		"&scope=" + url.QueryEscape("openid email profile") +
		"&state=" + state +
		"&access_type=online" +
		"&prompt=select_account"
	c.Redirect(http.StatusFound, authURL)
}

func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if code == "" || !h.consumeState("google", state) {
		c.String(http.StatusBadRequest, "invalid state or code")
		return
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://" + c.Request.Host
	}
	redirectURI := baseURL + "/v1/auth/google/callback"

	// 交换 access_token
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "google token exchange failed")
		return
	}
	defer resp.Body.Close()

	var tokResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil || tokResp.AccessToken == "" {
		log.Printf("[Google OAuth] token exchange error: %v resp=%+v", err, tokResp)
		c.String(http.StatusBadGateway, "invalid google response")
		return
	}

	// 拿用户信息
	userReq, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokResp.AccessToken)
	userResp, err := httpClient.Do(userReq)
	if err != nil {
		c.String(http.StatusBadGateway, "google user fetch failed")
		return
	}
	defer userResp.Body.Close()
	var gu struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&gu); err != nil {
		c.String(http.StatusBadGateway, "invalid google user data")
		return
	}
	if gu.Email == "" || !gu.VerifiedEmail {
		c.String(http.StatusBadRequest, "Google 邮箱未验证")
		return
	}

	user, _, err := h.upsertUser("google", gu.ID, gu.Email, gu.Name)
	if err != nil {
		log.Printf("[Google OAuth] upsert error: %v", err)
		c.String(http.StatusInternalServerError, "user creation failed")
		return
	}

	h.issueTokenAndRedirect(c, user)
}

