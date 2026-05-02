package handlers

import (
	crypto_rand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CronHandler 处理定时任务触发(由 ofelia 容器内调用)
type CronHandler struct {
	db      *gorm.DB
	mailCfg MailConfig
	secret  string // INTERNAL_CRON_TOKEN, 防止外部调用
}

func NewCronHandler(db *gorm.DB, mailCfg MailConfig, secret string) *CronHandler {
	return &CronHandler{db: db, mailCfg: mailCfg, secret: secret}
}

// 校验 token, 内部调用必须带 X-Cron-Token header
func (h *CronHandler) checkToken(c *gin.Context) bool {
	if h.secret == "" {
		log.Println("[Cron] WARN: INTERNAL_CRON_TOKEN not set, accepting all calls (dev mode)")
		return true
	}
	return c.GetHeader("X-Cron-Token") == h.secret
}

// DailyReport 生成昨日成本日报 + 超阈值发邮件
func (h *CronHandler) DailyReport(c *gin.Context) {
	if !h.checkToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// 设置(含告警邮箱 / 阈值)
	alertEmail := GetSettingValue(h.db, "alert_email", os.Getenv("EMAIL_FROM"))
	warnStr := GetSettingValue(h.db, "alert_warn_threshold", "100")
	critStr := GetSettingValue(h.db, "alert_critical_threshold", "500")
	warnT, _ := strconv.ParseFloat(warnStr, 64)
	critT, _ := strconv.ParseFloat(critStr, 64)

	// 时间范围: 昨天 00:00 ~ 今天 00:00 (服务器本地时区, 应当是 Asia/Shanghai)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterday := today.Add(-24 * time.Hour)

	var res struct {
		TotalRevenue   float64 // 用户付的钱(含 1.5x)
		TotalRequests  int64
		TotalUsers     int64
		FailedRequests int64
		TotalRecharge  float64 // 昨日充值成功金额
	}

	// 收入(requests.cost) + 调用次数 + 失败数
	h.db.Raw(`
		SELECT 
			COALESCE(SUM(cost), 0) AS total_revenue,
			COUNT(*) AS total_requests,
			COUNT(DISTINCT user_id) AS total_users,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS failed_requests
		FROM requests
		WHERE created_at >= ? AND created_at < ?
	`, yesterday, today).Scan(&res)

	// 昨日充值成功金额
	h.db.Raw(`
		SELECT COALESCE(SUM(amount), 0) AS total_recharge
		FROM recharge_orders
		WHERE payment_status = 'paid' AND paid_at >= ? AND paid_at < ?
	`, yesterday, today).Scan(&res)

	// 算上游真实成本: 默认倍率 1.5x, 用户付 1.5 元 => 你付 1 元给上游
	upstreamCost := res.TotalRevenue / 1.5
	grossProfit := res.TotalRevenue - upstreamCost
	failRate := 0.0
	if res.TotalRequests > 0 {
		failRate = float64(res.FailedRequests) / float64(res.TotalRequests) * 100
	}

	level := "info"
	prefix := ""
	if upstreamCost >= critT {
		level = "critical"
		prefix = "🚨 紧急 "
	} else if upstreamCost >= warnT {
		level = "warn"
		prefix = "⚠️ 警告 "
	}

	subject := fmt.Sprintf("%sTransitAI 日报 %s | 上游成本 ¥%.2f", prefix, yesterday.Format("01-02"), upstreamCost)
	body := fmt.Sprintf(`TransitAI 每日运营日报

📅 数据范围：%s 全天 (Asia/Shanghai)

💰 财务
- 用户消费总额（收入）  ¥%.2f
- 上游 API 真实成本    ¥%.2f
- 毛利              ¥%.2f
- 昨日充值成功         ¥%.2f

📊 流量
- API 调用总次数       %d
- 活跃用户数          %d
- 失败请求数         %d (%.1f%%)

📌 告警阈值
- 警告阈值         ¥%.2f (上游成本)
- 紧急阈值         ¥%.2f
- 当前级别         %s

如需调整阈值，登录管理后台 → 系统设置。
`, yesterday.Format("2006-01-02"), res.TotalRevenue, upstreamCost, grossProfit, res.TotalRecharge,
		res.TotalRequests, res.TotalUsers, res.FailedRequests, failRate, warnT, critT, level)

	// 总是记日志
	log.Printf("[DailyReport] %s | revenue=%.2f cost=%.2f profit=%.2f reqs=%d users=%d failed=%d level=%s",
		yesterday.Format("2006-01-02"), res.TotalRevenue, upstreamCost, grossProfit,
		res.TotalRequests, res.TotalUsers, res.FailedRequests, level)

	// 仅在 warn / critical 时发邮件; info 等级跳过(避免每日噪音)
	// 如需每日都收, 把下面 if 去掉
	sent := false
	if level != "info" && alertEmail != "" {
		if err := sendPlainMail(h.mailCfg, alertEmail, subject, body); err != nil {
			log.Printf("[DailyReport] email send error: %v", err)
		} else {
			sent = true
			log.Printf("[DailyReport] email sent to %s", alertEmail)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"date":           yesterday.Format("2006-01-02"),
		"revenue":        res.TotalRevenue,
		"upstream_cost":  upstreamCost,
		"gross_profit":   grossProfit,
		"requests":       res.TotalRequests,
		"users":          res.TotalUsers,
		"failed":         res.FailedRequests,
		"recharge_total": res.TotalRecharge,
		"level":          level,
		"email_sent":     sent,
	})
	_ = json.Marshal // 防 unused import
}

// sendPlainMail 复用 mailer 风格(避免 import cycle)
func sendPlainMail(cfg MailConfig, to, subject, body string) error {
	port, err := strconv.Atoi(cfg.Port)
	if err != nil {
		port = 587
	}
	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body)
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
}

