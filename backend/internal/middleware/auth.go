package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Claims represents JWT claims.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuth returns middleware that validates JWT Bearer tokens.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("auth_method", "jwt")
		c.Next()
	}
}

// AdminRequired ensures the user has admin role.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}

// APIKeyAuth returns middleware that validates sk-xxx API keys.
// Sets user_id and role in context for downstream handlers.
func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer sk-") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "missing or invalid API key, use: Authorization: Bearer sk-xxx",
					"type":    "auth_error",
				},
			})
			return
		}

		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		keyHash := hashKey(apiKey)

		type result struct {
			UserID      string `gorm:"column:user_id"`
			UserRole    string `gorm:"column:user_role"`
			IsActive    bool   `gorm:"column:is_active"`
			UserActive  bool   `gorm:"column:user_active"`
		}

		var row result
		err := db.Table("api_keys").
			Select("api_keys.user_id, users.role AS user_role, api_keys.is_active, users.is_active AS user_active").
			Joins("JOIN users ON users.id = api_keys.user_id").
			Where("api_keys.key_hash = ?", keyHash).
			First(&row).Error

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "invalid API key",
					"type":    "auth_error",
				},
			})
			return
		}

		if !row.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "API key is disabled",
					"type":    "auth_error",
				},
			})
			return
		}

		if !row.UserActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "account is disabled",
					"type":    "auth_error",
				},
			})
			return
		}

		// Set user info in context
		c.Set("user_id", row.UserID)
		c.Set("role", row.UserRole)
		c.Set("auth_method", "api_key")
		c.Set("api_key_hash", keyHash)
		c.Next()
	}
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
