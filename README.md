# AI API Gateway

一个 AI 模型 API 中转/聚合平台，提供统一的 OpenAI 兼容接口，支持多上游提供商（OpenAI、Anthropic、Google Gemini 等），包含用户鉴权、Token 精准计费、余额管理和管理后台。

## 功能特性

### 核心功能
- **统一 OpenAI 兼容接口**：对外暴露标准的 `/v1/chat/completions`、`/v1/models` 等接口
- **多上游支持**：OpenAI、Anthropic Claude、Google Gemini、国产模型（Qwen/DeepSeek）
- **流式响应**：支持 SSE 流式传输，低延迟透传
- **负载均衡**：上游账号池，支持轮询、加权、最少使用等策略
- **故障转移**：自动健康检查，失败重试，熔断机制

### 用户管理
- 邮箱注册/登录，支持 GitHub/Google OAuth
- API Key 管理（创建、删除、查看用量）
- 权限分级：游客/普通/VIP/管理员
- 单 Key 限速（RPM/TPM）和模型白名单

### 计费系统
- 实时 Token 统计（prompt_tokens/completion_tokens）
- 模型定价表，支持输入/输出单价配置
- 公开倍率系统（成本×倍率=售价）
- 余额预检查，不足返回 402
- 详细消费日志（模型、tokens、费用、IP、耗时）

### 钱包与支付
- 预充值余额系统
- 支付接入：Stripe（海外）、支付宝（国内）
- 充值订单管理
- 消费记录导出 CSV

### 管理后台
- 仪表盘：实时流水、用户数、请求量、毛利
- 用户管理：搜索、封禁、调整余额
- 上游渠道管理：API Key 管理、健康检查
- 模型与价格管理
- 日志查询与审计

### 监控告警
- 健康检查（DB + Redis 深度检测）
- Telegram Bot 告警（上游宕机、错误率飙升）
- 自动数据库备份

## 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin Web Framework
- **数据库**: PostgreSQL 16+
- **缓存**: Redis 7+
- **ORM**: GORM
- **认证**: JWT + bcrypt

### 前端
- **管理后台**: Vue 3 + Vite + Tailwind CSS
- **用户前台**: Vue 3 + Vite + Tailwind CSS

### 部署
- **容器**: Docker + Docker Compose
- **反向代理**: Caddy（自动 HTTPS）
- **服务器**: Ubuntu 22.04+，2C4G 起步
- **调度**: Ofelia（Docker 内 cron）

---

## 快速开始（开发环境）

### 环境要求
- Docker 20.10+
- Docker Compose 2.20+
- Git
- Go 1.21+（可选，本地开发）
- Node.js 18+（可选，前端开发）

### 启动开发环境

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env，至少修改 JWT_SECRET

# 2. 构建前端
make build-all

# 3. 启动
docker compose up -d

# 4. 验证
curl http://localhost:8080/health
```

服务启动后：
| 服务 | 地址 |
|------|------|
| API 入口 | `http://localhost:80/v1/` |
| 管理后台 | `http://localhost:80/admin/` |
| 用户前台 | `http://localhost:80/` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |

默认管理员账号：
- 邮箱: `admin@example.com`
- 密码: `admin123`

> **重要**：首次登录后请立即修改密码！

---

## 生产环境部署（VPS）

### 一键部署（推荐）

```bash
# 以 root 登录服务器
ssh root@your-server-ip

# 设置域名（必须！）
export DOMAIN=api.yourdomain.com

# 下载并运行部署脚本
curl -fsSL https://raw.githubusercontent.com/your-repo/main/scripts/setup.sh | bash
```

### 手动部署

#### 1. 服务器准备

```bash
# Ubuntu 22.04 / 24.04
sudo apt update && sudo apt upgrade -y
sudo apt install -y docker.io docker-compose-plugin git curl
sudo systemctl enable docker --now
```

#### 2. 配置防火墙

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS
sudo ufw --force enable
```

#### 3. 部署应用

```bash
# 克隆项目
git clone <your-repo-url> /opt/ai-api-gateway
cd /opt/ai-api-gateway

# 配置环境变量
cp .env.example .env
# 编辑 .env，修改以下值：
#   JWT_SECRET         → openssl rand -hex 32
#   ADMIN_EMAIL        → your@email.com
#   ADMIN_PASSWORD     → 强密码
#   DOMAIN             → api.yourdomain.com
#   ENVIRONMENT        → production
#   TELEGRAM_BOT_TOKEN → (可选) 用于告警通知

# 构建前端
make build-all

