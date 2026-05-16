package handlers

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"ai-api-gateway/internal/models"
	"ai-api-gateway/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GoofishSupplierHandler struct {
	db *gorm.DB
}

func NewGoofishSupplierHandler(db *gorm.DB) *GoofishSupplierHandler {
	return &GoofishSupplierHandler{db: db}
}

// 商品 SKU 配置 (硬编码 7 个商品)
type GoofishSKU struct {
	GoodsNo        string  `json:"goods_no"`
	GoodsName      string  `json:"goods_name"`
	Type           string  // balance | membership
	BalanceAmount  float64 // 原价(基础充值额, 不含赠送)
	Bonus          float64 // 活动赠送额(仅 balance 类型, 活动开启时叠加; membership 用 0)
	MembershipTier string  // for membership type
	MembershipDays int     // for membership type
	PriceCents     int64   // 售价 (分)
	CostCents      int64   // 成本 (分, = balance * 100)
	Note           string  // 对应 redeem_codes 的 note 字段
}

var goofishSKUs = []GoofishSKU{
	// BalanceAmount = 原价 / Bonus = 活动赠送(活动开启时叠加) / PriceCents = 防亏损最低售价线(分)
	{"recharge_100", "TransitAI $100 API 充值码 (赠 $8)", "balance", 100, 8, "", 0, 9500, 10000, "闲鱼$100充值码"},
	{"recharge_300", "TransitAI $300 API 充值码 (赠 $30)", "balance", 300, 30, "", 0, 28500, 30000, "闲鱼$300充值码"},
	{"recharge_500", "TransitAI $500 API 充值码 (赠 $75)", "balance", 500, 75, "", 0, 47500, 50000, "闲鱼$500充值码"},
	{"recharge_1000", "TransitAI $1000 API 充值码 (赠 $200)", "balance", 1000, 200, "", 0, 95000, 100000, "闲鱼$1000充值码"},
	{"recharge_3000", "TransitAI $3000 API 充值码 (赠 $750)", "balance", 3000, 750, "", 0, 285000, 300000, "闲鱼$3000充值码"},
	{"member_pro_30", "TransitAI 专业版 30 天 (含 $120 余额)", "membership", 120, 0, "pro", 30, 9400, 12000, "闲鱼专业版30天"},
	{"member_ent_30", "TransitAI 企业版 30 天 (含 $600 余额)", "membership", 600, 0, "enterprise", 30, 47400, 60000, "闲鱼企业版30天"},
}

func findSKU(goodsNo string) *GoofishSKU {
	for i := range goofishSKUs {
		if goofishSKUs[i].GoodsNo == goodsNo {
			return &goofishSKUs[i]
		}
	}
	return nil
}

// ============ 通用响应 ============
func resp(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(200, gin.H{"code": code, "msg": msg, "data": data})
}

