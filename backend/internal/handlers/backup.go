package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	cryptoRand "crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	"github.com/redis/go-redis/v9"
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
	db  *gorm.DB
	rdb *redis.Client
}

func NewBackupHandler(db *gorm.DB, rdb *redis.Client) *BackupHandler {
	return &BackupHandler{db: db, rdb: rdb}
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

// DryRun 把备份恢复到临时 database, 对比 count, 立即销毁 (不改主 DB)
type dryRunReq struct {
	BackupToken string `json:"backup_token" binding:"required"`
	Key         string `json:"key" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

type tableDiff struct {
	Name           string `json:"name"`
	CurrentCount   int64  `json:"current_count"`
	RestoredCount  int64  `json:"restored_count"`
	Delta          int64  `json:"delta"`
}

type dryRunResult struct {
	TestDBName string      `json:"test_db_name"`
	Tables     []tableDiff `json:"tables"`
	DurationMs int64       `json:"duration_ms"`
}

const testDBName = "ai_gateway_restore_test"

func (h *BackupHandler) DryRun(c *gin.Context) {
	t0 := time.Now()
	var req dryRunReq
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

	// 1. 从 R2 下载 + 解密
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
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

	// 2. 从解密 tar 抽 database.sql.gz 并解压到内存
	dumpSQL, err := extractDatabaseDump(plain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "extract dump failed: " + err.Error()})
		return
	}

	// 3. 主 Postgres 建 test database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DATABASE_URL not set"})
		return
	}
	// 先清理可能存在的旧 test db
	dropCmd := exec.CommandContext(ctx, "psql", dsn, "-c", "DROP DATABASE IF EXISTS "+testDBName)
	if out, err := dropCmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "drop old test db failed: " + err.Error() + " / " + string(out)})
		return
	}
	// 建新 test db (用 postgres 系统 db 连接执行 CREATE)
	createCmd := exec.CommandContext(ctx, "psql", dsn, "-c", "CREATE DATABASE "+testDBName)
	if out, err := createCmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create test db failed: " + err.Error() + " / " + string(out)})
		return
	}
	// defer: 无论成功失败都清理 test db
	defer func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_ = exec.CommandContext(dropCtx, "psql", dsn, "-c", "DROP DATABASE IF EXISTS "+testDBName).Run()
	}()

	// 4. 灌 dump 到 test db (dump 里含 CREATE DATABASE ai_gateway + \connect, 需要跳过这些行 或者直接连 test db 灌)
	// pg_dump --create 的 dump 会包含 CREATE DATABASE + \connect - 我们连 test db 执行, --set ON_ERROR_STOP=1 让 psql 遇到 CREATE DATABASE 报错跳过
	testDSN := replaceDBName(dsn, testDBName)
	loadCmd := exec.CommandContext(ctx, "psql", testDSN, "-v", "ON_ERROR_STOP=0", "-q")
	loadCmd.Stdin = bytes.NewReader(dumpSQL)
	if out, err := loadCmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load dump failed: " + err.Error() + " / " + string(out)[:min(500, len(out))]})
		return
	}

	// 5. 自动发现所有非系统表, 对比 count
	tables, err := discoverTables(ctx, dsn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "discover tables failed: " + err.Error()})
		return
	}
	diffs := make([]tableDiff, 0, len(tables))
	for _, tbl := range tables {
		current, _ := countRows(ctx, dsn, tbl)
		restored, _ := countRows(ctx, testDSN, tbl)
		diffs = append(diffs, tableDiff{
			Name:          tbl,
			CurrentCount:  current,
			RestoredCount: restored,
			Delta:         restored - current,
		})
	}
	sort.Slice(diffs, func(i, j int) bool {
		// abs(delta) 降序: 变化最大的在最上面
		ai, aj := diffs[i].Delta, diffs[j].Delta
		if ai < 0 {
			ai = -ai
		}
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})

	c.JSON(http.StatusOK, dryRunResult{
		TestDBName: testDBName,
		Tables:     diffs,
		DurationMs: time.Since(t0).Milliseconds(),
	})
}

// extractDatabaseDump 从解密后的外层 tar 里抽 database.sql.gz 并解压
func extractDatabaseDump(plain []byte) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(plain))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(hdr.Name, "database.sql.gz") {
			continue
		}
		gzBuf, err := io.ReadAll(io.LimitReader(tr, 500*1024*1024))
		if err != nil {
			return nil, err
		}
		gz, err := gzip.NewReader(bytes.NewReader(gzBuf))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		return io.ReadAll(gz)
	}
	return nil, fmt.Errorf("database.sql.gz not found in backup")
}

// replaceDBName: postgres://user:pass@host:5432/orig?sslmode=disable -> ...:5432/newdb?sslmode=disable
func replaceDBName(dsn, newDB string) string {
	// 简单实现: 找最后一个 / 到 ? 之间的部分替换
	qIdx := strings.Index(dsn, "?")
	var params string
	if qIdx >= 0 {
		params = dsn[qIdx:]
		dsn = dsn[:qIdx]
	}
	slashIdx := strings.LastIndex(dsn, "/")
	if slashIdx < 0 {
		return dsn + "/" + newDB + params
	}
	return dsn[:slashIdx] + "/" + newDB + params
}

func discoverTables(ctx context.Context, dsn string) ([]string, error) {
	sql := "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
	out, err := exec.CommandContext(ctx, "psql", dsn, "-t", "-A", "-c", sql).Output()
	if err != nil {
		return nil, err
	}
	var tables []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			tables = append(tables, line)
		}
	}
	return tables, nil
}

func countRows(ctx context.Context, dsn, tbl string) (int64, error) {
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %q", tbl)
	out, err := exec.CommandContext(ctx, "psql", dsn, "-t", "-A", "-c", sql).Output()
	if err != nil {
		return 0, err
	}
	var n int64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n, nil
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

// ===== 紧急备份 (对称版 backup_full.sh 的 Go 实现) =====

// encryptOpenSSL 兼容 `openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt`
// 输出格式: "Salted__" (8B) + salt (8B) + ciphertext
func encryptOpenSSL(plaintext []byte, password string) ([]byte, error) {
	// 生成 8 字节随机 salt
	salt := make([]byte, 8)
	if _, err := cryptoRand.Read(salt); err != nil {
		return nil, err
	}
	// PBKDF2 派生 key(32) + iv(16)
	dk := pbkdf2.Key([]byte(password), salt, pbkdf2Iter, 48, sha256.New)
	key := dk[:32]
	iv := dk[32:48]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// PKCS#7 padding
	blockSize := aes.BlockSize
	padLen := blockSize - (len(plaintext) % blockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// 拼装: Salted__ + salt + ciphertext
	out := make([]byte, 0, 16+len(ciphertext))
	out = append(out, []byte(openSSLMagic)...)
	out = append(out, salt...)
	out = append(out, ciphertext...)
	return out, nil
}

type emergencyReq struct {
	BackupToken string `json:"backup_token" binding:"required"`
	Password    string `json:"password" binding:"required"`     // BACKUP_ENC_PASSWORD
	Reason      string `json:"reason,omitempty"`                  // 触发原因(可选,写入 metadata)
}

type emergencyResult struct {
	Key           string `json:"key"`             // R2 里的文件名
	SizeBytes     int    `json:"size_bytes"`      // 加密后大小
	DurationMs    int64  `json:"duration_ms"`     // 耗时
	DBSizeBytes   int    `json:"db_size_bytes"`   // gzip 后 dump 大小
}

// POST /admin/backup/emergency - 触发紧急备份到 R2 (正式恢复前用)
func (h *BackupHandler) Emergency(c *gin.Context) {
	t0 := time.Now()
	var req emergencyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validateBackupToken(req.BackupToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "备份操作令牌无效或已过期"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	// 1. pg_dump 到内存, 加 gzip 压缩
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DATABASE_URL not set"})
		return
	}
	dumpCmd := exec.CommandContext(ctx, "pg_dump", dsn, "--clean", "--if-exists", "--no-owner", "--no-privileges")
	var dumpBuf bytes.Buffer
	var dumpErr bytes.Buffer
	dumpCmd.Stdout = &dumpBuf
	dumpCmd.Stderr = &dumpErr
	if err := dumpCmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pg_dump failed: " + err.Error() + " / " + dumpErr.String()})
		return
	}
	rawDump := dumpBuf.Bytes()

	// 2. gzip 压缩 dump
	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	if _, err := gzWriter.Write(rawDump); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gzip write: " + err.Error()})
		return
	}
	if err := gzWriter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gzip close: " + err.Error()})
		return
	}
	gzippedDump := gzBuf.Bytes()

	// 3. 打 tar 包 (跟 backup_full.sh 结构一致: emergency_TS/database.sql.gz + metadata.json)
	tsStr := time.Now().Format("20060102_150405")
	dirName := "emergency_" + tsStr
	adminID := c.GetString("user_id")
	metadata := fmt.Sprintf(`{
  "type": "emergency",
  "triggered_at": "%s",
  "triggered_by_user_id": "%s",
  "reason": %q,
  "db_size_bytes": %d
}
`, time.Now().UTC().Format(time.RFC3339), adminID, req.Reason, len(gzippedDump))

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, entry := range []struct {
		name    string
		content []byte
	}{
		{dirName + "/database.sql.gz", gzippedDump},
		{dirName + "/metadata.json", []byte(metadata)},
	} {
		hdr := &tar.Header{
			Name:    entry.name,
			Mode:    0644,
			Size:    int64(len(entry.content)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "tar header: " + err.Error()})
			return
		}
		if _, err := tw.Write(entry.content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "tar write: " + err.Error()})
			return
		}
	}
	if err := tw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tar close: " + err.Error()})
		return
	}

	// 4. openssl-兼容 AES-256-CBC 加密
	encrypted, err := encryptOpenSSL(tarBuf.Bytes(), req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encrypt: " + err.Error()})
		return
	}

	// 5. 上传 R2
	client, err := newR2Client(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	key := "emergency_" + tsStr + ".tar.enc"
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r2Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(encrypted),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "R2 upload: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, emergencyResult{
		Key:         key,
		SizeBytes:   len(encrypted),
		DurationMs:  time.Since(t0).Milliseconds(),
		DBSizeBytes: len(gzippedDump),
	})
}

// ===== 维护模式管理 (Commit 3 of B') =====

// 复用 middleware 里的常量
const maintenanceRedisKey = "maintenance:mode"

// POST /admin/backup/maintenance/enter - 开启维护模式
func (h *BackupHandler) EnterMaintenance(c *gin.Context) {
	var req struct {
		BackupToken string `json:"backup_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validateBackupToken(req.BackupToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "备份操作令牌无效或已过期"})
		return
	}
	if h.rdb == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redis not available"})
		return
	}
	ctx := context.Background()
	if err := h.rdb.Set(ctx, maintenanceRedisKey, "1", 30*time.Minute).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "maintenance_on",
		"expires_at_seconds": 30 * 60,
		"note":    "非 admin 请求将收到 503, 30 分钟后自动解锁",
	})
}

// POST /admin/backup/maintenance/exit - 关闭维护模式
func (h *BackupHandler) ExitMaintenance(c *gin.Context) {
	var req struct {
		BackupToken string `json:"backup_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validateBackupToken(req.BackupToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "备份操作令牌无效或已过期"})
		return
	}
	if h.rdb == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redis not available"})
		return
	}
	ctx := context.Background()
	if err := h.rdb.Del(ctx, maintenanceRedisKey).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "maintenance_off"})
}

// GET /admin/backup/maintenance/status - 查看当前维护模式状态 (不需要 backup_token, 供健康监控)
func (h *BackupHandler) MaintenanceStatus(c *gin.Context) {
	if h.rdb == nil {
		c.JSON(http.StatusOK, gin.H{"maintenance_mode": false, "note": "redis unavailable"})
		return
	}
	ctx := context.Background()
	val, err := h.rdb.Get(ctx, maintenanceRedisKey).Result()
	active := (err == nil && val == "1")
	ttl := int64(0)
	if active {
		if d, e := h.rdb.TTL(ctx, maintenanceRedisKey).Result(); e == nil {
			ttl = int64(d.Seconds())
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"maintenance_mode": active,
		"ttl_seconds":      ttl,
	})
}

