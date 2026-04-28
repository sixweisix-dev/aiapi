-- 订单意图区分: balance=普通充值, membership_pro=升级专业版, membership_enterprise=升级企业版
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS intent VARCHAR(50) NOT NULL DEFAULT 'balance';
