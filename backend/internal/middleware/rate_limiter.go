package middleware

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter Redis 滑动窗口限流器
type RateLimiter struct {
	rdb *redis.Client
}

func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// CheckRPM 检查请求/分钟，返回是否超限及当前窗口请求数
func (rl *RateLimiter) CheckRPM(ctx context.Context, key string, limit int) (allowed bool, current int, err error) {
	if limit <= 0 || rl.rdb == nil {
		return true, 0, nil
	}

	redisKey := fmt.Sprintf("rl:rpm:%s:%d", key, time.Now().Unix()/60)
	count, err := rl.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		log.Printf("[rate_limiter] redis err: %v (allowing request)", err)
		return true, 0, err
	}
	if count == 1 {
		rl.rdb.Expire(ctx, redisKey, 90*time.Second)
	}
	return int(count) <= limit, int(count), nil
}

// AddTokens 给 TPM 计数加上消耗的 tokens（请求成功后调用）
func (rl *RateLimiter) AddTokens(ctx context.Context, key string, tokens int) {
	if tokens <= 0 || rl.rdb == nil {
		return
	}
	redisKey := fmt.Sprintf("rl:tpm:%s:%d", key, time.Now().Unix()/60)
	rl.rdb.IncrBy(ctx, redisKey, int64(tokens))
	rl.rdb.Expire(ctx, redisKey, 90*time.Second)
}

// CheckTPM 检查 token/分钟（在请求开始前预估）
func (rl *RateLimiter) CheckTPM(ctx context.Context, key string, limit int) (allowed bool, current int) {
	if limit <= 0 || rl.rdb == nil {
		return true, 0
	}
	redisKey := fmt.Sprintf("rl:tpm:%s:%d", key, time.Now().Unix()/60)
	val, err := rl.rdb.Get(ctx, redisKey).Result()
	if err != nil {
		return true, 0
	}
	current, _ = strconv.Atoi(val)
	return current <= limit, current
}

// APIKeyRateLimit 中间件：基于 API Key 的 RPM/TPM 限流
// 需要在 APIKeyAuth 之后使用，因为依赖 c.Get("api_key_rpm") 等上下文
func APIKeyRateLimit(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil {
			c.Next()
			return
		}

		apiKeyID, _ := c.Get("api_key_id")
		apiKeyIDStr, _ := apiKeyID.(string)
		if apiKeyIDStr == "" {
			c.Next()
			return
		}

		// 从上下文取 RPM/TPM 限制（由 APIKeyAuth 中间件注入）
		rpmLimit := getIntFromCtx(c, "api_key_rpm")
		tpmLimit := getIntFromCtx(c, "api_key_tpm")

		if rpmLimit > 0 {
			allowed, current, _ := rl.CheckRPM(c.Request.Context(), apiKeyIDStr, rpmLimit)
			if !allowed {
				c.AbortWithStatusJSON(429, gin.H{"error": gin.H{
					"message": fmt.Sprintf("rate limit exceeded: %d / %d RPM", current, rpmLimit),
					"type":    "rate_limit_error",
				}})
				return
			}
			c.Header("X-RateLimit-Limit-Requests", fmt.Sprintf("%d", rpmLimit))
			c.Header("X-RateLimit-Remaining-Requests", fmt.Sprintf("%d", rpmLimit-current))
		}

		if tpmLimit > 0 {
			allowed, current := rl.CheckTPM(c.Request.Context(), apiKeyIDStr, tpmLimit)
			if !allowed {
				c.AbortWithStatusJSON(429, gin.H{"error": gin.H{
					"message": fmt.Sprintf("token rate limit exceeded: %d / %d TPM", current, tpmLimit),
					"type":    "rate_limit_error",
				}})
				return
			}
			c.Header("X-RateLimit-Limit-Tokens", fmt.Sprintf("%d", tpmLimit))
		}

		c.Next()
	}
}

func getIntFromCtx(c *gin.Context, key string) int {
	v, ok := c.Get(key)
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case *int64:
		if n == nil {
			return 0
		}
		return int(*n)
	}
	return 0
}
