package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"
)

const (
	r2Bucket       = "transitai-backups"
	pbkdf2Iter     = 200000
	openSSLMagic   = "Salted__"
	memoryTokenTTL = 15 * time.Minute
	dbSummaryLines = 100
)

type BackupHandler struct {
	db *gorm.DB
}

func NewBackupHandler(db *gorm.DB) *BackupHandler {
	return &BackupHandler{db: db}
}

// ===== R2 =====

func newR2Client(ctx context.Context) (*s3.Client, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	if accountID == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY 未配置")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	}), nil
}

// ===== openssl 兼容解密 =====

func decryptOpenSSL(ciphertext []byte, password string) ([]byte, error) {
	if len(ciphertext) < 16 {
		return nil, fmt.Errorf("ciphertext too short")
	}
	if string(ciphertext[:8]) != openSSLMagic {
		return nil, fmt.Errorf("not an openssl enc file")
	}
	salt := ciphertext[8:16]
	body := ciphertext[16:]
	dk := pbkdf2.Key([]byte(password), salt, pbkdf2Iter, 48, sha256.New)
	key := dk[:32]
	iv := dk[32:48]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(body)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext not multiple of block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(body))
	mode.CryptBlocks(plain, body)
	if len(plain) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(plain) {
		return nil, fmt.Errorf("invalid padding (wrong password?)")
	}
	for i := len(plain) - pad; i < len(plain); i++ {
		if int(plain[i]) != pad {
			return nil, fmt.Errorf("invalid padding (wrong password?)")
		}
	}
	return plain[:len(plain)-pad], nil
}

// ===== 短期令牌 (in-process) =====

var backupTokens = struct {
	m map[string]time.Time
}{m: map[string]time.Time{}}

func generateBackupToken(adminID string) string {
	nonce := fmt.Sprintf("%s|%d", adminID, time.Now().UnixNano())
	h := sha256.Sum256([]byte(nonce))
	tok := fmt.Sprintf("%x", h)
	backupTokens.m[tok] = time.Now().Add(memoryTokenTTL)
	now := time.Now()
	for k, exp := range backupTokens.m {
		if exp.Before(now) {
			delete(backupTokens.m, k)
		}
	}
	return tok
}

func validateBackupToken(tok string) bool {
	if tok == "" {
		return false
	}
	exp, ok := backupTokens.m[tok]
	if !ok {
		return false
	}
	if exp.Before(time.Now()) {
		delete(backupTokens.m, tok)
		return false
	}
	return true
}

// ===== Handlers =====

type verifyPasswordReq struct {
	Password string `json:"password" binding:"required"`
}

func (h *BackupHandler) VerifyPassword(c *gin.Context) {
	adminID := c.GetString("user_id")
	var req verifyPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user UserModel
	if err := h.db.First(&user, "id = ?", adminID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "password not set"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}
	token := generateBackupToken(adminID)
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in_seconds": int(memoryTokenTTL.Seconds())})
}

type listReq struct {
	BackupToken string `json:"backup_token" binding:"required"`
}

type BackupItem struct {
	Key      string    `json:"key"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func (h *BackupHandler) List(c *gin.Context) {
	var req listReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validateBackupToken(req.BackupToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "备份操作令牌无效或已过期"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	client, err := newR2Client(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(r2Bucket)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 list failed: " + err.Error()})
		return
	}
	items := make([]BackupItem, 0, len(out.Contents))
	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		if !strings.HasPrefix(key, "full_") || !strings.HasSuffix(key, ".tar.enc") {
			continue
		}
		items = append(items, BackupItem{
			Key:      key,
			Size:     aws.ToInt64(obj.Size),
			Modified: aws.ToTime(obj.LastModified),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Modified.After(items[j].Modified) })
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

type decryptReq struct {
	BackupToken string `json:"backup_token" binding:"required"`
	Key         string `json:"key" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

type decryptResult struct {
	Files     []string `json:"files"`
	DBSummary string   `json:"db_summary"`
	DBSize    int      `json:"db_size"`
	TotalSize int      `json:"total_size"`
}

func (h *BackupHandler) Decrypt(c *gin.Context) {
	var req decryptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validateBackupToken(req.BackupToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "备份操作令牌无效或已过期"})
		return
	}
	if !strings.HasPrefix(req.Key, "full_") || !strings.HasSuffix(req.Key, ".tar.enc") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backup key"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	client, err := newR2Client(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	obj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r2Bucket),
		Key:    aws.String(req.Key),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 get failed: " + err.Error()})
		return
	}
	defer obj.Body.Close()
	enc, err := io.ReadAll(io.LimitReader(obj.Body, 500*1024*1024))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read backup failed: " + err.Error()})
		return
	}
	plain, err := decryptOpenSSL(enc, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解密失败: " + err.Error()})
		return
	}
	result := decryptResult{TotalSize: len(plain)}
	tr := tar.NewReader(bytes.NewReader(plain))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "tar parse failed: " + err.Error()})
			return
		}
		result.Files = append(result.Files, hdr.Name)
		if strings.HasSuffix(hdr.Name, "database.sql.gz") {
			gzBuf, gzErr := io.ReadAll(io.LimitReader(tr, 200*1024*1024))
			if gzErr != nil {
				continue
			}
			result.DBSize = len(gzBuf)
			gz, gzErr := gzip.NewReader(bytes.NewReader(gzBuf))
			if gzErr != nil {
				continue
			}
			head, _ := io.ReadAll(io.LimitReader(gz, 32*1024))
			gz.Close()
			lines := strings.SplitN(string(head), "\n", dbSummaryLines+1)
			if len(lines) > dbSummaryLines {
				lines = lines[:dbSummaryLines]
			}
			result.DBSummary = strings.Join(lines, "\n")
		}
	}
	c.JSON(http.StatusOK, result)
}
