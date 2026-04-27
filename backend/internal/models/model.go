package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a platform user
type User struct {
	ID            uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Email         string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash  string    `gorm:"type:varchar(255);not null"`
	Username      *string   `gorm:"type:varchar(100);uniqueIndex"`
	AvatarURL     *string   `gorm:"type:text"`
	Role          string    `gorm:"type:varchar(50);not null;default:'user';check:role IN ('guest','user','vip','admin')"`
	Balance       float64   `gorm:"type:decimal(20,8);not null;default:0"`
	TotalSpent    float64   `gorm:"type:decimal(20,8);not null;default:0"`
	RequestCount  int       `gorm:"not null;default:0"`
	IsActive      bool      `gorm:"not null;default:true"`
	// 会员等级
	MembershipTier      string     `gorm:"type:varchar(20);not null;default:'free'"`
	MembershipExpiresAt *time.Time `gorm:"type:timestamp with time zone"`
	MembershipStartedAt *time.Time `gorm:"type:timestamp with time zone"`
	EmailVerified bool      `gorm:"not null;default:false"`
	LastLoginAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`

	// Relationships
	APIKeys        []APIKey        `gorm:"foreignKey:UserID"`
	Requests       []Request       `gorm:"foreignKey:UserID"`
	BillingRecords []BillingRecord `gorm:"foreignKey:UserID"`
	RechargeOrders []RechargeOrder `gorm:"foreignKey:UserID"`
	Subscriptions  []Subscription  `gorm:"foreignKey:UserID"`
}

// APIKey represents user's API key
type APIKey struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	Name       string     `gorm:"type:varchar(100);not null"`
	KeyHash    string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	Prefix     string     `gorm:"type:varchar(10);not null"`
	LastUsedAt *time.Time
	TotalUsed  int       `gorm:"not null;default:0"`
	RPMLimit   *int
	TPMLimit   *int
	IsActive   bool      `gorm:"not null;default:true"`
	ExpiresAt  *time.Time
	// 项目级管理（B 端能力）
	ProjectName       *string    `gorm:"type:varchar(100)"`
	MonthlyBudget     *float64   `gorm:"type:decimal(20,8)"`
	BudgetAlertPct    int        `gorm:"not null;default:80"`
	BudgetUsed        float64    `gorm:"type:decimal(20,8);not null;default:0"`
	BudgetPeriodStart *time.Time `gorm:"type:timestamp with time zone"`
	BudgetAlerted     bool       `gorm:"not null;default:false"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	// Relationships
	User             User                  `gorm:"foreignKey:UserID"`
	AllowedModels    []APIKeyAllowedModel  `gorm:"foreignKey:APIKeyID"`
	Requests         []Request             `gorm:"foreignKey:APIKeyID"`
}

// APIKeyAllowedModel represents many-to-many relationship between API keys and allowed models
type APIKeyAllowedModel struct {
	APIKeyID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ModelID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time

	// Relationships
	APIKey APIKey `gorm:"foreignKey:APIKeyID"`
	Model  Model  `gorm:"foreignKey:ModelID"`
}

// UpstreamChannel represents upstream provider API key
type UpstreamChannel struct {
	ID               uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Name             string    `gorm:"type:varchar(100);not null"`
	Provider         string    `gorm:"type:varchar(50);not null;index;check:provider IN ('openai','anthropic','google','qwen','deepseek')"`
	APIKeyEncrypted  string    `gorm:"type:text;not null"`
	BaseURL          *string   `gorm:"type:text"`
	Weight           int       `gorm:"not null;default:1"`
	MaxConcurrent    int       `gorm:"not null;default:10"`
	IsEnabled        bool      `gorm:"not null;default:true"`
	LastHealthCheck  *time.Time
	HealthStatus     string    `gorm:"type:varchar(20);not null;default:'unknown';check:health_status IN ('unknown','healthy','unhealthy')"`
	TotalRequests    int       `gorm:"not null;default:0"`
	TotalTokens      int       `gorm:"not null;default:0"`
	ErrorCount       int       `gorm:"not null;default:0"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`

	// Relationships
	Requests []Request `gorm:"foreignKey:UpstreamChannelID"`
}

// Model represents AI model with pricing
type Model struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	Name         string    `gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName  string    `gorm:"type:varchar(100);not null"`
	Provider     string    `gorm:"type:varchar(50);not null;check:provider IN ('openai','anthropic','google','qwen','deepseek')"`
	ContextLength int      `gorm:"not null;default:4096"`
	InputPrice   float64   `gorm:"type:decimal(20,8);not null"`
	OutputPrice  float64   `gorm:"type:decimal(20,8);not null"`
	Multiplier   float64   `gorm:"type:decimal(5,2);not null;default:1.0"`
	IsEnabled    bool      `gorm:"not null;default:true"`
	IsPublic     bool      `gorm:"not null;default:true"`
	Description  *string   `gorm:"type:text"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// Relationships
	Requests            []Request            `gorm:"foreignKey:ModelID"`
	APIKeyAllowedModels []APIKeyAllowedModel `gorm:"foreignKey:ModelID"`
}

