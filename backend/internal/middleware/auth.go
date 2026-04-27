package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/membership"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

		// === 滑动续期：剩余 < 7 天时自动签发新 token ===
		if claims.ExpiresAt != nil {
			remaining := time.Until(claims.ExpiresAt.Time)
			if remaining < 7*24*time.Hour && remaining > 0 {
				newClaims := &Claims{
					UserID: claims.UserID,
					Email:  claims.Email,
					Role:   claims.Role,
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now()),
						ID:        uuid.New().String(),
					},
				}
				newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
				if signed, err := newToken.SignedString([]byte(secret)); err == nil {
					c.Header("X-Refresh-Token", signed)
					c.Header("Access-Control-Expose-Headers", "X-Refresh-Token")
				}
			}
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
			UserID          string  `gorm:"column:user_id"`
			UserRole        string  `gorm:"column:user_role"`
			IsActive        bool    `gorm:"column:is_active"`
			UserActive      bool    `gorm:"column:user_active"`
			APIKeyID        string  `gorm:"column:api_key_id"`
			RPMLimit        *int64  `gorm:"column:rpm_limit"`
			TPMLimit        *int64  `gorm:"column:tpm_limit"`
			BlacklistReason     *string    `gorm:"column:blacklist_reason"`
			MembershipTier      string     `gorm:"column:membership_tier"`
			MembershipExpiresAt *time.Time `gorm:"column:membership_expires_at"`
		}

		var row result
		err := db.Table("api_keys").
			Select("api_keys.user_id, users.role AS user_role, api_keys.is_active, users.is_active AS user_active, api_keys.id AS api_key_id, api_keys.rpm_limit, api_keys.tpm_limit, users.blacklist_reason, users.membership_tier, users.membership_expires_at").
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
			msg := "account is disabled"
			if row.BlacklistReason != nil && *row.BlacklistReason != "" {
				msg = "account suspended: " + *row.BlacklistReason
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": msg,
					"type":    "account_suspended",
				},
			})
			return
		}

		// 计算会员等级有效限制：取 API Key 自定义和会员等级的较小值
		effectiveTier := membership.EffectiveTier(membership.Tier(row.MembershipTier), row.MembershipExpiresAt)
		tierLimits := membership.TierLimits[effectiveTier]
		finalRPM := row.RPMLimit
		finalTPM := row.TPMLimit
		if tierLimits.RPM > 0 {
			rpmCap := int64(tierLimits.RPM)
			if finalRPM == nil || *finalRPM == 0 || *finalRPM > rpmCap {
				finalRPM = &rpmCap
			}
		}
		if tierLimits.TPM > 0 {
			tpmCap := int64(tierLimits.TPM)
			if finalTPM == nil || *finalTPM == 0 || *finalTPM > tpmCap {
				finalTPM = &tpmCap
			}
		}

		// Set user info in context
		c.Set("user_id", row.UserID)
		c.Set("role", row.UserRole)
		c.Set("auth_method", "api_key")
		c.Set("api_key_hash", keyHash)
		c.Set("api_key_id", row.APIKeyID)
		c.Set("api_key_rpm", finalRPM)
		c.Set("api_key_tpm", finalTPM)
		c.Set("membership_tier", string(effectiveTier))
		c.Next()
	}
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
