-- ================================================================
-- Migration: 2026-07-11 支付FM 网关 + 缓存计费改造
-- ================================================================
-- 内容:
--   1. zhifux_orders 表 (支付FM 订单)
--   2. models 表加 cache_read_price / cache_write_price 字段
--   3. 软删除老 anthropic 直连模型
--   4. UPDATE claude-opus-4-6 / claude-sonnet-4-6 改到 group 9 (aitechflux)
--   5. INSERT 5 个新模型 (3 gpt-5.6 + claude-fable-5 + claude-sonnet-5)
--   6. UPDATE Aitechflux channel 的 supported_models
--   7. 填充所有 aitechflux 系模型的真实缓存价格
--   8. settings 表加首充设置
-- ================================================================

-- ============ 1. zhifux_orders 表 ============
CREATE TABLE IF NOT EXISTS zhifux_orders (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL,
  order_no VARCHAR(64) UNIQUE NOT NULL,
  platform_order_id VARCHAR(64),
  amount NUMERIC(10,2),
  pay_type VARCHAR(32),
  tier_id VARCHAR(32),
  status VARCHAR(20) DEFAULT 'pending',
  created_at TIMESTAMPTZ,
  paid_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_zhifux_orders_user ON zhifux_orders(user_id);

-- ============ 2. models 表加缓存价格字段 ============
ALTER TABLE models ADD COLUMN IF NOT EXISTS cache_read_price NUMERIC(20,8) DEFAULT 0 NOT NULL;
ALTER TABLE models ADD COLUMN IF NOT EXISTS cache_write_price NUMERIC(20,8) DEFAULT 0 NOT NULL;

-- ============ 3. 软删除老 anthropic 直连模型 ============
UPDATE models SET is_enabled=false, is_public=false, deleted_at=NOW()
WHERE name IN (
  'claude-3-5-haiku-20241022',
  'claude-3-5-sonnet-20241022',
  'claude-3-opus',
  'claude-3-opus-20240229',
  'claude-3-sonnet',
  'claude-sonnet-4-6-pro',
  'claude-opus-4-6-pro',
  'claude-opus-4-7-pro',
  'claude-sonnet-4-5-pro',
  'claude-haiku-4-5-20251001-pro'
);

-- ============ 3b. 恢复被早期软删的 claude-opus-4-6 / claude-sonnet-4-6 ============
UPDATE models SET deleted_at = NULL, updated_at = NOW()
WHERE name IN ('claude-opus-4-6', 'claude-sonnet-4-6');

-- ============ 4. UPDATE 现有 claude-opus-4-6 / claude-sonnet-4-6 改到 aitechflux ============
UPDATE models SET
  group_id = 9,
  provider = 'openai',
  upstream_name = 'claude-opus-4-6',
  input_price = 0.6 * 7 * 1.5 / 1000,
  output_price = 3.0 * 7 * 1.5 / 1000,
  cache_read_price = 0.068 * 7 * 1.5 / 1000,
  cache_write_price = 0.6 * 7 * 1.5 / 1000 * 1.25,
  multiplier = 1.0,
  updated_at = NOW()
WHERE name = 'claude-opus-4-6';

UPDATE models SET
  group_id = 9,
  provider = 'openai',
  upstream_name = 'claude-sonnet-4-6',
  input_price = 0.38 * 7 * 1.5 / 1000,
  output_price = 1.9 * 7 * 1.5 / 1000,
  cache_read_price = 0.068 * 7 * 1.5 / 1000,
  cache_write_price = 0.38 * 7 * 1.5 / 1000 * 1.25,
  multiplier = 1.0,
  updated_at = NOW()
WHERE name = 'claude-sonnet-4-6';

-- ============ 5. INSERT 5 个新模型 ============
INSERT INTO models (name, display_name, display_name_en, provider, context_length, input_price, output_price, cache_read_price, cache_write_price, multiplier, is_enabled, is_public, group_id, upstream_name, description, cost_per_call, created_at, updated_at)
VALUES (
  'claude-fable-5', 'Claude Fable 5', 'Claude Fable 5', 'openai', 200000,
  1.8 * 7 * 1.5 / 1000, 7.5 * 7 * 1.5 / 1000,
  0.18 * 7 * 1.5 / 1000, 1.8 * 7 * 1.5 / 1000 * 1.25,
  1.0, true, true, 9, 'claude-fable-5', 'Anthropic 最新旗舰模型', 0, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

INSERT INTO models (name, display_name, display_name_en, provider, context_length, input_price, output_price, cache_read_price, cache_write_price, multiplier, is_enabled, is_public, group_id, upstream_name, description, cost_per_call, created_at, updated_at)
VALUES (
  'claude-sonnet-5', 'Claude Sonnet 5', 'Claude Sonnet 5', 'openai', 200000,
  0.42 * 7 * 1.5 / 1000, 2.0 * 7 * 1.5 / 1000,
  0.068 * 7 * 1.5 / 1000, 0.42 * 7 * 1.5 / 1000 * 1.25,
  1.0, true, true, 9, 'claude-sonnet-5', 'Anthropic 新一代 Sonnet', 0, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

INSERT INTO models (name, display_name, display_name_en, provider, context_length, input_price, output_price, cache_read_price, cache_write_price, multiplier, is_enabled, is_public, group_id, upstream_name, description, cost_per_call, created_at, updated_at)
VALUES (
  'gpt-5.6-luna', 'GPT-5.6 Luna', 'GPT-5.6 Luna', 'openai', 400000,
  0.032 * 7 * 1.5 / 1000, 0.256 * 7 * 1.5 / 1000,
  0.018 * 7 * 1.5 / 1000, 0.042 * 7 * 1.5 / 1000,
  1.0, true, true, 9, 'gpt-5.6-luna', 'GPT-5.6 轻量版 (最经济)', 0, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

INSERT INTO models (name, display_name, display_name_en, provider, context_length, input_price, output_price, cache_read_price, cache_write_price, multiplier, is_enabled, is_public, group_id, upstream_name, description, cost_per_call, created_at, updated_at)
VALUES (
  'gpt-5.6-terra', 'GPT-5.6 Terra', 'GPT-5.6 Terra', 'openai', 400000,
  0.075 * 7 * 1.5 / 1000, 0.6 * 7 * 1.5 / 1000,
  0.048 * 7 * 1.5 / 1000, 0.1 * 7 * 1.5 / 1000,
  1.0, true, true, 9, 'gpt-5.6-terra', 'GPT-5.6 标准版', 0, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

INSERT INTO models (name, display_name, display_name_en, provider, context_length, input_price, output_price, cache_read_price, cache_write_price, multiplier, is_enabled, is_public, group_id, upstream_name, description, cost_per_call, created_at, updated_at)
VALUES (
  'gpt-5.6-sol', 'GPT-5.6 Sol', 'GPT-5.6 Sol', 'openai', 400000,
  0.148 * 7 * 1.5 / 1000, 1.184 * 7 * 1.5 / 1000,
  0.068 * 7 * 1.5 / 1000, 0.2 * 7 * 1.5 / 1000,
  1.0, true, true, 9, 'gpt-5.6-sol', 'GPT-5.6 旗舰版', 0, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;

-- ============ 6. UPDATE aitechflux 系原有模型的缓存价格 ============
UPDATE models SET
  cache_read_price = 0.068 * 7 * 1.5 / 1000,
  cache_write_price = 0.148 * 7 * 1.5 / 1000 * 1.25,
  updated_at = NOW()
WHERE name = 'codex-auto-review';

UPDATE models SET
  input_price = 0.138 * 7 * 1.5 / 1000,
  output_price = 0.828 * 7 * 1.5 / 1000,
  cache_read_price = 0.068 * 7 * 1.5 / 1000,
  cache_write_price = 0.138 * 7 * 1.5 / 1000 * 1.25,
  updated_at = NOW()
WHERE name = 'gpt-5.4';

UPDATE models SET
  input_price = 0.148 * 7 * 1.5 / 1000,
  output_price = 0.888 * 7 * 1.5 / 1000,
  cache_read_price = 0.068 * 7 * 1.5 / 1000,
  cache_write_price = 0.148 * 7 * 1.5 / 1000 * 1.25,
  updated_at = NOW()
WHERE name = 'gpt-5.5';

UPDATE models SET
  input_price = 0.086 * 7 * 1.5 / 1000,
  output_price = 0.99 * 7 * 1.5 / 1000,
  cache_read_price = 0.02 * 7 * 1.5 / 1000,
  cache_write_price = 0.15 * 7 * 1.5 / 1000,
  updated_at = NOW()
WHERE name = 'qwen-3-6-35b';

-- ============ 7. 其他模型缓存价格默认公式 (0.1x / 1.25x) ============
UPDATE models SET
  cache_read_price = input_price * 0.1,
  cache_write_price = input_price * 1.25
WHERE is_enabled = true AND cache_read_price = 0 AND cache_write_price = 0;

-- ============ 8. Aitechflux channel supported_models 追加新模型 ============
UPDATE upstream_channels SET
  supported_models = 'codex-auto-review,gpt-5.4,gpt-5.4-compact,gpt-5.5,gpt-5.5-compact,gpt-5.6-luna,gpt-5.6-terra,gpt-5.6-sol,claude-sonnet-4-6-hybrid,claude-sonnet-4-6,claude-opus-4-6,claude-sonnet-5,claude-fable-5,qwen-3-6-35b',
  updated_at = NOW()
WHERE name = 'Aitechflux';

-- ============ 9. Settings 首充设置 ============
INSERT INTO settings (key, value)
VALUES
  ('first_recharge_min_amount', '300'),
  ('first_recharge_bonus', '50')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
