package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-api-gateway/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)

type AuthHandler struct {
	db        *gorm.DB
	jwtSecret string
	rdb       *redis.Client
	mailCfg   MailConfig
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Username string `json:"username" binding:"omitempty,min=2,max=50"`
	CaptchaID  string `json:"captcha_id"`
	CaptchaAns string `json:"captcha_answer"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string    `json:"token"`
	User  UserBrief `json:"user"`
}

type UserBrief struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Balance float64 `json:"balance"`
}

func NewAuthHandler(db *gorm.DB, jwtSecret string, rdb *redis.Client, mailCfg MailConfig) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, rdb: rdb, mailCfg: mailCfg}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify captcha
	if req.CaptchaID != "" && !VerifyCaptcha(req.CaptchaID, req.CaptchaAns) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}
	// Check if email already exists
	var count int64
	h.db.Model(&UserModel{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	// Hash password
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Register hash error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = strings.Split(req.Email, "@")[0]
	}

	user := UserModel{
		Email:         req.Email,
		PasswordHash:  string(hashedBytes),
		Username:      &username,
		Role:          "user",
		Balance:       0,
		IsActive:      true,
		EmailVerified: false,
	}

	if err := h.db.Create(&user).Error; err != nil {
		log.Printf("Register create error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	token, err := h.generateJWT(user.ID.String(), user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		Token: token,
		User: UserBrief{
			ID:      user.ID.String(),
			Email:   user.Email,
			Role:    user.Role,
			Balance: user.Balance,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user UserModel
	result := h.db.Where("email = ?", req.Email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is disabled"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Update last login
	now := time.Now()
	h.db.Model(&user).Update("last_login_at", &now)

	token, err := h.generateJWT(user.ID.String(), user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User: UserBrief{
			ID:      user.ID.String(),
			Email:   user.Email,
			Role:    user.Role,
			Balance: user.Balance,
		},
	})
}

// Me returns current user info (requires JWT middleware).
func (h *AuthHandler) Me(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	jwtClaims := claims.(*middleware.Claims)
	userID := jwtClaims.UserID

	var user UserModel
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID.String(),
		"email":          user.Email,
		"username":       user.Username,
		"role":           user.Role,
		"balance":        user.Balance,
		"total_spent":    user.TotalSpent,
		"request_count":  user.RequestCount,
		"is_active":      user.IsActive,
		"email_verified": user.EmailVerified,
		"created_at":     user.CreatedAt,
	})
}

func (h *AuthHandler) generateJWT(userID, email, role string) (string, error) {
	claims := &middleware.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// UserModel is a minimal copy used in auth — matches the actual models.User but avoids import cycle.
type UserModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Email         string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash  string     `gorm:"type:varchar(255);not null"`
	Username      *string    `gorm:"type:varchar(100);uniqueIndex"`
	AvatarURL     *string    `gorm:"type:text"`
	Role          string     `gorm:"type:varchar(50);not null;default:'user'"`
	Balance       float64    `gorm:"type:decimal(20,8);not null;default:0"`
	TotalSpent    float64    `gorm:"type:decimal(20,8);not null;default:0"`
	RequestCount  int        `gorm:"not null;default:0"`
	IsActive      bool       `gorm:"not null;default:true"`
	EmailVerified bool       `gorm:"not null;default:false"`
	LastLoginAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserModel) TableName() string {
	return "users"
}

// ---- Change Password ----

type ChangePasswordRequest struct {
        OldPassword string `json:"old_password" binding:"required"`
        NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
        userIDRaw, exists := c.Get("user_id")
        if !exists {
                c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
                return
        }
        userID := userIDRaw.(string)

        var req ChangePasswordRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        var user UserModel
        if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
                c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
                return
        }

        if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
                c.JSON(http.StatusUnauthorized, gin.H{"error": "old password is incorrect"})
                return
        }

        // Password strength: min 8 chars, upper, lower, digit
        if len(req.NewPassword) < 8 {
                c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
                return
        }
        hasUpper, hasLower, hasDigit := false, false, false
        for _, ch := range req.NewPassword {
                switch {
                case ch >= 'A' && ch <= 'Z': hasUpper = true
                case ch >= 'a' && ch <= 'z': hasLower = true
                case ch >= '0' && ch <= '9': hasDigit = true
                }
        }
        if !hasUpper || !hasLower || !hasDigit {
                c.JSON(http.StatusBadRequest, gin.H{"error": "password must contain uppercase, lowercase and digit"})
                return
        }

        hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
                return
        }

        if err := h.db.Model(&user).Update("password_hash", string(hashedBytes)).Error; err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"message": "password updated successfully"})
}

// ---- Forgot Password ----

type ForgotPasswordRequest struct {
        Email      string `json:"email" binding:"required,email"`
        CaptchaID  string `json:"captcha_id" binding:"required"`
        CaptchaAns string `json:"captcha_answer" binding:"required"`
}

type ResetPasswordRequest struct {
        Token       string `json:"token" binding:"required"`
        NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
        var req ForgotPasswordRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        if !VerifyCaptcha(req.CaptchaID, req.CaptchaAns) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
                return
        }

        var user UserModel
        if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
                // 不暴露用户是否存在
                c.JSON(http.StatusOK, gin.H{"message": "如果邮箱存在，重置链接已发送"})
                return
        }

        token := uuid.New().String()
        if h.rdb != nil {
                StoreEmailToken(h.rdb, req.Email, token)
        }

        resetURL := "https://transitai.cloud/reset-password?token=" + token
        go func() {
                if err := SendResetEmail(h.mailCfg, req.Email, resetURL); err != nil {
                        log.Printf("Failed to send reset email to %s: %v", req.Email, err)
                } else {
                        log.Printf("Reset email sent to %s", req.Email)
                }
        }()

        c.JSON(http.StatusOK, gin.H{"message": "如果邮箱存在，重置链接已发送"})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
        var req ResetPasswordRequest
        if err := c.ShouldBindJSON(&req); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
                return
        }

        if h.rdb == nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "service unavailable"})
                return
        }

        email, err := GetEmailByToken(h.rdb, req.Token)
        if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "重置链接无效或已过期"})
                return
        }

        hasUpper, hasLower, hasDigit := false, false, false
        for _, ch := range req.NewPassword {
                switch {
                case ch >= 'A' && ch <= 'Z': hasUpper = true
                case ch >= 'a' && ch <= 'z': hasLower = true
                case ch >= '0' && ch <= '9': hasDigit = true
                }
        }
        if !hasUpper || !hasLower || !hasDigit {
                c.JSON(http.StatusBadRequest, gin.H{"error": "密码需包含大小写字母和数字"})
                return
        }

        hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
                return
        }

        if err := h.db.Model(&UserModel{}).Where("email = ?", email).Update("password_hash", string(hashedBytes)).Error; err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
                return
        }

        DeleteToken(h.rdb, req.Token)
        c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}
