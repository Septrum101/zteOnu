package factory

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/septrum101/zteOnu/utils"
)

func New(user string, passwd string, ip string, port int, iface string, mac string) *Factory {
	return &Factory{
		user:   user,
		passwd: passwd,
		ip:     ip,
		port:   port,
		iface:  iface,
		mac:    mac,
		cli: resty.New().SetHeader("User-Agent", "curl/8.8.0-DEV").
			SetTimeout(10 * time.Second).
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

// LocalMACs returns the MAC addresses of all candidate local interfaces. If
// iface is non-empty only that interface is considered; otherwise every
// non-loopback interface with a 6-byte MAC qualifies. An error is returned
// when no usable MAC can be found.
func LocalMACs(iface string) ([][6]byte, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var macs [][6]byte
	seen := make(map[[6]byte]bool)
	for _, i := range ifaces {
		if iface != "" && i.Name != iface {
			continue
		}
		if iface == "" && i.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(i.HardwareAddr) == 6 {
			var m [6]byte
			copy(m[:], i.HardwareAddr)
			if !seen[m] {
				seen[m] = true
				macs = append(macs, m)
			}
		}
	}
	if len(macs) == 0 {
		if iface == "" {
			return nil, errors.New("no suitable network interface MAC address found")
		}
		return nil, fmt.Errorf("interface %q has no usable 6-byte MAC address", iface)
	}
	return macs, nil
}

// ClientMACs returns the candidate MAC addresses used to derive the SendInfo
// payload. If a custom MAC was supplied via New it is the only candidate;
// otherwise every local interface MAC (filtered by iface) qualifies.
func (f *Factory) ClientMACs() ([][6]byte, error) {
	if f.mac != "" {
		hw, err := net.ParseMAC(f.mac)
		if err != nil {
			return nil, fmt.Errorf("invalid MAC %q: %w", f.mac, err)
		}
		if len(hw) != 6 {
			return nil, fmt.Errorf("MAC %q must be 6 bytes, got %d", f.mac, len(hw))
		}
		var m [6]byte
		copy(m[:], hw)
		return [][6]byte{m}, nil
	}
	return LocalMACs(f.iface)
}

// macLabel renders a MAC for error messages.
func macLabel(mac [6]byte) string {
	return net.HardwareAddr(mac[:]).String()
}

// sendInfo tries every candidate client MAC until one is accepted by the
// device (HTTP 200). It only returns an error when all candidates have failed,
// since the interface MAC auto-detected may not be the one the device has
// associated with this client.
func (f *Factory) sendInfo() error {
	macs, err := f.ClientMACs()
	if err != nil {
		return err
	}

	var lastErr error
	for _, mac := range macs {
		command := []byte("SendInfo.gch?info=12|")
		command = append(command, MacToMagicBytes(mac)...)

		payload, err := utils.ECBEncrypt(command, f.key)
		if err != nil {
			return err
		}
		resp, err := f.cli.R().SetBody(payload).Post("webFacEntry")
		if err != nil {
			lastErr = fmt.Errorf("MAC %s: %w", macLabel(mac), err)
			continue
		}
		switch resp.StatusCode() {
		case 200:
			return nil
		case 400:
			lastErr = fmt.Errorf("MAC %s: unknown errors", macLabel(mac))
		case 401:
			lastErr = fmt.Errorf("MAC %s: info error", macLabel(mac))
		default:
			lastErr = fmt.Errorf("MAC %s: %s", macLabel(mac), resp.String())
		}
	}

	return fmt.Errorf("all %d candidate MACs rejected: %w", len(macs), lastErr)
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
	if resp.StatusCode() != 200 {
		return "", "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), resp.String())
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
	if user == "" || pass == "" {
		return "", "", fmt.Errorf("factory mode response carries no credentials: %q", string(dec))
	}

	return
}

func (f *Factory) handle() (tlUser string, tlPass string, err error) {
	fmt.Println(strings.Repeat("-", 35))

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
	}
	fmt.Println("ok")

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