# 启动
docker compose up -d

# 验证健康
curl http://localhost:8080/health
# 预期: {"status":"ok","service":"ai-api-gateway","checks":{"database":true,"redis":true}}
```

#### 4. 配置域名与 HTTPS

```bash
# 编辑 Caddyfile，取消生产配置块的注释，替换 example.com 为你的域名
nano Caddyfile

# 重启 Caddy 自动申请 Let's Encrypt 证书
docker compose restart caddy
```

### 生产环境 `.env` 参考

```bash
# 必须修改
JWT_SECRET=<随机 64 位十六进制字符串>
ADMIN_EMAIL=admin@yourdomain.com
ADMIN_PASSWORD=<强密码>
ENVIRONMENT=production

# 数据库（默认即可，端口不暴露到公网）
DATABASE_URL=postgres://postgres:postgres@postgres:5432/ai_gateway?sslmode=disable

# 上游 API 密钥（至少配置一个）
OPENAI_API_KEY=sk-xxxx
ANTHROPIC_API_KEY=sk-ant-xxxx
GOOGLE_API_KEY=xxxx

# 监控（可选，推荐配置）
TELEGRAM_BOT_TOKEN=<BotFather 获取的 Token>
TELEGRAM_CHAT_ID=<告警接收群组 ID>

# 支付（按需配置）
ALIPAY_APP_ID=xxxx
ALIPAY_PRIVATE_KEY=xxxx
STRIPE_SECRET_KEY=sk_test_xxxx
```

---

## 运维指南

### 服务管理

```bash
# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f          # 所有服务
docker compose logs -f backend  # 仅后端
docker compose logs -f caddy    # 仅反向代理

# 重启单个服务
docker compose restart backend

# 停止所有服务
docker compose down

# 彻底清理（删除数据卷）
docker compose down -v
```

### 数据库备份

**自动备份**（每日凌晨 2:00，由 Ofelia cron 调度）：
```bash
# 查看 cron 任务
docker compose exec ofelia cat /etc/ofelia/config.ini

# 查看备份文件
ls -la backups/
```

**手动备份**：
```bash
make backup
# 或
docker compose exec postgres pg_dump -U postgres ai_gateway > backups/manual_$(date +%Y%m%d).sql
```

**恢复备份**：
```bash
make restore FILE=backups/ai_gateway_20250101_020000.sql
# 或
cat backups/backup.sql | docker compose exec -T postgres psql -U postgres ai_gateway
```

备份文件默认保留 30 天，位置在 `./backups/` 目录。

### 监控与告警

系统内置 Telegram Bot 告警，自动检测：

| 告警类型 | 触发条件 | 检查频率 |
|---------|---------|---------|
| 上游不可用 | 渠道 health_status = unhealthy | 每 5 分钟 |
| 提供商全面宕机 | 某提供商所有渠道不可用 | 每 5 分钟 |
| 错误率飙升 | 5xx 占比 > 20% | 每 5 分钟 |
| 大量慢请求 | 超过 10 个请求耗时 > 30s | 每 5 分钟 |

配置方式：
1. 在 Telegram 中搜索 `@BotFather`，创建 Bot 获取 Token
2. 获取 Chat ID（发送消息给 Bot 后访问 `https://api.telegram.org/bot<TOKEN>/getUpdates`）
3. 在 `.env` 中设置 `TELEGRAM_BOT_TOKEN` 和 `TELEGRAM_CHAT_ID`
4. 重启后端：`docker compose restart backend`

### 健康检查端点

```
GET /health
{
  "status": "ok",
  "service": "ai-api-gateway",
  "checks": {
    "database": true,
    "redis": true
  }
}
```

可以在 UptimeRobot、Pingdom 等外部监控服务中配置此端点。

### 日志管理

Docker 日志自动轮转（每个容器）：
- 每个日志文件最大 10MB
- 保留最近 3 个文件
- 配置在 `docker-compose.yml` 的 `x-logging` 锚点

---

## API 文档

### OpenAI 兼容接口

```bash
# 列出模型
curl http://localhost/v1/models \
  -H "Authorization: Bearer sk-xxxx"

# 聊天补全（非流式）
curl http://localhost/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# 聊天补全（流式）
curl http://localhost/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### 管理 API

```bash
# 管理员登录
curl -X POST http://localhost/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin123"}'

# 管理后台仪表盘（使用返回的 token）
curl http://localhost/v1/admin/dashboard \
  -H "Authorization: Bearer <jwt-token>"
