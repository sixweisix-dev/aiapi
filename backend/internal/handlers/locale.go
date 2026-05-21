package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// DetectLocale 根据 Cloudflare 的 CF-IPCountry header 返回建议的 locale。
// 中文区 (CN/HK/TW/MO) → zh; 其他 → en
func DetectLocale(c *gin.Context) {
	country := c.GetHeader("CF-IPCountry")
	if country == "" {
		country = "UNKNOWN"
	}
	locale := "en"
	for _, cc := range []string{"CN", "HK", "TW", "MO"} {
		if strings.EqualFold(country, cc) {
			locale = "zh"
			break
		}
	}
	c.JSON(200, gin.H{
		"country": country,
		"locale":  locale,
	})
}