// CodeRestockCheck 检查兑换码库存，不足时自动生成并发邮件
func (h *CronHandler) CodeRestockCheck(c *gin.Context) {
	if !h.checkToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	h.doRestock(c)
}

// AdminRestock 管理员手动触发（JWT 鉴权）
func (h *CronHandler) AdminRestock(c *gin.Context) {
	h.doRestock(c)
}

func (h *CronHandler) doRestock(c *gin.Context) {
	alertEmail := GetSettingValue(h.db, "alert_email", os.Getenv("EMAIL_FROM"))

	// 各档位配置: type, balance, tier, days, threshold(低于此数补货), generate(每次补多少)
	type stockCfg struct {
		Note      string
		Type      string
		Balance   float64
		Tier      string
		Days      int
		Threshold int
		Generate  int
	}

	// 从 settings 动态读阶梯规则
	tiersJSON := GetSettingValue(h.db, "recharge_tiers", "[]")
	var tierDefs []struct {
		Min   float64 `json:"min"`
		Bonus float64 `json:"bonus"`
	}
	if err := json.Unmarshal([]byte(tiersJSON), &tierDefs); err != nil {
		log.Printf("[Restock] parse recharge_tiers failed: %v", err)
	}

	var configs []stockCfg
	for _, t := range tierDefs {
		note := fmt.Sprintf("闲鱼¥%.0f充值码", t.Min)
		balance := t.Min + t.Bonus
		thresh := 5
		if t.Min >= 3000 {
			thresh = 2
		} else if t.Min >= 1000 {
			thresh = 3
		} else if t.Min >= 500 {
			thresh = 3
		}
		gen := 20
		if t.Min >= 3000 {
			gen = 3
		} else if t.Min >= 1000 {
			gen = 5
		} else if t.Min >= 500 {
			gen = 10
		}
		configs = append(configs, stockCfg{note, "balance", balance, "free", 0, thresh, gen})
	}
	// 固定会员档位
	configs = append(configs,
		stockCfg{"闲鱼专业版30天", "membership", 120, "pro", 30, 3, 10},
		stockCfg{"闲鱼企业版30天", "membership", 600, "enterprise", 30, 1, 5},
	)

	type restockResult struct {
		Note  string
		Count int
		Codes []string
	}

	var results []restockResult
	for _, cfg := range configs {
		// 查未使用数量
		var unused int64
		h.db.Raw(`SELECT COUNT(*) FROM redeem_codes WHERE note=? AND status='unused'`, cfg.Note).Scan(&unused)
		if int(unused) > cfg.Threshold {
			continue
		}

		// 生成新码
		batchID := fmt.Sprintf("auto_%d", time.Now().Unix())
		var codes []string
		for i := 0; i < cfg.Generate; i++ {
			b := make([]byte, 8)
			crypto_rand.Read(b)
			code := strings.ToUpper(hex.EncodeToString(b))
			formatted := code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
			codes = append(codes, formatted)
			expires := time.Now().Add(180 * 24 * time.Hour)
			mtier := cfg.Tier
			if mtier == "" {
				mtier = "free"
			}
			h.db.Exec(`INSERT INTO redeem_codes(code,type,balance_amount,membership_tier,membership_days,batch_id,note,expires_at) VALUES(?,?,?,?,?,?,?,?)`,
				formatted, cfg.Type, cfg.Balance, mtier, cfg.Days, batchID, cfg.Note, expires)
		}
		results = append(results, restockResult{cfg.Note, len(codes), codes})
		log.Printf("[Restock] %s: 库存不足(%d)，已生成 %d 个新码", cfg.Note, unused, len(codes))
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "库存充足，无需补货"})
		return
	}

	// 发邮件通知
	if alertEmail != "" {
		body := fmt.Sprintf("TransitAI 兑换码自动补货通知\n\n以下档位已自动生成新码，请及时添加到闲鱼自动发货库存：\n\n")
		for _, r := range results {
			body += fmt.Sprintf("【%s】新增 %d 个：\n", r.Note, r.Count)
			for _, code := range r.Codes {
				body += code + "\n"
			}
			body += "\n"
		}
		body += "\n请登录闲鱼，将以上对应的码添加到自动发货库存中。"
		sendPlainMail(h.mailCfg, alertEmail, "⚠️ TransitAI 兑换码库存补货通知", body)
	}

	c.JSON(http.StatusOK, gin.H{"restocked": len(results), "details": results})
}