// ============ 签名验证 + body 读取 ============
// 返回 (bodyJSON, ok)
func (h *GoofishSupplierHandler) verifySign(c *gin.Context) (string, bool) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[Goofish-Supplier] read body 失败: %v", err)
		resp(c, 1, "read body failed", nil)
		return "", false
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	bodyStr := string(bodyBytes)
	log.Printf("[Goofish-Supplier] >>> 收到请求 path=%s mch_id=%s ts=%s sign=%s body=%q", c.Request.URL.Path, c.Query("mch_id"), c.Query("timestamp"), c.Query("sign"), bodyStr)

	appID := GetSettingValue(h.db, "goofish_app_key", "")
	appSecret := GetSettingValue(h.db, "goofish_app_secret", "")
	mchID := GetSettingValue(h.db, "goofish_mch_id", "")
	mchSecret := GetSettingValue(h.db, "goofish_mch_secret", "")

	if appID == "" || appSecret == "" || mchID == "" || mchSecret == "" {
		log.Printf("[Goofish-Supplier] 凭证未配全 appID=%v appSecret=%v mchID=%v mchSecret=%v", appID!="", appSecret!="", mchID!="", mchSecret!="")
		resp(c, 401, "未配置闲管家凭证 (app_id/app_secret/mch_id/mch_secret)", nil)
		return "", false
	}

	tsStr := c.Query("timestamp")
	var ts int64
	fmt.Sscanf(tsStr, "%d", &ts)
	if abs(time.Now().Unix()-ts) > 300 {
		log.Printf("[Goofish-Supplier] 时间戳过期 对方ts=%d 我方=%d 差=%d秒", ts, time.Now().Unix(), abs(time.Now().Unix()-ts))
		resp(c, 408, "时间戳已超过有效期", nil)
		return "", false
	}

	providedSign := c.Query("sign")
	if !utils.GoofishSupplierVerifySign(appID, appSecret, mchID, mchSecret, bodyStr, ts, providedSign) {
		expectedSign := utils.GoofishSupplierSign(appID, appSecret, mchID, mchSecret, bodyStr, ts)
		log.Printf("[Goofish-Supplier] 签名校验失败\n  对方传 mch_id=%s body=%q ts=%d sign=%s\n  我们算 expected=%s (用 db 里 mch_id=%s mch_secret 前6位=%.6s)",
			c.Query("mch_id"), bodyStr, ts, providedSign, expectedSign, mchID, mchSecret)
		resp(c, 401, "签名错误", nil)
		return "", false
	}
	log.Printf("[Goofish-Supplier] 签名校验通过")
	return bodyStr, true
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// ============ 1. 查询平台信息 ============
// POST /xgj/open/goofish/platform/info
func (h *GoofishSupplierHandler) PlatformInfo(c *gin.Context) {
	if _, ok := h.verifySign(c); !ok {
		return
	}
	appID := GetSettingValue(h.db, "goofish_app_key", "")
	var appIDInt int64
	fmt.Sscanf(appID, "%d", &appIDInt)
	resp(c, 0, "OK", gin.H{"app_id": appIDInt})
}

// ============ 2. 查询商户信息 ============
// POST /xgj/open/goofish/mch/info
func (h *GoofishSupplierHandler) MchInfo(c *gin.Context) {
	if _, ok := h.verifySign(c); !ok {
		return
	}
	mchID := GetSettingValue(h.db, "goofish_mch_id", "")
	var mchIDInt int64
	fmt.Sscanf(mchID, "%d", &mchIDInt)
	resp(c, 0, "OK", gin.H{
		"mch_id":   mchIDInt,
		"mch_name": "TransitAI",
		"balance":  9999999, // 自研系统固定大额 (无库存限制)
	})
}

