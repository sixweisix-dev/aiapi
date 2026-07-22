package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const maintenanceKey = "maintenance:mode"

// MaintenanceMode 中间件: 维护模式期间拒绝新请求(admin/health/backup 除外)
// 前置 middleware, 放在链最前(auth 之前也能拒)
func MaintenanceMode(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 白名单路径 (维护中仍允许): admin 后台 + backup 管理 + 健康检查
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/admin/") || path == "/health" || path == "/v1/auth/login" || path == "/v1/auth/config" {
			c.Next()
			return
		}

		if rdb == nil {
			c.Next()
			return
		}
		ctx := context.Background()
		val, err := rdb.Get(ctx, maintenanceKey).Result()
		if err != nil || val != "1" {
			c.Next()
			return
		}

		// 维护中: 拒绝
		c.Header("Retry-After", "60")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "系统维护中, 请稍后重试",
				"type":    "maintenance_mode",
				"retry_after_seconds": 60,
			},
		})
	}
}
