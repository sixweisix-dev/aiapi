package membership

import "time"

// Tier 会员等级
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// Limits 各等级的资源限制
type Limits struct {
	RPM            int
	TPM            int
	MaxAPIKeys     int
	BudgetAlerts   bool
	CSVExport      bool
	InvoiceSupport bool
	SLAGuarantee   float64
	DisplayName    string
}

// TierLimits 各等级的限制配置
var TierLimits = map[Tier]Limits{
	TierFree: {
		RPM:            6,
		TPM:            10000,
		MaxAPIKeys:     1,
		BudgetAlerts:   false,
		CSVExport:      true,
		InvoiceSupport: false,
		DisplayName:    "免费版",
	},
	TierPro: {
		RPM:            60,
		TPM:            100000,
		MaxAPIKeys:     5,
		BudgetAlerts:   true,
		CSVExport:      true,
		InvoiceSupport: true,
		DisplayName:    "专业版",
	},
	TierEnterprise: {
		RPM:            600,
		TPM:            1000000,
		MaxAPIKeys:     0,
		BudgetAlerts:   true,
		CSVExport:      true,
		InvoiceSupport: true,
		SLAGuarantee:   0.995,
		DisplayName:    "企业版",
	},
}

// RechargeUpgrade 充值金额对应的会员升级
type RechargeUpgrade struct {
	Tier         Tier
	BonusAmount  float64
	DurationDays int
}

// RechargeRules 充值金额 → 升级规则
var RechargeRules = map[float64]RechargeUpgrade{
	99: {
		Tier:         TierPro,
		BonusAmount:  120,
		DurationDays: 30,
	},
	499: {
		Tier:         TierEnterprise,
		BonusAmount:  600,
		DurationDays: 30,
	},
}

// CalculateBonus 根据充值金额计算实际到账金额 + 是否升级
func CalculateBonus(amount float64) (actualAmount float64, upgradeTier Tier, durationDays int) {
	if rule, ok := RechargeRules[amount]; ok {
		return rule.BonusAmount, rule.Tier, rule.DurationDays
	}
	return amount, "", 0
}

// IsActive 检查会员是否有效（未过期）
func IsActive(tier Tier, expiresAt *time.Time) bool {
	if tier == TierFree {
		return true
	}
	if expiresAt == nil {
		return false
	}
	return expiresAt.After(time.Now())
}

// EffectiveTier 返回当前有效等级（已过期的 pro/enterprise 视为 free）
func EffectiveTier(tier Tier, expiresAt *time.Time) Tier {
	if IsActive(tier, expiresAt) {
		return tier
	}
	return TierFree
}