// Request represents API request log
type Request struct {
	ID                uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index"`
	APIKeyID         *uuid.UUID `gorm:"type:uuid;index"`
	ModelID          uuid.UUID `gorm:"type:uuid;not null;index"`
	UpstreamChannelID *uuid.UUID `gorm:"type:uuid;index"`
	RequestID        *string   `gorm:"type:varchar(100)"`
	Path             string    `gorm:"type:varchar(255);not null"`
	Method           string    `gorm:"type:varchar(10);not null"`
	StatusCode       int       `gorm:"not null"`
	PromptTokens     int       `gorm:"not null;default:0"`
	CompletionTokens int       `gorm:"not null;default:0"`
	TotalTokens      int       `gorm:"not null;default:0"`
	Cost             float64   `gorm:"type:decimal(20,8);not null;default:0"`
	DurationMs       int       `gorm:"not null"`
	IPAddress        *string   `gorm:"type:inet"`
	UserAgent        *string   `gorm:"type:text"`
	RequestBody      *[]byte   `gorm:"type:jsonb"`
	ResponseBody     *[]byte   `gorm:"type:jsonb"`
	ErrorMessage     *string   `gorm:"type:text"`
	CreatedAt        time.Time `gorm:"index"`

	// Relationships
	User             User             `gorm:"foreignKey:UserID"`
	APIKey           *APIKey          `gorm:"foreignKey:APIKeyID"`
	Model            Model            `gorm:"foreignKey:ModelID"`
	UpstreamChannel  *UpstreamChannel `gorm:"foreignKey:UpstreamChannelID"`
	BillingRecord    *BillingRecord   `gorm:"foreignKey:RequestID"`
}

// BillingRecord represents billing transaction
type BillingRecord struct {
	ID            uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	RequestID    *uuid.UUID `gorm:"type:uuid;index"`
	Type         string    `gorm:"type:varchar(50);not null;check:type IN ('chat_completion','recharge','adjustment','refund')"`
	Amount       float64   `gorm:"type:decimal(20,8);not null"`
	BalanceBefore float64   `gorm:"type:decimal(20,8);not null"`
	BalanceAfter  float64   `gorm:"type:decimal(20,8);not null"`
	Description  *string   `gorm:"type:text"`
	Metadata     *[]byte   `gorm:"type:jsonb"`
	CreatedAt    time.Time `gorm:"index"`

	// Relationships
	User    User     `gorm:"foreignKey:UserID"`
	Request *Request `gorm:"foreignKey:RequestID"`
}

// RechargeOrder represents recharge transaction
type RechargeOrder struct {
	ID            uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	OrderNo      string    `gorm:"type:varchar(100);uniqueIndex;not null"`
	Amount       float64   `gorm:"type:decimal(20,8);not null"`
	PaymentMethod string    `gorm:"type:varchar(50);not null;check:payment_method IN ('stripe','alipay','wechat','usdt')"`
	PaymentStatus string    `gorm:"type:varchar(50);not null;default:'pending';check:payment_status IN ('pending','processing','paid','failed','refunded')"`
	PaymentID    *string   `gorm:"type:varchar(255)"`
	PaidAt       *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// Subscription represents user subscription plan
type Subscription struct {
	ID                  uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID             uuid.UUID `gorm:"type:uuid;not null;index"`
	PlanName           string    `gorm:"type:varchar(100);not null"`
	PlanType           string    `gorm:"type:varchar(50);not null;check:plan_type IN ('monthly','quarterly','yearly')"`
	Amount             float64   `gorm:"type:decimal(20,8);not null"`
	TokenQuota         *int
	StartDate          time.Time
	EndDate            time.Time
	IsActive           bool      `gorm:"not null;default:true"`
	AutoRenew          bool      `gorm:"not null;default:false"`
	StripeSubscriptionID *string   `gorm:"type:varchar(255)"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          gorm.DeletedAt `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// AuditLog represents admin audit log
type AuditLog struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primary_key"`
	UserID      *uuid.UUID `gorm:"type:uuid;index"`
	Action       string    `gorm:"type:varchar(100);not null"`
	ResourceType *string   `gorm:"type:varchar(50)"`
	ResourceID   *string   `gorm:"type:varchar(100)"`
	OldValues    *[]byte   `gorm:"type:jsonb"`
	NewValues    *[]byte   `gorm:"type:jsonb"`
	IPAddress    *string   `gorm:"type:inet"`
	UserAgent    *string   `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"index"`

	// Relationships
	User *User `gorm:"foreignKey:UserID"`
}