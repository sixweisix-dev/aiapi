-- 敏感词词库（动态维护，管理员可在后台管理）
CREATE TABLE IF NOT EXISTS sensitive_keywords (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    keyword TEXT NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL,  -- political / sexual / violence / jailbreak
    severity INT NOT NULL DEFAULT 1, -- 1=warn 2=block 3=blacklist
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sensitive_keywords_enabled ON sensitive_keywords(is_enabled) WHERE is_enabled = TRUE;

-- 默认词库（按类别）
INSERT INTO sensitive_keywords (keyword, category, severity) VALUES
-- 政治敏感（severity 3 = 直接拉黑，最严）
('习近平', 'political', 3),
('习总书记', 'political', 3),
('共产党', 'political', 3),
('中共', 'political', 3),
('六四', 'political', 3),
('天安门事件', 'political', 3),
('法轮功', 'political', 3),
('达赖', 'political', 3),
('港独', 'political', 3),
('台独', 'political', 3),
('疆独', 'political', 3),
('藏独', 'political', 3),
('反共', 'political', 3),
('翻墙', 'political', 2),

-- 色情（severity 3 = 直接拉黑）
('色情', 'sexual', 3),
('porn', 'sexual', 3),
('做爱', 'sexual', 3),
('性交', 'sexual', 3),
('裸照', 'sexual', 3),
('儿童色情', 'sexual', 3),
('child porn', 'sexual', 3),
('csam', 'sexual', 3),
('loli', 'sexual', 3),

-- 暴力犯罪指导（severity 3）
('炸弹制作', 'violence', 3),
('如何制造炸弹', 'violence', 3),
('how to make a bomb', 'violence', 3),
('枪支改装', 'violence', 3),
('毒品合成', 'violence', 3),
('制毒', 'violence', 3),
('how to kill', 'violence', 2),
('如何杀人', 'violence', 3),
('自杀方法', 'violence', 2),

-- 提示词注入/越狱（severity 1 = 警告 + 计数）
('ignore previous instructions', 'jailbreak', 1),
('ignore the above', 'jailbreak', 1),
('disregard previous', 'jailbreak', 1),
('忽略上述指令', 'jailbreak', 1),
('忽略之前的指令', 'jailbreak', 1),
('jailbreak mode', 'jailbreak', 1),
('developer mode', 'jailbreak', 1),
('DAN mode', 'jailbreak', 1),
('do anything now', 'jailbreak', 1)
ON CONFLICT (keyword) DO NOTHING;
