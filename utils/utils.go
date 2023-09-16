package utils

import (
	"bytes"
	"crypto/aes"
)

func ECBEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	origData = PKCS5Padding(origData, block.BlockSize())
	crypted := make([]byte, len(origData))
	// 对每个block进行加密
	for i := 0; i < len(origData); i += block.BlockSize() {
		block.Encrypt(crypted[i:i+block.BlockSize()], origData[i:i+block.BlockSize()])
	}
	return crypted, nil
}

func ECBDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	origData := make([]byte, len(crypted))
	// 对每个block进行解密
	for i := 0; i < len(crypted); i += block.BlockSize() {
		block.Decrypt(origData[i:i+block.BlockSize()], crypted[i:i+block.BlockSize()])
	}
	origData = PKCS5UnPadding(origData)
	return origData, nil
}

func PKCS5Padding(origData []byte, blockSize int) []byte {
	padding := blockSize - len(origData)%blockSize
	padtext := bytes.Repeat([]byte{0}, padding)
	return append(origData, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	return bytes.TrimRight(origData, "\x00")
}