```

---

## 项目结构

```
ai-api-gateway/
├── backend/                    # Go 后端服务
│   ├── cmd/api/main.go        # 入口点
│   ├── internal/
│   │   ├── adapter/           # 上游适配器（OpenAI/Anthropic/Gemini）
│   │   ├── billing/           # 计费引擎
│   │   ├── config/            # 配置加载
│   │   ├── database/          # DB/Redis 初始化、迁移
│   │   ├── handlers/          # HTTP 处理器
│   │   ├── middleware/        # 鉴权中间件
│   │   ├── models/            # GORM 模型
│   │   ├── monitoring/        # 监控与 Telegram 告警
│   │   └── upstream/          # 上游连接池
│   ├── migrations/            # SQL 迁移脚本
│   └── Dockerfile
├── frontend/
│   ├── admin/                 # 管理后台（Vue 3）
│   └── user/                  # 用户前台（Vue 3）
├── docker/
│   ├── postgres/init.sql      # 数据库初始化
│   └── ofelia/config.ini      # Cron 任务配置
├── scripts/
│   ├── backup.sh              # 数据库备份脚本
│   └── setup.sh               # VPS 一键部署脚本
├── backups/                   # 数据库备份文件
├── docker-compose.yml         # Docker Compose 配置
├── Caddyfile                  # Caddy 反向代理配置
├── .env.example               # 环境变量示例
└── Makefile                   # 项目管理命令
```

---

## 安全建议

1. **修改默认密码**：首次启动后立即修改管理员密码
2. **更新 JWT 密钥**：生产环境必须生成强随机 `JWT_SECRET`
3. **配置防火墙**：只开放必要端口（80, 443, 22）
4. **数据库端口**：绑定到 `127.0.0.1` 避免公网暴露
5. **定期更新**：保持 Docker 镜像和系统更新
6. **监控日志**：定期检查错误日志和安全事件
7. **敏感数据加密**：上游 API Key 在数据库中加密存储
8. **HTTPS 强制**：生产环境使用 Caddy 自动 HTTPS

---

## 阶段实施计划

本项目按阶段实施：

1. **阶段一**：项目骨架 — 初始化、Docker Compose、数据库迁移、ORM 模型 ✅
2. **阶段二**：核心网关 — OpenAI 兼容接口、上游适配器、负载均衡 ✅
3. **阶段三**：鉴权与计费 — 用户注册登录、API Key、Token 计量扣费 ✅
4. **阶段四**：钱包与支付 — 余额系统、支付宝/Stripe 支付 ✅
5. **阶段五**：管理后台 — Vue 3 管理面板、仪表盘、用户管理 ✅
6. **阶段六**：用户前台 — 用户面板、API Key 管理、Playground ✅
7. **阶段七**：运维 — 监控告警、自动备份、部署文档 ✅

---

## 合规声明

### 必须遵守的原则
1. **遵守上游 TOS**：确保具备合法分销/中转资质
2. **计费透明**：用户请求什么模型就用什么模型，严禁后台静默替换
3. **价格透明**：倍率明牌公开，不隐藏真实价格
4. **数据隐私**：用户对话内容严禁出售或共享给第三方
5. **明确退款规则**：在用户协议中写明退款政策

### 禁止的功能
- ❌ 破解 Claude Pro/ChatGPT Plus 网页端
- ❌ 模型暗换（用户付 Opus 价格，后台路由到 Sonnet）
- ❌ "虚拟刀"汇率戏法隐藏真实价格
- ❌ 出售用户聊天记录给模型厂商
- ❌ 绕过上游 TOS 的"账号池+共享 Cookie"方案

---

## VPS 部署步骤摘要

```bash
# 1. SSH 到服务器
ssh root@your-server-ip

# 2. 更新系统
apt update && apt upgrade -y

# 3. 安装 Docker
apt install -y docker.io docker-compose-plugin

# 4. 克隆项目
git clone <your-repo> /opt/ai-api-gateway
cd /opt/ai-api-gateway

# 5. 配置环境
cp .env.example .env
nano .env   # 修改 JWT_SECRET, ADMIN_PASSWORD 等

# 6. 构建前端
make build-all

# 7. 启动
docker compose up -d

# 8. 配置域名
# 编辑 Caddyfile，设置你的域名，取消生产配置注释

# 9. 重启 Caddy 获取 HTTPS
docker compose restart caddy
```

## 许可证

本项目采用 MIT 许可证。

## 免责声明

本项目为开源软件，使用者需自行承担风险。开发者不对因使用本项目造成的任何损失负责。使用 AI 服务请遵守相关法律法规和服务条款。
