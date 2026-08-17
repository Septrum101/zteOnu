package crypto

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestAES(t *testing.T) {
	// the string to be encrypted
	orig := "hello world"
	// the encryption key
	key := "1234567890123456"
	fmt.Println("plaintext: ", orig)

	encrypted, err := ECBEncrypt([]byte(orig), []byte(key))
	if err != nil {
		fmt.Println("encrypt error: ", err)
		return
	}
	fmt.Println("encrypted: ", hex.EncodeToString(encrypted))

	decrypted, err := ECBDecrypt(encrypted, []byte(key))
	if err != nil {
		fmt.Println("decrypt error: ", err)
		return
	}
	fmt.Println("decrypted: ", string(decrypted))
	if orig != string(decrypted) {
		t.Error("original string not equal decrypted string")
	}
}
