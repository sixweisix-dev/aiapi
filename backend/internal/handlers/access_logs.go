package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const accessLogPath = "/var/log/caddy/access.log"

type AccessLogEntry struct {
	TS         float64 `json:"ts"`
	Method     string  `json:"method"`
	URI        string  `json:"uri"`
	Host       string  `json:"host"`
	Status     int     `json:"status"`
	DurationMs float64 `json:"duration_ms"`
	Size       int     `json:"size"`
	ClientIP   string  `json:"client_ip"`
	UserAgent  string  `json:"user_agent"`
	Country    string  `json:"country"`
}

// tailFile reads last `limit` lines from a file efficiently
func tailFile(path string, limit int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 简单实现:读全文件,取后 limit 行。日志由 roll_size 50mb 控制不会太大
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

func firstHeaderValue(h map[string][]string, key string) string {
	if vs, ok := h[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// ListAccessLogs returns recent Caddy access log entries (parsed from JSON file)
func ListAccessLogs(c *gin.Context) {
	limit := 200
	if l := c.Query("limit"); l != "" {
		if v := parseInt(l, 200); v > 0 && v <= 1000 {
			limit = v
		}
	}
	statusFilter := c.Query("status") // "error" = 4xx/5xx only

	lines, err := tailFile(accessLogPath, limit*3) // 多读一些,过滤后不够
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"logs": []AccessLogEntry{}, "note": "access log not yet created"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]AccessLogEntry, 0, limit)
	// 倒序遍历(最新在最上面)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var raw struct {
			TS       float64 `json:"ts"`
			Status   int     `json:"status"`
			Duration float64 `json:"duration"`
			Size     int     `json:"size"`
			Request  struct {
				Method   string              `json:"method"`
				URI      string              `json:"uri"`
				Host     string              `json:"host"`
				ClientIP string              `json:"client_ip"`
				Headers  map[string][]string `json:"headers"`
			} `json:"request"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if statusFilter == "error" && raw.Status < 400 {
			continue
		}
		cfIP := firstHeaderValue(raw.Request.Headers, "Cf-Connecting-Ip")
		if cfIP == "" {
			cfIP = raw.Request.ClientIP
		}
		result = append(result, AccessLogEntry{
			TS:         raw.TS,
			Method:     raw.Request.Method,
			URI:        raw.Request.URI,
			Host:       raw.Request.Host,
			Status:     raw.Status,
			DurationMs: raw.Duration * 1000,
			Size:       raw.Size,
			ClientIP:   cfIP,
			UserAgent:  firstHeaderValue(raw.Request.Headers, "User-Agent"),
			Country:    firstHeaderValue(raw.Request.Headers, "Cf-Ipcountry"),
		})
		if len(result) >= limit {
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{"logs": result, "total": len(result)})
}