// PromoDateCheck 检查活动日期，自动开关活动
func (h *CronHandler) PromoDateCheck(c *gin.Context) {
	if !h.checkToken(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	now := time.Now()
	start := GetSettingValue(h.db, "promo_start", "")
	end := GetSettingValue(h.db, "promo_end", "")
	current := GetSettingValue(h.db, "promo_enabled", "false")

	if start == "" || end == "" {
		c.JSON(http.StatusOK, gin.H{"message": "未设置活动日期"})
		return
	}

	startT, err1 := time.Parse("2006-01-02", start)
	endT, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusOK, gin.H{"message": "日期格式错误"})
		return
	}
	endT = endT.Add(24*time.Hour - time.Second) // 结束日期包含当天

	shouldEnable := now.After(startT) && now.Before(endT)
	newVal := "false"
	if shouldEnable {
		newVal = "true"
	}

	if current == newVal {
		c.JSON(http.StatusOK, gin.H{"message": "活动状态无需变更", "promo_enabled": shouldEnable})
		return
	}

	h.db.Exec(`UPDATE settings SET value=? WHERE key='promo_enabled'`, newVal)
	log.Printf("[PromoCheck] 活动状态变更: %s -> %s", current, newVal)

	alertEmail := GetSettingValue(h.db, "alert_email", os.Getenv("EMAIL_FROM"))
	if alertEmail != "" {
		action := "已开启"
		if !shouldEnable {
			action = "已关闭"
		}
		sendPlainMail(h.mailCfg, alertEmail,
			"TransitAI 充值活动状态变更",
			fmt.Sprintf("充值赠送活动%s\n\n活动时间：%s 至 %s\n当前时间：%s",
				action, start, end, now.Format("2006-01-02 15:04")))
	}

	c.JSON(http.StatusOK, gin.H{"message": "活动状态已更新", "promo_enabled": shouldEnable})
}
