package factory

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/septrum101/zteOnu/app/crypto"
)

func (f *Factory) reset() error {
	// active onu web service first, increase the chances of success
	if _, err := f.cli.R().Get("/"); err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody("SendSq.gch").Post("webFac")
	if err != nil {
		return err
	}
	// 400 means the stale session was reset; when the device is already in a
	// factory session it answers 200 with an empty body, which is equally fine.
	if resp.StatusCode() == 400 || (resp.StatusCode() == 200 && resp.String() == "") {
		return nil
	}

	return errors.New(resp.String())
}

func (f *Factory) reqFactoryMode() error {
	_, err := f.cli.R().SetBody("RequestFactoryMode.gch").Post("webFac")
	// The device accepts the request by closing the connection, which surfaces
	// as an EOF error; any other transport failure is real.
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (f *Factory) sendSq() (uint8, error) {
	var version uint8

	r := time.Now().Second()
	resp, err := f.cli.R().SetBody(fmt.Sprintf("SendSq.gch?rand=%d\r\n", r)).Post("webFac")
	if err != nil {
		return 0, err
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

	payload, err := crypto.ECBEncrypt(
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
		if _, err := crypto.ECBDecrypt(resp.Body(), f.key); err != nil {
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

// sendInfo sends the SendInfo payload for a single candidate MAC; the device
// answers HTTP 200 only for a MAC it associates with this client.
func (f *Factory) sendInfo(mac [6]byte) error {
	command := []byte("SendInfo.gch?info=12|")
	command = append(command, MacToMagicBytes(mac)...)

	payload, err := crypto.ECBEncrypt(command, f.key)
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

	payload, err := crypto.ECBEncrypt([]byte(command), f.key)
	if err != nil {
		return
	}
	resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
	if err != nil {
		return
	}
	if resp.StatusCode() != 200 {
		return "", "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), resp.String())
	}

	dec, err := crypto.ECBDecrypt(resp.Body(), f.key)
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
	if user == "" || pass == "" {
		return "", "", fmt.Errorf("factory mode response carries no credentials: %q", string(dec))
	}

	return
}

func (f *Factory) handle(mac *[6]byte) (tlUser string, tlPass string, err error) {
	fmt.Print("step [0] reset factory: ")
	if err = f.reset(); err != nil {
		return
	}
	fmt.Println("ok")

	fmt.Print("step [1] request factory mode: ")
	if err = f.reqFactoryMode(); err != nil {
		return
	}
	fmt.Println("ok")

	var ver uint8
	fmt.Print("step [2] send sq: ")
	ver, err = f.sendSq()
	if err != nil {
		return
	}
	fmt.Println("ok")

	fmt.Print("step [3] check login auth: ")
	switch ver {
	case 1:
		if err = f.checkLoginAuth(); err != nil {
			return
		}
	case 2:
		if mac == nil {
			err = errors.New("device requires a client MAC (SendInfo)")
			return
		}
		if err = f.sendInfo(*mac); err != nil {
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
		fmt.Println("fail")
		return
	}
	fmt.Println("ok")
	return
}

// HandleMAC runs the full webFac flow with the given candidate MAC used for
// the SendInfo payload and returns the granted temp telnet credentials. The
// HTTP flow returns credentials even for a MAC the device will not honor over
// telnet, so the caller must verify each result with an actual telnet login
// and fall through to the next candidate MAC when it fails.
func (f *Factory) HandleMAC(mac [6]byte) (tlUser string, tlPass string, err error) {
	return f.handle(&mac)
}
