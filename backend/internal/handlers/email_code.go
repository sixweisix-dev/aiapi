package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type EmailCodeHandler struct {
	db      *gorm.DB
	rdb     *redis.Client
	mailCfg MailConfig
}

func NewEmailCodeHandler(db *gorm.DB, rdb *redis.Client, mailCfg MailConfig) *EmailCodeHandler {
	return &EmailCodeHandler{db: db, rdb: rdb, mailCfg: mailCfg}
}

type SendCodeRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token" binding:"required"`
	Purpose        string `json:"purpose" binding:"required,oneof=register"`
}

// keys
//   ec:code:<purpose>:<email>     -> 6位数字, 10分钟
//   ec:cd:<email>                 -> 冷却标记, 60秒
//   ec:daily:<email>              -> 当日发送计数, 24小时
//   ec:ip:<ip>                    -> 当日IP计数, 24小时

func (h *EmailCodeHandler) SendCode(c *gin.Context) {
	var req SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !VerifyTurnstile(req.TurnstileToken, c.ClientIP()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "人机验证失败"})
		return
	}

	if req.Purpose == "register" {
		var count int64
		h.db.Model(&UserModel{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱已注册"})
			return
		}
	}

	ctx := context.Background()
	cdKey := "ec:cd:" + req.Email
	dailyKey := "ec:daily:" + req.Email
	ipKey := "ec:ip:" + c.ClientIP()
	codeKey := "ec:code:" + req.Purpose + ":" + req.Email

	if n, _ := h.rdb.Exists(ctx, cdKey).Result(); n > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "操作过于频繁，请 60 秒后再试"})
		return
	}
	daily, _ := h.rdb.Get(ctx, dailyKey).Int()
	if daily >= 5 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "该邮箱今日发送次数已达上限"})
		return
	}
	ipDaily, _ := h.rdb.Get(ctx, ipKey).Int()
	if ipDaily >= 20 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
		return
	}

	code := genCode6()
	if err := SendVerifyCodeEmail(h.mailCfg, req.Email, code); err != nil {
		fmt.Printf("[EmailCode] send error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "邮件发送失败，请稍后重试"})
		return
	}

	h.rdb.Set(ctx, codeKey, code, 10*time.Minute)
	h.rdb.Set(ctx, cdKey, "1", 60*time.Second)
	h.rdb.Incr(ctx, dailyKey)
	h.rdb.Expire(ctx, dailyKey, 24*time.Hour)
	h.rdb.Incr(ctx, ipKey)
	h.rdb.Expire(ctx, ipKey, 24*time.Hour)

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送，请查收邮件"})
}

func genCode6() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// VerifyEmailCode 校验并消费验证码(成功后即删除,一次性)
func VerifyEmailCode(rdb *redis.Client, purpose, email, code string) bool {
	if code == "" {
		return false
	}
	ctx := context.Background()
	key := "ec:code:" + purpose + ":" + email
	stored, err := rdb.Get(ctx, key).Result()
	if err != nil || stored != code {
		return false
	}
	rdb.Del(ctx, key)
	return true
}

func SendVerifyCodeEmail(cfg MailConfig, toEmail, code string) error {
	port, err := strconv.Atoi(cfg.Port)
	if err != nil {
		port = 587
	}
	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	subject := "TransitAI 注册验证码"
	body := fmt.Sprintf(`您好，

您的 TransitAI 注册验证码是：

%s

10 分钟内有效。如非本人操作，请忽略此邮件。

TransitAI 团队`, code)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, toEmail, subject, body)
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	return smtp.SendMail(addr, auth, cfg.From, []string{toEmail}, []byte(msg))
}
