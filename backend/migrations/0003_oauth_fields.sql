-- OAuth 登录支持: 加 github_id / google_id 字段, 允许 password_hash 为空

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS github_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS google_id VARCHAR(64),
    ALTER COLUMN password_hash DROP NOT NULL;

-- 唯一索引(允许多个 null, 但同 provider id 只能对应一个用户)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id) WHERE github_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
