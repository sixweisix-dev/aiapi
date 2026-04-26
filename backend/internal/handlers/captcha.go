package handlers

import (
	"net/http"
	"time"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"context"
	"fmt"
)

type CaptchaHandler struct {
	redis *redis.Client
}

func NewCaptchaHandler(redis *redis.Client) *CaptchaHandler {
	return &CaptchaHandler{redis: redis}
}

// GenerateCaptcha returns a new captcha id and image URL
func (h *CaptchaHandler) GenerateCaptcha(c *gin.Context) {
	id := captcha.New()
	c.JSON(http.StatusOK, gin.H{
		"captcha_id":  id,
		"captcha_url": "/v1/auth/captcha/" + id + ".png",
	})
}

// ServeCaptchaImage serves the captcha PNG image
func (h *CaptchaHandler) ServeCaptchaImage(c *gin.Context) {
	id := c.Param("id")
	// strip .png suffix
	if len(id) > 4 && id[len(id)-4:] == ".png" {
		id = id[:len(id)-4]
	}
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache")
	if err := captcha.WriteImage(c.Writer, id, captcha.StdWidth, captcha.StdHeight); err != nil {
		c.Status(http.StatusNotFound)
	}
}

// VerifyCaptcha checks captcha answer, returns error string if invalid
func VerifyCaptcha(captchaID, answer string) bool {
	return captcha.VerifyString(captchaID, answer)
}

// StoreEmailToken stores a password reset token in Redis
func StoreEmailToken(rdb *redis.Client, email, token string) error {
	key := fmt.Sprintf("reset_token:%s", token)
	return rdb.Set(context.Background(), key, email, 30*time.Minute).Err()
}

// GetEmailByToken retrieves email by reset token
func GetEmailByToken(rdb *redis.Client, token string) (string, error) {
	key := fmt.Sprintf("reset_token:%s", token)
	return rdb.Get(context.Background(), key).Result()
}

// DeleteToken deletes the reset token after use
func DeleteToken(rdb *redis.Client, token string) error {
	key := fmt.Sprintf("reset_token:%s", token)
	return rdb.Del(context.Background(), key).Err()
}
