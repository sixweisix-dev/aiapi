package models

import (
"time"
)

// GoofishOrder 记录从闲管家 webhook 收到的订单推送
type GoofishOrder struct {
ID           uint      `gorm:"primaryKey" json:"id"`
OrderNo      string    `gorm:"column:order_no;uniqueIndex;size:64;not null" json:"order_no"`
SellerID     int64     `gorm:"column:seller_id" json:"seller_id"`
UserName     string    `gorm:"column:user_name;size:64" json:"user_name"`
OrderType    int       `gorm:"column:order_type;index" json:"order_type"`         // 1普通/7卡密/8直充
OrderStatus  int       `gorm:"column:order_status;index" json:"order_status"`     // 订单状态
RefundStatus int       `gorm:"column:refund_status;default:0" json:"refund_status"`
ProductID    int64     `gorm:"column:product_id" json:"product_id"`               // 管家商品ID
ItemID       int64     `gorm:"column:item_id" json:"item_id"`                     // 闲鱼商品ID
ModifyTime   int64     `gorm:"column:modify_time" json:"modify_time"`             // 闲管家更新时间戳(秒)
RawPayload   string    `gorm:"column:raw_payload;type:text" json:"raw_payload"`   // 完整原文(便于调试)
RedeemCode   string    `gorm:"column:redeem_code;size:64" json:"redeem_code"`     // 关联的充值码
ProcessedAt  *time.Time `gorm:"column:processed_at" json:"processed_at"`          // 处理时间
CreatedAt    time.Time `json:"created_at"`
UpdatedAt    time.Time `json:"updated_at"`
}

func (GoofishOrder) TableName() string {
return "goofish_orders"
}
