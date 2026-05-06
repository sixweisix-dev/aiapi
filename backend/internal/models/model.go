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
	FirstRechargeAt *time.Time `gorm:"index" json:"first_recharge_at,omitempty"`
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

	// === 额度管理 (Phase 1 新增) ===
	QuotaType           string     `gorm:"type:varchar(20);not null;default:'unlimited'"` // unlimited/daily/fixed
	DailyQuotaUSD       float64    `gorm:"type:decimal(10,4);not null;default:0"`        // 每日额度 USD (daily 模式)
	QuotaUsedTodayUSD   float64    `gorm:"type:decimal(10,4);not null;default:0"`        // 今日已用 USD
	TotalQuotaUSD       float64    `gorm:"type:decimal(12,4);not null;default:0"`        // 固定总额 USD (fixed 模式)
	UsedTotalUSD        float64    `gorm:"type:decimal(12,4);not null;default:0"`        // 累计已用 USD (永不重置)
	QuotaResetAt        *time.Time `gorm:""`                                              // 上次重置时间
	SubscriptionStart   *time.Time `gorm:""`                                              // 订阅开始
	SubscriptionEnd     *time.Time `gorm:""`                                              // 订阅结束
	QuotaStatus         string     `gorm:"type:varchar(20);not null;default:'normal'"`   // normal/warning/critical/exhausted
	ErrorStreak         int        `gorm:"not null;default:0"`                            // 连续失败次数(达阈值自动禁用)

	// === 成本统计 ===
	DailyCostCNY        float64    `gorm:"type:decimal(20,8);not null;default:0"`        // 今日成本(CNY)
	MonthlyCostCNY      float64    `gorm:"type:decimal(20,8);not null;default:0"`        // 本月成本(CNY)

	// === 缓存命中率 ===
	CacheHitTokens      int64      `gorm:"not null;default:0"`                            // 累计 cache_read tokens
	CacheTotalTokens    int64      `gorm:"not null;default:0"`                            // 累计 input+cache_read tokens

	// === 延迟统计 (近 1 小时滑窗, 由 cron 计算) ===
	AvgLatencyMs        int        `gorm:"not null;default:0"`
	P95LatencyMs        int        `gorm:"not null;default:0"`

	// === 专属渠道 ===
	IsDedicated         bool       `gorm:"not null;default:false"`                        // 是否专属渠道
	DedicatedUserIDs    string     `gorm:"type:text;not null;default:''"`                // 逗号分隔 UUID
	DedicatedUserIDsAuto string    `gorm:"type:text;not null;default:''"`                // 自动隔离名单(每日0点重置)

	// === 对账倍率（让 widget 余额跟上游后台对齐用）===
	// 默认 1.0；用法：跑一段时间后对比上游真实消耗 vs 我方 quota_used_today_usd
	// reconcile_multiplier = quota_used_today_usd / 上游真实消耗
	// widget 余额 = daily_quota − quota_used_today / reconcile_multiplier
	ReconcileMultiplier float64 `gorm:"type:decimal(5,4);not null;default:1;column:reconcile_multiplier"`

	// === 计费模式 (区分按量付费 vs 包月套餐) ===
	// "pay_as_you_go" (默认): 按 token 计费, 成本 = revenue / reconcile_multiplier
	// "subscription": 包月套餐, 成本固定为月费 / 30 (天)
	BillingMode   string  `gorm:"type:varchar(20);not null;default:'pay_as_you_go'"`
	MonthlyFeeCNY float64 `gorm:"type:decimal(10,2);not null;default:0"`

	// === 启用 1 小时 prompt cache beta header ===
	// true: 注入 anthropic-beta: prompt-caching-1h-2025-04-09 (TTL 5min → 60min)
	// 仅对真支持 cache 的上游 (如 Anthropic 直连) 有效, 反代池开了无用
	EnableCache1hBeta bool `gorm:"not null;default:false;column:enable_cache_1h_beta"`

	// === 自动注入 cache_control 到 system block ===
	// true: 网关层解析 request body, 给长 system 自动加 cache_control
	// 配合 EnableCache1hBeta 决定 ttl (true→1h, false→5m)
	AutoInjectCache bool `gorm:"not null;default:false"`

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
	BonusAmount    float64 `gorm:"type:decimal(20,8);not null;default:0" json:"bonus_amount"`
	Intent         string  `gorm:"type:varchar(50);not null;default:'balance'" json:"intent"`
	UpgradesToTier *string `gorm:"type:varchar(50)" json:"upgrades_to_tier,omitempty"`

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

// Setting represents a key-value system setting
type Setting struct {
	Key       string    `gorm:"type:varchar(100);primary_key" json:"key"`
	Value     string    `gorm:"type:text;not null;default:''" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

