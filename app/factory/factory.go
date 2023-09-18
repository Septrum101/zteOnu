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

type Factory struct {
	user   string
	passwd string
	ip     string
	port   int
	Cli    *resty.Client
	Key    []byte
}

func New(user string, passwd string, ip string, port int) *Factory {
	return &Factory{
		user:   user,
		passwd: passwd,
		ip:     ip,
		port:   port,
		Cli:    resty.New().SetBaseURL(fmt.Sprintf("http://%s:%d", ip, port)),
	}
}

func (f *Factory) Reset() error {
	resp, err := f.Cli.R().SetBody("SendSq.gch").Post("webFac")
	if err != nil {
		return err
	}
	if resp.StatusCode() == 400 {
		return nil
	}

	return errors.New(resp.String())
}

func (f *Factory) ReqFactoryMode() error {
	_, err := f.Cli.R().SetBody("RequestFactoryMode.gch").Post("webFac")
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
	resp, err := f.Cli.R().SetBody(fmt.Sprintf("SendSq.gch?rand=%d", r)).Post("webFac")
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
	pool := keyPool[idx : idx+24]

	for i := range pool {
		f.Key = append(f.Key, (pool[i]^0xA5)&0xFF)
	}

	return version, nil
}

func (f *Factory) CheckLoginAuth() error {
	payload, err := utils.ECBEncrypt(
		[]byte(fmt.Sprintf("CheckLoginAuth.gch?version50&user=%s&pass=%s", f.user, f.passwd)), f.Key)
	if err != nil {
		return err
	}

	resp, err := f.Cli.R().SetBody(payload).Post("webFacEntry")
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
	resp, err := f.Cli.R().SetBody(payload).Post("webFacEntry")
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
	resp, err := f.Cli.R().SetBody(payload).Post("webFacEntry")
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
