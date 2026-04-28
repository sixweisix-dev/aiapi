-- 充值赠送活动相关字段
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_recharge_at TIMESTAMPTZ;
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS bonus_amount DECIMAL(20,8) NOT NULL DEFAULT 0;
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS upgrades_to_tier VARCHAR(50);

-- 默认阶梯规则 + 首充设置
INSERT INTO settings (key, value) VALUES
    ('recharge_tiers', '[{"min":100,"bonus":8},{"min":300,"bonus":30},{"min":500,"bonus":75},{"min":1000,"bonus":200},{"min":3000,"bonus":750}]'),
    ('first_recharge_bonus', '50'),
    ('recharge_promo_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
