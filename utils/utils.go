package utils

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
)

func ECBEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	origData = padding(origData, block.BlockSize())
	encrypted := make([]byte, len(origData))
	// 对每个block进行加密
	for i := 0; i < len(origData); i += block.BlockSize() {
		block.Encrypt(encrypted[i:i+block.BlockSize()], origData[i:i+block.BlockSize()])
	}
	return encrypted, nil
}

func ECBDecrypt(encrypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// padding bytes
	if len(encrypted)%16 != 0 {
		encrypted = padding(encrypted, block.BlockSize())
	}

	origData := make([]byte, len(encrypted))
	// 对每个block进行解密
	for i := 0; i < len(encrypted); i += block.BlockSize() {
		block.Decrypt(origData[i:i+block.BlockSize()], encrypted[i:i+block.BlockSize()])
	}
	origData = unPadding(origData)
	return origData, nil
}

func padding(origData []byte, blockSize int) []byte {
	padding := blockSize - len(origData)%blockSize
	padText := bytes.Repeat([]byte{0}, padding)
	return append(origData, padText...)
}

func unPadding(origData []byte) []byte {
	return bytes.TrimRight(origData, "\x00")
}

func Base64Decrypt(b64 string, key []byte) ([]byte, error) {
	encrypted, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}

	decrypted, err := ECBDecrypt(encrypted, key)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}
