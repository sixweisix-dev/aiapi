package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

func Md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func GoofishSign(appKey, bodyJSON string, timestamp int64, appSecret string) string {
	if bodyJSON == "" {
		bodyJSON = "{}"
	}
	bodyMd5 := Md5Hex(bodyJSON)
	signStr := fmt.Sprintf("%s,%s,%d,%s", appKey, bodyMd5, timestamp, appSecret)
	return Md5Hex(signStr)
}

func GoofishVerifySign(appKey, bodyJSON string, timestamp int64, appSecret, providedSign string) bool {
	return GoofishSign(appKey, bodyJSON, timestamp, appSecret) == providedSign
}
