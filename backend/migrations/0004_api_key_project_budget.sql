-- 项目级 API Key：增加项目名 + 预算管理
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS project_name VARCHAR(100);
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS monthly_budget DECIMAL(20,8);
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS budget_alert_pct INT NOT NULL DEFAULT 80;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS budget_used DECIMAL(20,8) NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS budget_period_start TIMESTAMP WITH TIME ZONE DEFAULT date_trunc('month', NOW());
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS budget_alerted BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_api_keys_budget_used ON api_keys(budget_used) WHERE monthly_budget IS NOT NULL;
