-- 模型可选绑定固定渠道; 渠道可选显式 failover 列表

-- Model 加 upstream_channel_id (可空, 绑定到具体渠道)
ALTER TABLE models ADD COLUMN IF NOT EXISTS upstream_channel_id UUID REFERENCES upstream_channels(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_models_upstream_channel_id ON models(upstream_channel_id);

-- Channel 加 fallback_channel_ids (逗号分隔 UUID, 显式故障转移顺序)
ALTER TABLE upstream_channels ADD COLUMN IF NOT EXISTS fallback_channel_ids TEXT NOT NULL DEFAULT '';
