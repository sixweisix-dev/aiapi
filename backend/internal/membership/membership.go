package membership

import (
	"encoding/json"
	"time"
)

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

// RechargeTier 阶梯赠送规则一项
type RechargeTier struct {
	Min   float64 `json:"min"`
	Bonus float64 `json:"bonus"`
}

// CalculateBonus 根据充值金额计算实际到账金额 + 是否升级
// tiersJSON: 来自 settings 表的阶梯规则 JSON 字符串
// firstRechargeBonus: 首充额外赠送金额(0 表示无)
// isFirstRecharge: 当前用户是否首次充值
//
// 优先级:
// 1. 命中会员套餐(99/499) → 走会员逻辑, 不叠加阶梯/首充
// 2. 否则按阶梯找最高匹配档位 + 首充叠加
func CalculateBonus(amount float64, tiersJSON string, firstRechargeBonus float64, isFirstRecharge bool) (actualAmount float64, upgradeTier Tier, durationDays int, tierBonus float64, firstBonus float64) {
	// 优先: 会员套餐
	if rule, ok := RechargeRules[amount]; ok {
		return rule.BonusAmount, rule.Tier, rule.DurationDays, 0, 0
	}

	// 阶梯赠送: 找充值金额满足的最高档
	tiers := parseTiers(tiersJSON)
	for _, t := range tiers {
		if amount >= t.Min && t.Bonus > tierBonus {
			tierBonus = t.Bonus
		}
	}

	// 首充叠加
	if isFirstRecharge && firstRechargeBonus > 0 {
		firstBonus = firstRechargeBonus
	}

	actualAmount = amount + tierBonus + firstBonus
	return
}

// ParseTiers 解析阶梯 JSON, 失败返回空(无赠送)
func parseTiers(raw string) []RechargeTier {
	var out []RechargeTier
	if raw == "" {
		return out
	}
	_ = jsonUnmarshal([]byte(raw), &out)
	return out
}

// 内联 json 解析(避免在该 package 引入 encoding/json 顶层 import 链)
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
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
