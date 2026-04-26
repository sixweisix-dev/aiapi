-- 违规追踪 + 黑名单
ALTER TABLE users ADD COLUMN IF NOT EXISTS violation_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS blacklist_reason TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS blacklisted_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_users_violation_count ON users(violation_count) WHERE violation_count > 0;

-- 违规日志表（审计追溯）
CREATE TABLE IF NOT EXISTS violation_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    api_key_id UUID REFERENCES api_keys(id),
    violation_type VARCHAR(50) NOT NULL,
    matched_keyword TEXT,
    request_snippet TEXT,
    ip_address INET,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_violation_logs_user_id ON violation_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_violation_logs_created_at ON violation_logs(created_at);
