package utils

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestAES(t *testing.T) {
	// 需要加密的字符串
	orig := "hello world"
	// 加密密钥
	key := "1234567890123456"
	fmt.Println("原文: ", orig)

	encrypted, err := ECBEncrypt([]byte(orig), []byte(key))
	if err != nil {
		fmt.Println("加密出错: ", err)
		return
	}
	fmt.Println("加密后: ", hex.EncodeToString(encrypted))

	decrypted, err := ECBDecrypt(encrypted, []byte(key))
	if err != nil {
		fmt.Println("解密出错: ", err)
		return
	}
	fmt.Println("解密后: ", string(decrypted))
}
