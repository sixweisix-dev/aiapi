-- Settings table for runtime-configurable system parameters
CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default values
INSERT INTO settings (key, value) VALUES
    ('signup_bonus', '5'),
    ('allow_registration', 'true'),
    ('announcement', '')
ON CONFLICT (key) DO NOTHING;
