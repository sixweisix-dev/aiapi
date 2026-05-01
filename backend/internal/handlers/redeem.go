package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RedeemHandler struct{ db *gorm.DB }

func NewRedeemHandler(db *gorm.DB) *RedeemHandler { return &RedeemHandler{db: db} }

// RedeemCode 用户兑换
func (h *RedeemHandler) RedeemCode(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入兑换码"})
		return
	}
	code := strings.TrimSpace(strings.ToUpper(req.Code))

	var rc struct {
		ID             string
		Type           string
		BalanceAmount  float64
		FaceValue      float64
		MembershipTier string
		MembershipDays int
		Status         string
		ExpiresAt      *time.Time
	}
	if err := h.db.Raw(`SELECT id,type,balance_amount,face_value,membership_tier,membership_days,status,expires_at FROM redeem_codes WHERE code=?`, code).Scan(&rc).Error; err != nil || rc.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "兑换码无效"})
		return
	}
	if rc.Status != "unused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "兑换码已被使用"})
		return
	}
	if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "兑换码已过期"})
		return
	}

	// 检查活动开关
	promoEnabled := GetSettingValue(h.db, "promo_enabled", "true") == "true"

	// 计算实际到账金额：活动关闭则用面值
	actualAmount := rc.BalanceAmount
	if !promoEnabled && rc.FaceValue > 0 {
		actualAmount = rc.FaceValue
	}

	// 事务：标记已用 + 到账
	err := h.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Exec(`UPDATE redeem_codes SET status='used',redeemed_by=?,redeemed_at=? WHERE id=? AND status='unused'`,
			userID, now, rc.ID).Error; err != nil {
			return err
		}
		if rc.Type == "balance" || actualAmount > 0 {
			if err := tx.Exec(`UPDATE users SET balance=balance+? WHERE id=?`, actualAmount, userID).Error; err != nil {
				return err
			}
			// 首充标记
			tx.Exec(`UPDATE users SET first_recharge_at=? WHERE id=? AND first_recharge_at IS NULL`, now, userID)
		}
		if rc.Type == "membership" && rc.MembershipTier != "" && rc.MembershipTier != "free" {
			var expiresAt time.Time
			var curExpires *time.Time
			h.db.Raw(`SELECT membership_expires_at FROM users WHERE id=?`, userID).Scan(&curExpires)
			if curExpires != nil && curExpires.After(now) {
				expiresAt = curExpires.Add(time.Duration(rc.MembershipDays) * 24 * time.Hour)
			} else {
				expiresAt = now.Add(time.Duration(rc.MembershipDays) * 24 * time.Hour)
			}
			if err := tx.Exec(`UPDATE users SET membership_tier=?,membership_expires_at=?,membership_started_at=COALESCE(membership_started_at,?) WHERE id=?`,
				rc.MembershipTier, expiresAt, now, userID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "兑换失败，请重试"})
		return
	}

	msg := fmt.Sprintf("兑换成功！")
	if actualAmount > 0 && rc.Type == "balance" {
		msg = fmt.Sprintf("兑换成功！余额 +¥%.2f", actualAmount)
	} else if rc.MembershipTier != "free" {
		msg = fmt.Sprintf("兑换成功！已开通 %s %d 天，余额 +¥%.2f", rc.MembershipTier, rc.MembershipDays, actualAmount)
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// AdminGenerateCodes 管理员批量生成兑换码
func (h *RedeemHandler) AdminGenerateCodes(c *gin.Context) {
	var req struct {
		Count          int     `json:"count" binding:"required,min=1,max=1000"`
		Type           string  `json:"type" binding:"required"`
		BalanceAmount  float64 `json:"balance_amount"`
		FaceValue      float64 `json:"face_value"`
		MembershipTier string  `json:"membership_tier"`
		MembershipDays int     `json:"membership_days"`
		ExpiryDays     int     `json:"expiry_days"`
		Note           string  `json:"note"`
		AutoCalc       bool    `json:"auto_calc"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// auto_calc=true 时按阶梯规则计算 balance_amount
	if req.AutoCalc && req.Type == "balance" && req.FaceValue > 0 {
		tiersJSON := GetSettingValue(h.db, "recharge_tiers", "[]")
		var tiers []struct {
			Min   float64 `json:"min"`
			Bonus float64 `json:"bonus"`
		}
		if err := json.Unmarshal([]byte(tiersJSON), &tiers); err == nil {
			var bonus float64
			for _, t := range tiers {
				if req.FaceValue >= t.Min {
					bonus = t.Bonus
				}
			}
			req.BalanceAmount = req.FaceValue + bonus
		}
	}
	if req.FaceValue == 0 {
		req.FaceValue = req.BalanceAmount
	}

	batchID := fmt.Sprintf("batch_%d", time.Now().Unix())
	var codes []string
	for i := 0; i < req.Count; i++ {
		b := make([]byte, 8)
		rand.Read(b)
		code := strings.ToUpper(hex.EncodeToString(b))
		// 格式: XXXX-XXXX-XXXX-XXXX
		formatted := code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
		codes = append(codes, formatted)

		var expiresAt *time.Time
		if req.ExpiryDays > 0 {
			t := time.Now().Add(time.Duration(req.ExpiryDays) * 24 * time.Hour)
			expiresAt = &t
		}
		mtier := req.MembershipTier
		if mtier == "" {
			mtier = "free"
		}
		h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,face_value,membership_tier,membership_days,batch_id,note,expires_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			formatted, req.Type, req.BalanceAmount, req.FaceValue, mtier, req.MembershipDays, batchID, req.Note, expiresAt)
	}

	c.JSON(http.StatusOK, gin.H{
		"batch_id": batchID,
		"count":    len(codes),
		"codes":    codes,
	})
}

// AdminListCodes 管理员查看兑换码
func (h *RedeemHandler) AdminListCodes(c *gin.Context) {
	batchID := c.Query("batch_id")
	status := c.Query("status")

	query := `SELECT id,code,type,balance_amount,membership_tier,membership_days,status,redeemed_at,expires_at,batch_id,note FROM redeem_codes WHERE 1=1`
	var args []interface{}
	if batchID != "" {
		query += " AND batch_id=?"
		args = append(args, batchID)
	}
	if status != "" {
		query += " AND status=?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 500"

	var rows []map[string]interface{}
	h.db.Raw(query, args...).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"codes": rows})
}

// PreviewCode 预览兑换码权益（不兑换）
func (h *RedeemHandler) PreviewCode(c *gin.Context) {
	userID := c.GetString("user_id")
	code := strings.TrimSpace(strings.ToUpper(c.Query("code")))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入兑换码"})
		return
	}

	var rc struct {
		ID             string
		Type           string
		BalanceAmount  float64
		FaceValue      float64
		MembershipTier string
		MembershipDays int
		Status         string
		ExpiresAt      *time.Time
	}
	if err := h.db.Raw(`SELECT id,type,balance_amount,face_value,membership_tier,membership_days,status,expires_at FROM redeem_codes WHERE code=?`, code).Scan(&rc).Error; err != nil || rc.ID == "" {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": "兑换码无效"})
		return
	}
	if rc.Status != "unused" {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": "兑换码已被使用"})
		return
	}
	if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": "兑换码已过期"})
		return
	}

	// 判断是否首充
	var firstRechargeCount int64
	h.db.Raw(`SELECT COUNT(*) FROM users WHERE id=? AND first_recharge_at IS NOT NULL`, userID).Scan(&firstRechargeCount)
	isFirst := firstRechargeCount == 0

	// 读取首充礼金额
	var firstBonus float64
	if isFirst && rc.Type == "balance" {
		var val string
		h.db.Raw(`SELECT value FROM settings WHERE key='first_recharge_bonus'`).Scan(&val)
		fmt.Sscanf(val, "%f", &firstBonus)
	}

	promoEnabled := GetSettingValue(h.db, "promo_enabled", "true") == "true"
	actualAmount := rc.BalanceAmount
	if !promoEnabled && rc.FaceValue > 0 {
		actualAmount = rc.FaceValue
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":             true,
		"type":              rc.Type,
		"balance_amount":    actualAmount,
		"original_amount":   rc.BalanceAmount,
		"promo_enabled":     promoEnabled,
		"membership_tier":   rc.MembershipTier,
		"membership_days":   rc.MembershipDays,
		"is_first_recharge": isFirst,
		"first_bonus":       firstBonus,
	})
}