// ============ 3. 查询商品列表 ============
// POST /xgj/open/goofish/goods/list
func (h *GoofishSupplierHandler) GoodsList(c *gin.Context) {
	bodyStr, ok := h.verifySign(c)
	if !ok {
		return
	}
	var req struct {
		Keyword   string `json:"keyword"`
		GoodsType int    `json:"goods_type"` // 1直充 2卡密 3券码
		PageNo    int    `json:"page_no"`
		PageSize  int    `json:"page_size"`
	}
	json.Unmarshal([]byte(bodyStr), &req)
	if req.PageNo <= 0 {
		req.PageNo = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}

	items := []gin.H{}
	for _, sku := range goofishSKUs {
		// 关键字过滤
		if req.Keyword != "" {
			if !strings.Contains(sku.GoodsName, req.Keyword) && sku.GoodsNo != req.Keyword {
				continue
			}
		}
		// goods_type 过滤: 我们的商品类型为 2 卡密 (充值码即卡密)
		if req.GoodsType != 0 && req.GoodsType != 2 {
			continue
		}
		items = append(items, skuToGoodsItem(sku))
	}

	// 简单分页
	total := len(items)
	start := (req.PageNo - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	resp(c, 0, "OK", gin.H{
		"list":      items[start:end],
		"total":     total,
		"page_no":   req.PageNo,
		"page_size": req.PageSize,
	})
}

func skuToGoodsItem(sku GoofishSKU) gin.H {
	return gin.H{
		"goods_no":     sku.GoodsNo,
		"goods_name":   sku.GoodsName,
		"goods_type":   2, // 2 卡密商品 (充值码即卡密)
		"goods_status": 1,
		"goods_stock":  9999,
		"goods_price":  sku.PriceCents,
		"cost_price":   sku.CostCents,
	}
}

// ============ 4. 查询商品详情 ============
// POST /xgj/open/goofish/goods/info
func (h *GoofishSupplierHandler) GoodsInfo(c *gin.Context) {
	bodyStr, ok := h.verifySign(c)
	if !ok {
		return
	}
	var req struct{ GoodsNo string `json:"goods_no"` }
	json.Unmarshal([]byte(bodyStr), &req)
	sku := findSKU(req.GoodsNo)
	if sku == nil {
		resp(c, 1100, "商品不存在", nil)
		return
	}
	item := skuToGoodsItem(*sku)
	item["goods_desc"] = sku.GoodsName + " - 卡密发货, 在 transitai.cloud 输入卡密激活到账户"
	resp(c, 0, "OK", item)
}

// ============ 5. 创建卡密订单 ⭐ 核心 ============
// POST /xgj/open/goofish/order/purchase/create
func (h *GoofishSupplierHandler) OrderPurchaseCreate(c *gin.Context) {
	bodyStr, ok := h.verifySign(c)
	if !ok {
		return
	}
	var req struct {
		OrderNo     string `json:"order_no"`
		GoodsNo     string `json:"goods_no"`
		BuyQuantity int    `json:"buy_quantity"`
		MaxAmount   int64  `json:"max_amount"`
		NotifyURL   string `json:"notify_url"`
		BizOrderNo  string `json:"biz_order_no"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &req); err != nil {
		log.Printf("[Goofish-Supplier] 下单参数错误(json解析失败): %v", err)
		resp(c, 1201, "下单参数错误", nil)
		return
	}

	sku := findSKU(req.GoodsNo)
	if sku == nil {
		log.Printf("[Goofish-Supplier] 商品不存在! 闲管家传的 goods_no=%q 不在 goofishSKUs 列表中", req.GoodsNo)
		resp(c, 1100, "商品不存在", nil)
		return
	}
	if req.BuyQuantity <= 0 {
		req.BuyQuantity = 1
	}
	if req.OrderNo == "" {
		resp(c, 1201, "order_no 缺失", nil)
		return
	}

	// 总成本校验
	totalCost := sku.PriceCents * int64(req.BuyQuantity)
	if req.MaxAmount > 0 && totalCost > req.MaxAmount {
		log.Printf("[Goofish-Supplier] 金额校验失败 totalCost=%d > maxAmount=%d", totalCost, req.MaxAmount)
		resp(c, 1202, "下单金额低于成本价", nil)
		return
	}

	// 检查 order_no 是否已存在 (幂等)
	var existing models.GoofishOrder
	if err := h.db.Where("order_no = ?", req.OrderNo).First(&existing).Error; err == nil && existing.ID > 0 {
		log.Printf("[Goofish-Supplier] 幂等命中 order_no=%s 已存在 existing.ID=%v", req.OrderNo, existing.ID)
		resp(c, 1203, "下单管家订单号已存在", nil)
		return
	}

	// === 实时生成 N 张卡密 ===
	cards := []gin.H{}
	cardCodes := []string{}
	batchID := fmt.Sprintf("xgj_%s", req.OrderNo)

	// 活动状态: balance 类型在活动开启时加赠送额; membership 不参与活动
	promoEnabled := GetSettingValue(h.db, "recharge_promo_enabled", "false") == "true"
	actualBalance := sku.BalanceAmount
	if sku.Type == "balance" && promoEnabled {
		actualBalance = sku.BalanceAmount + sku.Bonus
	}

	for i := 0; i < req.BuyQuantity; i++ {
		b := make([]byte, 8)
		crypto_rand.Read(b)
		code := strings.ToUpper(hex.EncodeToString(b))
		formatted := code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
		cardCodes = append(cardCodes, formatted)
		expires := time.Now().Add(180 * 24 * time.Hour)

		var e error
		if sku.Type == "balance" {
			e = h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,NOW())`,
				formatted, "balance", actualBalance, "free", 0, batchID, sku.Note, expires, "unused").Error
		} else {
			e = h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,NOW())`,
				formatted, "membership", sku.BalanceAmount, sku.MembershipTier, sku.MembershipDays, batchID, sku.Note, expires, "unused").Error
		}
		if e != nil {
			log.Printf("[Goofish-Supplier] 生成卡密失败: %v", e)
			resp(c, 500, "系统异常,请重试", nil)
			return
		}
		cards = append(cards, gin.H{"card_pwd": formatted})
	}

	// === 入库 goofish_orders ===
	now := time.Now()
	outOrderNo := fmt.Sprintf("XGJ%d%s", now.Unix(), req.OrderNo[len(req.OrderNo)-6:])
	order := models.GoofishOrder{
		OrderNo:      req.OrderNo,
		UserName:     "",
		OrderType:    7, // 卡密订单 (与 webhook 一致)
		OrderStatus:  20,
		RefundStatus: 0,
		ModifyTime:   now.Unix(),
		RawPayload:   bodyStr,
		RedeemCode:   strings.Join(cardCodes, ","),
		ProcessedAt:  &now,
		UpdatedAt:    now,
	}
	h.db.Create(&order)

	log.Printf("[Goofish-Supplier] 卡密订单生成 order_no=%s sku=%s qty=%d codes=%v",
		req.OrderNo, req.GoodsNo, req.BuyQuantity, cardCodes)

	resp(c, 0, "OK", gin.H{
		"order_type":   2, // 卡密
		"order_no":     req.OrderNo,
		"out_order_no": outOrderNo,
		"order_status": 20, // 已成功
		"order_amount": totalCost,
		"goods_no":     req.GoodsNo,
		"goods_name":   sku.GoodsName,
		"buy_quantity": req.BuyQuantity,
		"order_time":   now.Unix(),
		"end_time":     now.Unix(),
		"card_items":   cards,
		"remark":       fmt.Sprintf("卡密已生成, 在 transitai.cloud 输入即可激活到账户"),
	})
}


// ============ 5b. 创建直充订单 (闲管家自研系统必走此接口) ============
// POST /xgj/open/goofish/order/recharge/create
func (h *GoofishSupplierHandler) OrderRechargeCreate(c *gin.Context) {
	bodyStr, ok := h.verifySign(c)
	if !ok {
		return
	}
	var req struct {
		OrderNo     string                 `json:"order_no"`
		GoodsNo     string                 `json:"goods_no"`
		BuyQuantity int                    `json:"buy_quantity"`
		MaxAmount   int64                  `json:"max_amount"`
		NotifyURL   string                 `json:"notify_url"`
		BizOrderNo  string                 `json:"biz_order_no"`
		BizContent  map[string]interface{} `json:"biz_content"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &req); err != nil {
		resp(c, 1201, "下单参数错误", nil)
		return
	}
	sku := findSKU(req.GoodsNo)
	if sku == nil {
		log.Printf("[Goofish-Supplier] 商品不存在! 闲管家传的 goods_no=%q 不在 goofishSKUs 列表中", req.GoodsNo)
		resp(c, 1100, "商品不存在", nil)
		return
	}
	if req.BuyQuantity <= 0 {
		req.BuyQuantity = 1
	}
	if req.OrderNo == "" {
		resp(c, 1201, "order_no 缺失", nil)
		return
	}
	totalCost := sku.PriceCents * int64(req.BuyQuantity)
	if req.MaxAmount > 0 && totalCost > req.MaxAmount {
		resp(c, 1202, "下单金额低于成本价", nil)
		return
	}

	// 幂等
	var existing models.GoofishOrder
	if err := h.db.Where("order_no = ?", req.OrderNo).First(&existing).Error; err == nil && existing.ID > 0 {
		resp(c, 1203, "下单管家订单号已存在", nil)
		return
	}

	// 拿 account (买家在闲鱼填的, 可能是邮箱)
	buyerAccount := ""
	if req.BizContent != nil {
		if v, ok := req.BizContent["account"].(string); ok {
			buyerAccount = v
		}
	}

	// 生成 N 张卡密
	cards := []gin.H{}
	cardCodes := []string{}
	batchID := fmt.Sprintf("xgj_rc_%s", req.OrderNo)

	// 活动状态: balance 类型在活动开启时加赠送额; membership 不参与活动
	promoEnabled := GetSettingValue(h.db, "recharge_promo_enabled", "false") == "true"
	actualBalance := sku.BalanceAmount
	if sku.Type == "balance" && promoEnabled {
		actualBalance = sku.BalanceAmount + sku.Bonus
	}

	for i := 0; i < req.BuyQuantity; i++ {
		b := make([]byte, 8)
		crypto_rand.Read(b)
		code := strings.ToUpper(hex.EncodeToString(b))
		formatted := code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
		cardCodes = append(cardCodes, formatted)
		expires := time.Now().Add(180 * 24 * time.Hour)
		var e error
		if sku.Type == "balance" {
			e = h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,NOW())`,
				formatted, "balance", actualBalance, "free", 0, batchID, sku.Note, expires, "unused").Error
		} else {
			e = h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,NOW())`,
				formatted, "membership", sku.BalanceAmount, sku.MembershipTier, sku.MembershipDays, batchID, sku.Note, expires, "unused").Error
		}
		if e != nil {
			log.Printf("[Goofish-Supplier-Recharge] 生成卡密失败: %v", e)
			resp(c, 500, "系统异常,请重试", nil)
			return
		}
		// 多种字段都塞充值码 (兼容闲管家不同字段处理逻辑)
		cards = append(cards, gin.H{
			"ticket_no":  "TransitAI 充值码",
			"ticket_pwd": formatted,
		})
	}

	now := time.Now()
	outOrderNo := fmt.Sprintf("XGJ%d%s", now.Unix(), req.OrderNo[len(req.OrderNo)-6:])
	order := models.GoofishOrder{
		OrderNo:      req.OrderNo,
		UserName:     buyerAccount,
		OrderType:    1, // 直充
		OrderStatus:  20,
		RefundStatus: 0,
		ModifyTime:   now.Unix(),
		RawPayload:   bodyStr,
		RedeemCode:   strings.Join(cardCodes, ","),
		ProcessedAt:  &now,
		UpdatedAt:    now,
	}
	h.db.Create(&order)

	log.Printf("[Goofish-Supplier-Recharge] 直充订单 order=%s sku=%s qty=%d account=%s codes=%v",
		req.OrderNo, req.GoodsNo, req.BuyQuantity, buyerAccount, cardCodes)

	// 充值说明 + 卡密 (放 remark, 多种格式都试)
	codeStr := strings.Join(cardCodes, ", ")
	remark := fmt.Sprintf("🎉 充值码: %s\n\n【使用方法】\n1. 访问 https://transitai.cloud 注册账号 (任意邮箱)\n2. 个人中心 → 余额 → 输入充值码激活\n3. 余额到账, 即可使用 Claude API\n\n【活动】\n· %s\n· 经济版 0.6× 倍率 (默认), 官方直连 -pro 后缀 2.0× 倍率\n· 全系 Claude 模型支持, OpenAI/Anthropic 双格式兼容\n· 余额永久有效\n\n问题随时咨询客服~", codeStr, sku.GoodsName)

	resp(c, 0, "OK", gin.H{
		"order_no":     req.OrderNo,
		"out_order_no": outOrderNo,
		"order_status": 20,
		"order_amount": totalCost,
		"goods_name":   sku.GoodsName,
		"order_time":   now.Unix(),
		"end_time":     now.Unix(),
		"ticket_items": cards, // 闲管家如果识别这个字段会自动发给买家
		"card_items":   cards, // 兜底
		"remark":       remark,
	})
}

// ============ 6. 查询订单详情 ============
// POST /xgj/open/goofish/order/info
func (h *GoofishSupplierHandler) OrderInfo(c *gin.Context) {
	bodyStr, ok := h.verifySign(c)
	if !ok {
		return
	}
	var req struct{ OrderNo string `json:"order_no"` }
	json.Unmarshal([]byte(bodyStr), &req)

	var order models.GoofishOrder
	if err := h.db.Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
		resp(c, 1200, "订单不存在", nil)
		return
	}

	// 解析卡密
	cards := []gin.H{}
	if order.RedeemCode != "" {
		for _, code := range strings.Split(order.RedeemCode, ",") {
			cards = append(cards, gin.H{"card_pwd": code})
		}
	}

	resp(c, 0, "OK", gin.H{
		"order_type":   2,
		"order_no":     order.OrderNo,
		"out_order_no": fmt.Sprintf("XGJ%d", order.CreatedAt.Unix()),
		"order_status": order.OrderStatus,
		"order_amount": 0,
		"goods_no":     "",
		"goods_name":   "",
		"buy_quantity": len(cards),
		"order_time":   order.CreatedAt.Unix(),
		"end_time":     order.CreatedAt.Unix(),
		"card_items":   cards,
	})
}
