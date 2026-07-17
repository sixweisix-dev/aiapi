#!/bin/bash
# 全量加密备份 - 数据库 + 敏感配置文件
# 由 ofelia 每天 03:00 触发

set -uo pipefail

# ---- 配置 ----
PROJECT_DIR="/home/ubuntu/token-api"
BACKUP_DIR="$PROJECT_DIR/backups"
STAGING_DIR="$BACKUP_DIR/.staging"
GDRIVE_REMOTE="r2:transitai-backups"
LOCAL_RETAIN_DAYS=7
REMOTE_RETAIN_DAYS=30
LOG_FILE="$BACKUP_DIR/backup.log"

# 敏感文件清单(除 pg_dump 之外要备的)
CONFIG_FILES=(
  ".env"
  "docker-compose.yml"
  "Caddyfile"
  "docker/ofelia/config.ini"
)
# 目录(比如支付宝私钥所在)
CONFIG_DIRS=(
  ".config/rclone"
)

# 加载 .env
if [ -f "$PROJECT_DIR/.env" ]; then
  # 用 while+read 逐行处理, 避免特殊字符被 shell 解析
  while IFS='=' read -r k v; do
    case "$k" in
      BARK_KEY|BACKUP_ENC_PASSWORD)
        export "$k=$v"
        ;;
    esac
  done < <(grep -E '^(BARK_KEY|BACKUP_ENC_PASSWORD)=' "$PROJECT_DIR/.env")
fi

log() { echo "[$(date '+%F %T')] $*" | tee -a "$LOG_FILE"; }

bark_notify() {
  local title="$1"
  local body="$2"
  if [ -z "${BARK_KEY:-}" ]; then
    log "WARN: BARK_KEY not set, skipping notification"
    return
  fi
  curl -sSf --max-time 10 -X POST \
    "https://api.day.app/${BARK_KEY}/${title}/${body}?group=TransitAI&sound=alarm" \
    >/dev/null 2>&1 || log "WARN: bark push failed"
}

fail() {
  log "FAIL: $*"
  bark_notify "TransitAI备份失败" "$*"
  # 清理 staging
  [ -d "$STAGING_DIR" ] && rm -rf "$STAGING_DIR"
  exit 1
}

# ---- 前置检查 ----
[ -n "${BACKUP_ENC_PASSWORD:-}" ] || fail "BACKUP_ENC_PASSWORD 未设置"
command -v openssl >/dev/null 2>&1 || fail "openssl not installed"
command -v rclone >/dev/null 2>&1 || fail "rclone not installed"

mkdir -p "$BACKUP_DIR" "$STAGING_DIR"
chmod 700 "$STAGING_DIR"

TS=$(date +'%Y%m%d_%H%M%S')
STAGE="$STAGING_DIR/full_${TS}"
mkdir -p "$STAGE"

log "===== 开始备份 ${TS} ====="

# ---- 1. pg_dump ----
log "1/4 pg_dump..."
if ! docker compose -f "$PROJECT_DIR/docker-compose.yml" exec -T postgres \
     pg_dump -U postgres --clean --if-exists --create --no-owner ai_gateway \
     | gzip > "$STAGE/database.sql.gz"; then
  fail "pg_dump 失败"
fi
DB_SIZE=$(stat -c%s "$STAGE/database.sql.gz")
[ "$DB_SIZE" -gt 1024 ] || fail "pg_dump 输出过小 ${DB_SIZE}字节"
log "  DB $((DB_SIZE/1024))KB"

# ---- 2. 配置文件打包 ----
log "2/4 打包配置文件..."
cd /home/ubuntu || fail "cannot cd /home/ubuntu"
TAR_LIST="$STAGE/.tar_list"
: > "$TAR_LIST"
for f in "${CONFIG_FILES[@]}"; do
  full="token-api/$f"
  [ -f "$full" ] && echo "$full" >> "$TAR_LIST" || log "  skip missing: $full"
done
for d in "${CONFIG_DIRS[@]}"; do
  [ -d "$d" ] && echo "$d" >> "$TAR_LIST" || log "  skip missing: $d"
done
if ! tar czf "$STAGE/configs.tar.gz" -T "$TAR_LIST" 2>/dev/null; then
  fail "tar 打包失败"
fi
CFG_SIZE=$(stat -c%s "$STAGE/configs.tar.gz")
log "  Configs $((CFG_SIZE/1024))KB"

# ---- 3. 合并 + 加密 ----
log "3/4 合并 & 加密..."
BUNDLE="$STAGE.tar"
tar cf "$BUNDLE" -C "$STAGING_DIR" "full_${TS}" || fail "bundle tar 失败"

ENC_OUT="$BACKUP_DIR/full_${TS}.tar.enc"
# openssl 3.x 默认强 KDF
if ! openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt \
     -in "$BUNDLE" -out "$ENC_OUT" \
     -pass env:BACKUP_ENC_PASSWORD 2>/dev/null; then
  fail "openssl 加密失败"
fi
ENC_SIZE=$(stat -c%s "$ENC_OUT")
log "  Enc $((ENC_SIZE/1024))KB → $ENC_OUT"

# 清理 staging (含明文!)
rm -rf "$STAGE" "$BUNDLE"

# ---- 4. 上传 R2 ----
log "4/4 上传 R2..."
if ! rclone copy "$ENC_OUT" "$GDRIVE_REMOTE" \
     --timeout 300s --retries 3 --low-level-retries 5; then
  fail "rclone 上传失败"
fi
log "  上传完成"

# ---- 5. 清理过期 ----
log "清理本地 ${LOCAL_RETAIN_DAYS}天前 / 云端 ${REMOTE_RETAIN_DAYS}天前"
find "$BACKUP_DIR" -maxdepth 1 -name 'full_*.tar.enc' -mtime +${LOCAL_RETAIN_DAYS} -delete
# 旧格式的裸备份文件也扫一下
find "$BACKUP_DIR" -maxdepth 1 -name 'ai_gateway_*.sql*' -mtime +${LOCAL_RETAIN_DAYS} -delete
rclone delete "$GDRIVE_REMOTE" --min-age ${REMOTE_RETAIN_DAYS}d --timeout 300s 2>>"$LOG_FILE" || log "WARN: 云端清理有警告"

TOTAL_KB=$((ENC_SIZE/1024))
log "===== 完成 ${TOTAL_KB}KB ====="
bark_notify "TransitAI备份OK" "${TOTAL_KB}KB→R2"
