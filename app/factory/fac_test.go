package factory

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stich86/zteOnu/utils"
)

func TestNewDev(t *testing.T) {
	key := getKeyPool(2, 17, 42)
	b, _ := utils.Base64Decrypt("KrGN86tQlVmegSE8nKnnXwqLhxbr4PeQIPZzqZhXUrLgifbGDNOzjPyBuAXSWfZUMUhlQKyGZ7GbxaqCYKV8hsUbCO1vI0BG8UGrb3cIGb0=", key)
	fmt.Println(string(b))
	bs := bytes.Split(b, []byte("|"))
	fmt.Println(base64.StdEncoding.EncodeToString(bs[1]))
}
