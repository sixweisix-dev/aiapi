# 备份恢复 SOP

## 前置
- BACKUP_ENC_PASSWORD 密码（43 位）
- rclone 已配好 r2 remote

## 步骤

### 1. 列出可用备份
    rclone lsf r2:transitai-backups | grep 'full_.*\.tar\.enc$'

### 2. 下载指定备份到临时目录
    mkdir -p /tmp/restore && cd /tmp/restore
    TARGET=full_YYYYMMDD_HHMMSS.tar.enc
    rclone copy "r2:transitai-backups/$TARGET" ./

### 3. 解密（用 awk 版本读密码，跟备份脚本一致）
    PWD=$(awk -F= '/^BACKUP_ENC_PASSWORD=/{print $2}' /home/ubuntu/token-api/.env)
    openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -in "$TARGET" -out full.tar -pass "pass:$PWD"
    unset PWD
    tar xf full.tar
    cd full_YYYYMMDD_HHMMSS/

### 4. 恢复 DB 到临时容器验证
    docker run -d --rm --name restore_test -e POSTGRES_PASSWORD=test postgres:16-alpine
    sleep 5
    zcat database.sql.gz | docker exec -i restore_test psql -U postgres
    # 查数据
    docker exec restore_test psql -U postgres -d ai_gateway -c "SELECT COUNT(*) FROM users;"

### 5. 恢复配置文件
    tar tzf configs.tar.gz  # 看清单
    # 有需要的话 tar xzf configs.tar.gz -C /some/place

### 6. 清理
    docker stop restore_test
    cd /tmp && rm -rf /tmp/restore

## 生产恢复（谨慎！）
    - 先停业务: docker compose stop backend
    - dump 到主 postgres: zcat database.sql.gz | docker compose exec -T postgres psql -U postgres
    - 重启: docker compose up -d backend
