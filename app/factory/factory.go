package factory

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
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
		cli:    resty.New().SetBaseURL(fmt.Sprintf("http://%s:%d", ip, port)),
	}
}

func (f *Factory) Reset() error {
	resp, err := f.cli.R().SetBody("SendSq.gch").Post("webFac")
	if err != nil {
		return err
	}
	if resp.StatusCode() == 400 {
		return nil
	}

	return errors.New(resp.String())
}

func (f *Factory) ReqFactoryMode() error {
	_, err := f.cli.R().SetBody("RequestFactoryMode.gch").Post("webFac")
	if err != nil {
		if err.(*url.Error).Err.Error() != "EOF" {
			return err
		}
	}
	return nil
}

func (f *Factory) SendSq() (uint8, error) {
	var (
		keyPool []byte
		idx     int
		version uint8
	)

	r := rand.New(rand.NewSource(time.Now().Unix())).Intn(60)
	resp, err := f.cli.R().SetBody(fmt.Sprintf("SendSq.gch?rand=%d", r)).Post("webFac")
	if err != nil {
		fmt.Println(err)
	}
	if resp.StatusCode() != 200 {
		return 0, errors.New(resp.String())
	}

	if strings.Contains(resp.String(), "newrand") {
		keyPool = AesKeyPoolNew
		version = 2

		newRand, _ := strconv.Atoi(strings.ReplaceAll(resp.String(), "newrand=", ""))
		idx = ((0x1000193*r)&0x3F ^ newRand) % 60
	} else if len(resp.String()) == 0 {
		keyPool = AesKeyPool
		version = 1
	} else {
		return 0, errors.New("unknown error")
	}

	// Get keys
	pool := keyPool[idx : idx+24]
	f.Key = make([]byte, len(pool))
	for i := range pool {
		f.Key[i] = (pool[i] ^ 0xA5) & 0xFF
	}

	return version, nil
}

func (f *Factory) CheckLoginAuth() error {
	payload, err := utils.ECBEncrypt(
		[]byte(fmt.Sprintf("CheckLoginAuth.gch?version50&user=%s&pass=%s", f.user, f.passwd)), f.Key)
	if err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		if _, err := utils.ECBDecrypt(resp.Body(), f.Key); err != nil {
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

func (f *Factory) SendInfo() error {
	payload, err := utils.ECBEncrypt([]byte("SendInfo.gch?info=6|"), f.Key)
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

func (f *Factory) FactoryMode() (string, string, error) {
	payload, err := utils.ECBEncrypt([]byte("FactoryMode.gch?mode=2&user=notused"), f.Key)
	if err != nil {
		return "", "", err
	}
	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return "", "", err
	}

	dec, err := utils.ECBDecrypt(resp.Body(), f.Key)
	if err != nil {
		return "", "", err
	}
	u, err := url.ParseQuery(string(bytes.ReplaceAll(dec, []byte("FactoryModeAuth.gch?"), []byte(""))))
	if err != nil {
		return "", "", err
	}

	return u.Get("user"), u.Get("pass"), nil
}

func (f *Factory) Handle() (string, string, error) {
	fmt.Println(strings.Repeat("-", 35))

	fmt.Print("step [0] reset factory: ")
	if err := f.Reset(); err != nil {
		return "", "", fmt.Errorf("reset errors: %v\n", err)
	} else {
		fmt.Println("ok")
	}

	fmt.Print("step [1] request factory mode: ")
	if err := f.ReqFactoryMode(); err != nil {
		return "", "", err
	} else {
		fmt.Println("ok")
	}

	fmt.Print("step [2] send sq: ")
	ver, err := f.SendSq()
	if err != nil {
		return "", "", err
	} else {
		fmt.Println("ok")
	}

	fmt.Print("step [3] check login auth: ")
	switch ver {
	case 1:
		if err := f.CheckLoginAuth(); err != nil {
			return "", "", err
		}
	case 2:
		if err := f.SendInfo(); err != nil {
			return "", "", err
		}
		if err := f.CheckLoginAuth(); err != nil {
			return "", "", err
		}
	}
	fmt.Println("ok")

	fmt.Print("step [4] enter factory mode: ")
	tlUser, tlPass, err := f.FactoryMode()
	if err != nil {
		return "", "", err
	} else {
		fmt.Println("ok")
	}

	fmt.Println(strings.Repeat("-", 35))

	return tlUser, tlPass, nil
}
