# AI API Gateway Makefile

.PHONY: help start stop restart logs clean build migrate backup restore \
        ps health shell-db shell-redis test deploy cron-logs \
		setup log-rotate admin-build user-build build-all

help:
	@echo "AI API Gateway Management Commands:"
	@echo ""
	@echo "=== Service Lifecycle ==="
	@echo "  start         - Start all services (docker compose up -d)"
	@echo "  stop          - Stop all services (docker compose down)"
	@echo "  restart       - Restart all services"
	@echo "  logs          - View all service logs (docker compose logs -f)"
	@echo "  ps            - Show service status"
	@echo "  clean         - Stop services and remove volumes"
	@echo ""
	@echo "=== Build ==="
	@echo "  build         - Rebuild backend Docker image"
	@echo "  admin-build   - Build admin frontend"
	@echo "  user-build    - Build user frontend"
	@echo "  build-all     - Build all services"
	@echo ""
	@echo "=== Database ==="
	@echo "  migrate       - Run database migrations"
	@echo "  backup        - Create database backup"
	@echo "  restore       - Restore database from backup file"
	@echo "  shell-db      - Open PostgreSQL shell"
	@echo "  shell-redis   - Open Redis CLI"
	@echo ""
	@echo "=== Monitoring & Logs ==="
	@echo "  health        - Check service health"
	@echo "  cron-logs     - View cron (Ofelia) logs"
	@echo "  log-rotate    - Force Docker log rotation"
	@echo ""
	@echo "=== Development ==="
	@echo "  dev-backend   - Run backend locally (go run)"
	@echo "  dev-db        - Start only DB & Redis"
	@echo "  dev-logs      - Watch backend logs"
	@echo "  test          - Run Go tests"
	@echo ""
	@echo "=== Deployment ==="
	@echo "  setup         - Run server bootstrap script"
	@echo "  deploy        - Git pull, rebuild, restart"
	@echo ""

start:
	docker compose up -d

stop:
	docker compose down

restart: stop start

logs:
	docker compose logs -f

clean:
	docker compose down -v

build:
	docker compose build backend

build-admin:
	cd frontend/admin && npm install && npm run build

build-user:
	cd frontend/user && npm install && npm run build

build-all: build build-admin build-user
	docker compose build

migrate:
	@echo "Running database migrations..."
	docker compose exec backend ./main

backup:
	@echo "Creating database backup..."
	./scripts/backup.sh

health:
	@echo "Checking service health..."
	@curl -s http://localhost:8080/health || echo "API service is not responding"
	@docker compose ps

ps:
	docker compose ps

# Development commands
dev-backend:
	cd backend && go run cmd/api/main.go

dev-db:
	docker compose up -d postgres redis

dev-logs:
	docker compose logs -f backend

# Testing
test:
	cd backend && go test ./...

# Database management
db-shell:
	docker compose exec postgres psql -U postgres -d ai_gateway

redis-cli:
	docker compose exec redis redis-cli

# Production deployment
deploy:
	@echo "Deploying to production..."
	git pull
	docker compose pull
	docker compose up -d --build
	docker compose exec backend ./main
	@echo "Deployment completed!"

# Monitoring & Logs
cron-logs:
	docker compose logs -f ofelia

log-rotate:
	@echo "Forcing log rotation..."
	docker compose exec caddy sh -c "kill -USR1 1" 2>/dev/null || true

# Database restore (usage: make restore FILE=backup.sql)
restore:
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make restore FILE=backup.sql"; \
		exit 1; \
	fi
	cat $(FILE) | docker compose exec -T postgres psql -U postgres ai_gateway

# Server bootstrap
setup:
	@echo "Running server bootstrap..."
	sudo bash scripts/setup.sh