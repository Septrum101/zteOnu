package factory

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/thank243/zteOnu/utils"
)

func New(user string, passwd string, ip string, port int) *Factory {
	return &Factory{
		user:   user,
		passwd: passwd,
		ip:     ip,
		port:   port,
		cli: resty.New().SetHeader("User-Agent", "curl/8.8.0-DEV").
			SetBaseURL(fmt.Sprintf("http://%s:%d", ip, port)),
	}
}

func (f *Factory) reset() error {
	// active onu web service first, increase the chances of success
	if _, err := f.cli.R().Get("/"); err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody("SendSq.gch").Post("webFac")
	if err != nil {
		return err
	}
	if resp.StatusCode() == 400 {
		return nil
	}

	return errors.New(resp.String())
}

func (f *Factory) reqFactoryMode() error {
	_, err := f.cli.R().SetBody("RequestFactoryMode.gch").Post("webFac")
	if err != nil {
		if err.(*url.Error).Err.Error() != "EOF" {
			return err
		}
	}
	return nil
}

func (f *Factory) sendSq() (uint8, error) {
	var version uint8

	r := time.Now().Second()
	resp, err := f.cli.R().SetBody(fmt.Sprintf("SendSq.gch?rand=%d\r\n", r)).Post("webFac")
	if err != nil {
		fmt.Println(err)
	}
	if resp.StatusCode() != 200 {
		return 0, errors.New(resp.String())
	}

	if strings.Contains(resp.String(), "newrand") {
		version = 2
		newRand, _ := strconv.Atoi(strings.ReplaceAll(resp.String(), "newrand=", ""))
		f.key = getKeyPool(version, r, newRand)
	} else if len(resp.String()) == 0 {
		version = 1
		f.key = getKeyPool(version, r, 0)
	} else {
		return 0, errors.New("unknown error")
	}

	return version, nil
}

func (f *Factory) checkLoginAuth() error {
	command := fmt.Sprintf("CheckLoginAuth.gch?&version61&user=%s&pass=%s", f.user, f.passwd)

	payload, err := utils.ECBEncrypt(
		[]byte(command), f.key)
	if err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		if _, err := utils.ECBDecrypt(resp.Body(), f.key); err != nil {
			return err
		}
		return nil
	case 400:
		return errors.New("unknown errors")
	case 401:
		return errors.New("errors user or password")
	default:
		return errors.New(resp.String())
	}
}

func (f *Factory) sendInfo() error {
	command := []byte("SendInfo.gch?info=12|")
	magicBytes, err := base64.StdEncoding.DecodeString(magicBytesBase64)
	if err != nil {
		return err
	}
	command = append(command, magicBytes...)

	payload, err := utils.ECBEncrypt(command, f.key)
	if err != nil {
		return err
	}
	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return err
	}

	switch resp.StatusCode() {
	case 200:
		return nil
	case 400:
		return errors.New("unknown errors")
	case 401:
		return errors.New("info error")
	default:
		return errors.New(resp.String())
	}
}

func (f *Factory) factoryMode() (user string, pass string, err error) {
	command := "FactoryMode.gch?mode=2&user=notused"

	payload, err := utils.ECBEncrypt([]byte(command), f.key)
	if err != nil {
		return
	}
	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return
	}

	dec, err := utils.ECBDecrypt(resp.Body(), f.key)
	if err != nil {
		return
	}

	u, err := url.Parse(string(dec))
	if err != nil {
		return
	}

	q := u.Query()
	user = q.Get("user")
	pass = q.Get("pass")

	return
}

func (f *Factory) handle() (tlUser string, tlPass string, err error) {
	fmt.Println(strings.Repeat("-", 35))

	fmt.Print("step [0] reset factory: ")
	if err = f.reset(); err != nil {
		return
	} else {
		fmt.Println("ok")
	}

	fmt.Print("step [1] request factory mode: ")
	if err = f.reqFactoryMode(); err != nil {
		return
	} else {
		fmt.Println("ok")
	}

	var ver uint8
	fmt.Print("step [2] send sq: ")
	ver, err = f.sendSq()
	if err != nil {
		return
	} else {
		fmt.Println("ok")
	}

	fmt.Print("step [3] check login auth: ")
	switch ver {
	case 1:
		if err = f.checkLoginAuth(); err != nil {
			return
		}
	case 2:
		if err = f.sendInfo(); err != nil {
			return
		}
		if err = f.checkLoginAuth(); err != nil {
			return
		}
	}
	fmt.Println("ok")

	fmt.Print("step [4] enter factory mode: ")
	tlUser, tlPass, err = f.factoryMode()
	if err != nil {
		return
	} else {
		fmt.Println("ok")
	}

	fmt.Println(strings.Repeat("-", 35))

	return
}

func (f *Factory) Handle() (tlUser string, tlPass string, err error) {
	count := 0
	for {
		tlUser, tlPass, err = f.handle()
		if err != nil {
			count++
			if count > 10 {
				return
			}
			fmt.Println(err, fmt.Sprintf("Attempt retrying..(%d/10)", count))
			time.Sleep(time.Millisecond * 500)
			continue
		}
		break
	}

	return
}

func getKeyPool(version uint8, r int, newR int) []byte {
	idx := r
	keyPool := AesKeyPool[idx : idx+24]
	if version == 2 {
		idx = ((0x1000193*r)&0x3F ^ newR) % 60
		keyPool = AesKeyPoolNew[idx : idx+24]
	}
	newKeyPool := make([]byte, len(keyPool))
	for i := range keyPool {
		newKeyPool[i] = (keyPool[i] ^ 0xA5) & 0xFF
	}

	return newKeyPool
}
