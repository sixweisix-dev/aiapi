package handlers

import (
	"crypto/rand"
	"encoding/hex"
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
		ID            string
		Type          string
		BalanceAmount float64
		MembershipTier string
		MembershipDays int
		Status        string
		ExpiresAt     *time.Time
	}
	if err := h.db.Raw(`SELECT id,type,balance_amount,membership_tier,membership_days,status,expires_at FROM redeem_codes WHERE code=?`, code).Scan(&rc).Error; err != nil || rc.ID == "" {
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

	// 事务：标记已用 + 到账
	err := h.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Exec(`UPDATE redeem_codes SET status='used',redeemed_by=?,redeemed_at=? WHERE id=? AND status='unused'`,
			userID, now, rc.ID).Error; err != nil {
			return err
		}
		if rc.Type == "balance" || rc.BalanceAmount > 0 {
			if err := tx.Exec(`UPDATE users SET balance=balance+? WHERE id=?`, rc.BalanceAmount, userID).Error; err != nil {
				return err
			}
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
	if rc.BalanceAmount > 0 {
		msg = fmt.Sprintf("兑换成功！余额 +¥%.2f", rc.BalanceAmount)
	} else if rc.MembershipTier != "free" {
		msg = fmt.Sprintf("兑换成功！已开通 %s %d 天", rc.MembershipTier, rc.MembershipDays)
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// AdminGenerateCodes 管理员批量生成兑换码
func (h *RedeemHandler) AdminGenerateCodes(c *gin.Context) {
	var req struct {
		Count          int     `json:"count" binding:"required,min=1,max=1000"`
		Type           string  `json:"type" binding:"required"`
		BalanceAmount  float64 `json:"balance_amount"`
		MembershipTier string  `json:"membership_tier"`
		MembershipDays int     `json:"membership_days"`
		ExpiryDays     int     `json:"expiry_days"`
		Note           string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
		h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at) VALUES(?,?,?,?,?,?,?,?)`,
			formatted, req.Type, req.BalanceAmount, mtier, req.MembershipDays, batchID, req.Note, expiresAt)
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
