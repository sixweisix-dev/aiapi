package vertex

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const scope = "https://www.googleapis.com/auth/cloud-platform"

// TokenManager 管理 Vertex AI 的 OAuth2 access token
// 内部用 oauth2 库自动 refresh (token 默认 1 小时过期, 自动续期)
type TokenManager struct {
	mu              sync.RWMutex
	credentialsFile string
	tokenSource     oauth2.TokenSource
	projectID       string
	enabled         bool
}

// New 从 service account JSON 文件创建 TokenManager
// credentialsFile 为空字符串 -> 返回 disabled 的 manager (vertex_ai 渠道不可用, 但不阻塞启动)
func New(credentialsFile string) (*TokenManager, error) {
	tm := &TokenManager{credentialsFile: credentialsFile}
	if credentialsFile == "" {
		log.Println("[vertex] VERTEX_CREDENTIALS_PATH not set, vertex_ai channels disabled")
		return tm, nil
	}
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return tm, fmt.Errorf("read vertex credentials %s: %w", credentialsFile, err)
	}
	creds, err := google.CredentialsFromJSON(context.Background(), data, scope)
	if err != nil {
		return tm, fmt.Errorf("parse vertex credentials: %w", err)
	}
	tm.tokenSource = creds.TokenSource
	tm.projectID = creds.ProjectID
	tm.enabled = true
	log.Printf("[vertex] credentials loaded: project=%s", tm.projectID)
	return tm, nil
}

func (t *TokenManager) IsEnabled() bool {
	if t == nil {
		return false
	}
	return t.enabled
}

func (t *TokenManager) ProjectID() string {
	if t == nil {
		return ""
	}
	return t.projectID
}

// GetToken 返回有效 access token. oauth2 库自动管理 refresh.
func (t *TokenManager) GetToken(ctx context.Context) (string, error) {
	if t == nil || !t.enabled {
		return "", fmt.Errorf("vertex token manager not enabled")
	}
	tok, err := t.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get vertex token: %w", err)
	}
	return tok.AccessToken, nil
}
