package utils

import "fmt"

// GoofishSupplierSign 闲管家"自研系统-货源方"接口签名
// sign = md5("app_id,app_secret,bodyMd5,timestamp,mch_id,mch_secret")
//   注意: 这跟 GoofishSign (订单推送 webhook) 是不同的算法
func GoofishSupplierSign(appID, appSecret, mchID, mchSecret, bodyJSON string, timestamp int64) string {
	if bodyJSON == "" {
		bodyJSON = "{}"
	}
	bodyMd5 := Md5Hex(bodyJSON)
	signStr := fmt.Sprintf("%s,%s,%s,%d,%s,%s", appID, appSecret, bodyMd5, timestamp, mchID, mchSecret)
	return Md5Hex(signStr)
}

func GoofishSupplierVerifySign(appID, appSecret, mchID, mchSecret, bodyJSON string, timestamp int64, providedSign string) bool {
	return GoofishSupplierSign(appID, appSecret, mchID, mchSecret, bodyJSON, timestamp) == providedSign
}
