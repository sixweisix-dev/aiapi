-- 会员等级系统：免费 / 专业 / 企业
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_tier VARCHAR(20) NOT NULL DEFAULT 'free';
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_expires_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_started_at TIMESTAMP WITH TIME ZONE;

-- 校验等级合法值
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_membership_tier') THEN
    ALTER TABLE users ADD CONSTRAINT chk_users_membership_tier 
      CHECK (membership_tier IN ('free', 'pro', 'enterprise'));
  END IF;
END $$;

-- 索引：定期扫到期的会员
CREATE INDEX IF NOT EXISTS idx_users_membership_expires 
  ON users(membership_expires_at) 
  WHERE membership_tier != 'free' AND membership_expires_at IS NOT NULL;

-- 充值订单加 tier 升级标记（用于审计 / 退款追溯）
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS upgrades_to_tier VARCHAR(20);
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS bonus_amount DECIMAL(20,8) DEFAULT 0;
