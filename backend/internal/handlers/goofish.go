package handlers

import (
	"encoding/json"
	"io"
	"log"
	"strconv"
	"time"

	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GoofishHandler struct {
	db *gorm.DB
}

func NewGoofishHandler(db *gorm.DB) *GoofishHandler {
	return &GoofishHandler{db: db}
}

// OrderWebhook 接收闲管家订单推送
func (h *GoofishHandler) OrderWebhook(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(200, gin.H{"result": "fail", "msg": "read body failed"})
		return
	}
	bodyStr := string(bodyBytes)

	appKey := GetSettingValue(h.db, "goofish_app_key", "")
	appSecret := GetSettingValue(h.db, "goofish_app_secret", "")
	if appKey == "" || appSecret == "" {
		log.Printf("[Goofish] webhook 调用但未配置 AppKey/AppSecret")
		c.JSON(200, gin.H{"result": "fail", "msg": "未配置闲管家凭证"})
		return
	}

	providedSign := c.Query("sign")
	timestampStr := c.Query("timestamp")
	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)

	if !utils.GoofishVerifySign(appKey, bodyStr, timestamp, appSecret, providedSign) {
		log.Printf("[Goofish] 签名校验失败 ts=%d sign=%s", timestamp, providedSign)
		c.JSON(200, gin.H{"result": "fail", "msg": "签名校验失败"})
		return
	}

	var payload struct {
		SellerID     int64  `json:"seller_id"`
		UserName     string `json:"user_name"`
		OrderNo      string `json:"order_no"`
		OrderType    int    `json:"order_type"`
		OrderStatus  int    `json:"order_status"`
		RefundStatus int    `json:"refund_status"`
		ModifyTime   int64  `json:"modify_time"`
		ProductID    int64  `json:"product_id"`
		ItemID       int64  `json:"item_id"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("[Goofish] 解析 body 失败: %v", err)
		c.JSON(200, gin.H{"result": "fail", "msg": "JSON 解析失败"})
		return
	}
	if payload.OrderNo == "" {
		c.JSON(200, gin.H{"result": "fail", "msg": "order_no 缺失"})
		return
	}

	order := models.GoofishOrder{
		OrderNo: payload.OrderNo, SellerID: payload.SellerID, UserName: payload.UserName,
		OrderType: payload.OrderType, OrderStatus: payload.OrderStatus, RefundStatus: payload.RefundStatus,
		ProductID: payload.ProductID, ItemID: payload.ItemID, ModifyTime: payload.ModifyTime,
		RawPayload: bodyStr, UpdatedAt: time.Now(),
	}
	res := h.db.Where("order_no = ?", payload.OrderNo).Assign(order).FirstOrCreate(&order)
	if res.Error != nil {
		log.Printf("[Goofish] DB upsert 失败: %v", res.Error)
		c.JSON(200, gin.H{"result": "fail", "msg": "数据库错误"})
		return
	}

	log.Printf("[Goofish] 订单已记录 order_no=%s status=%d type=%d",
		payload.OrderNo, payload.OrderStatus, payload.OrderType)
	c.JSON(200, gin.H{"result": "success", "msg": "接收成功"})
}

// AdminListOrders 管理员查询闲管家订单列表
func (h *GoofishHandler) AdminListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize > 200 {
		pageSize = 200
	}
	q := h.db.Model(&models.GoofishOrder{})
	if status := c.Query("status"); status != "" {
		q = q.Where("order_status = ?", status)
	}
	if orderType := c.Query("order_type"); orderType != "" {
		q = q.Where("order_type = ?", orderType)
	}
	var total int64
	q.Count(&total)
	var items []models.GoofishOrder
	q.Order("modify_time DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items)
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminExportRedeemCodes 导出未使用的闲鱼充值码 CSV
func (h *GoofishHandler) AdminExportRedeemCodes(c *gin.Context) {
	noteFilter := c.DefaultQuery("note_filter", "闲鱼")
	type codeRow struct {
		Code          string     `gorm:"column:code"`
		BalanceAmount float64    `gorm:"column:balance_amount"`
		Note          string     `gorm:"column:note"`
		ExpiresAt     *time.Time `gorm:"column:expires_at"`
	}
	var rows []codeRow
	h.db.Raw(`SELECT code, balance_amount, note, expires_at FROM redeem_codes
	          WHERE status='unused' AND note LIKE ? ORDER BY balance_amount, code`,
		"%"+noteFilter+"%").Scan(&rows)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="goofish_codes_`+time.Now().Format("20060102_150405")+`.csv"`)
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	c.Writer.Write([]byte("充值码,面额(元),备注,过期时间\n"))
	for _, r := range rows {
		exp := ""
		if r.ExpiresAt != nil {
			exp = r.ExpiresAt.Format("2006-01-02")
		}
		c.Writer.Write([]byte(r.Code + "," +
			strconv.FormatFloat(r.BalanceAmount, 'f', 2, 64) + "," +
			r.Note + "," + exp + "\n"))
	}
}

// AdminStockSummary 库存概况
func (h *GoofishHandler) AdminStockSummary(c *gin.Context) {
	type stockRow struct {
		Note          string  `json:"note"`
		BalanceAmount float64 `json:"balance_amount"`
		Unused        int64   `json:"unused"`
		Used          int64   `json:"used"`
	}
	var rows []stockRow
	h.db.Raw(`SELECT note, balance_amount,
	          SUM(CASE WHEN status='unused' THEN 1 ELSE 0 END) AS unused,
	          SUM(CASE WHEN status='used' THEN 1 ELSE 0 END) AS used
	          FROM redeem_codes WHERE note LIKE '%闲鱼%'
	          GROUP BY note, balance_amount ORDER BY balance_amount DESC`).Scan(&rows)
	threshold, _ := strconv.Atoi(GetSettingValue(h.db, "goofish_stock_alert_threshold", "5"))
	c.JSON(200, gin.H{"items": rows, "threshold": threshold})
}
