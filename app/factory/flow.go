package factory

import (
	"bytes"
	"encoding/binary"
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
	f.clientRand = r
	resp, err := f.cli.R().SetBody(fmt.Sprintf("SendSq.gch?rand=%d\r\n", r)).Post("webFac")
	if err != nil {
		return 0, err
	}
	if resp.StatusCode() != 200 {
		return 0, errors.New(resp.String())
	}

	body := resp.Body()
	bodyStr := resp.String()

	// V3: re_rand=N&M&<mac> (2024-2026 firmware)
	if strings.Contains(bodyStr, "re_rand=") {
		version = f.parseReRandResponse(body)
		return version, nil
	}

	// V2: newrand=N (2022 firmware)
	if strings.Contains(bodyStr, "newrand") {
		version = VersionV2
		newRand, _ := strconv.Atoi(strings.ReplaceAll(bodyStr, "newrand=", ""))
		f.serverRand = newRand
		f.key = getKeyPool(version, r, newRand)
		return version, nil
	}

	// V1: empty response (legacy firmware)
	if len(bodyStr) == 0 {
		version = VersionV1
		f.key = getKeyPool(version, r, 0)
		return version, nil
	}

	return 0, fmt.Errorf("unknown response format: %s", bodyStr)
}

// parseReRandResponse handles re_rand=N&M&<mac> format for V3/2026 protocols
func (f *Factory) parseReRandResponse(body []byte) uint8 {
	// Try 2026 format first: re_rand=N&M&<6 MAC bytes>
	if ver, ok := f.parse2026Response(body); ok {
		return ver
	}

	// Try 2024/2025 format: re_rand=N&M&<mac bytes>
	return f.parse2024Response(body)
}

func (f *Factory) parse2026Response(body []byte) (uint8, bool) {
	prefix := []byte("re_rand=")
	if !bytes.HasPrefix(body, prefix) {
		return 0, false
	}

	// Find the two & delimiters
	rest := body[len(prefix):]
	idx1 := bytes.IndexByte(rest, '&')
	if idx1 < 0 {
		return 0, false
	}
	idx2 := bytes.IndexByte(rest[idx1+1:], '&')
	if idx2 < 0 {
		return 0, false
	}
	idx2 += idx1 + 1

	// Parse server rand (first number after re_rand=)
	serverRandStr := string(rest[:idx1])
	serverRand, err := strconv.Atoi(serverRandStr)
	if err != nil {
		return 0, false
	}

	// Skip second number, get MAC bytes
	macStart := idx2 + 1
	if len(rest) < macStart+6 {
		return 0, false
	}

	f.serverRand = serverRand
	copy(f.serverMac[:], rest[macStart:macStart+6])
	f.key = getKeyPool(Version2026, f.clientRand, serverRand)

	return Version2026, true
}

func (f *Factory) parse2024Response(body []byte) uint8 {
	// Format: re_rand=N&M&<mac>
	idx1 := bytes.IndexByte(body, '&')
	idx2 := -1
	if idx1 >= 0 {
		idx2 = bytes.IndexByte(body[idx1+1:], '&')
		if idx2 >= 0 {
			idx2 += idx1 + 1
		}
	}

	if idx1 < 0 || idx2 < 0 {
		return VersionV3
	}

	// Parse re_rand value
	prefix := []byte("re_rand=")
	if bytes.HasPrefix(body, prefix) {
		randStr := string(body[len(prefix):idx1])
		if serverRand, err := strconv.Atoi(randStr); err == nil {
			f.serverRand = serverRand
		}
	}

	// Get MAC bytes after second &
	macStart := idx2 + 1
	if len(body) >= macStart+6 {
		copy(f.serverMac[:], body[macStart:macStart+6])
	}

	f.key = getKeyPool(VersionV3, f.clientRand, f.serverRand)
	return VersionV3
}

func (f *Factory) checkLoginAuth() error {
	return f.checkLoginAuthVersion("61", true)
}

func (f *Factory) checkLoginAuthVersion(version string, extraAmp bool) error {
	var command string
	if version != "" {
		amp := ""
		if extraAmp {
			amp = "&"
		}
		command = fmt.Sprintf("CheckLoginAuth.gch?%sversion%s&user=%s&pass=%s", amp, version, f.user, f.passwd)
	} else {
		command = fmt.Sprintf("CheckLoginAuth.gch?user=%s&pass=%s", f.user, f.passwd)
	}

	payload, err := crypto.ECBEncrypt([]byte(command), f.key)
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

// sendInfoV3 sends the SendInfo payload for V3 (2024/2025) protocol
func (f *Factory) sendInfoV3(clientMac [6]byte) error {
	payload := buildV3Payload(f.serverMac, clientMac, f.clientRand)
	command := append([]byte(fmt.Sprintf("SendInfo.gch?info=%d|", len(payload)/4)), payload...)

	encrypted, err := crypto.ECBEncrypt(padTo16(command), f.key)
	if err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody(encrypted).Post("webFacEntry")
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("sendInfoV3 failed: %d", resp.StatusCode())
	}
	return nil
}

// sendInfo2026 sends the SendInfo payload for 2026 protocol
func (f *Factory) sendInfo2026(clientMac [6]byte) error {
	payload := build2026Payload(f.serverMac, clientMac)
	command := append([]byte("SendInfo.gch?info=23|"), payload...)

	encrypted, err := crypto.ECBEncrypt(padTo16(command), f.key)
	if err != nil {
		return err
	}

	resp, err := f.cli.R().SetBody(encrypted).Post("webFacEntry")
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("sendInfo2026 failed: %d", resp.StatusCode())
	}
	return nil
}

// buildV3Payload creates the payload array for V3 protocol (2024/2025)
func buildV3Payload(serverMac, clientMac [6]byte, clientRand int) []byte {
	// Create payload array similar to CredentialsModuleCrypto.CreatePayloadArray
	values := make([]int, 0, 32)

	// Header
	values = append(values, 0, 1, 0, 2)

	// Server MAC bytes
	for _, b := range serverMac {
		values = append(values, int(b))
	}

	// Client MAC bytes
	for _, b := range clientMac {
		values = append(values, int(b))
	}

	// Trailer for verification
	values = append(values, 0, 255, 114, 70, 52, 17, 0)

	// Convert to bytes (little-endian int32 per value)
	payload := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(payload[i*4:], uint32(v))
	}

	return payload
}

// build2026Payload creates the payload for 2026 protocol
func build2026Payload(serverMac, clientMac [6]byte) []byte {
	values := make([]int, 0, 32)

	// Header
	values = append(values, 0, 1, 0, 2)

	// Server MAC bytes
	for _, b := range serverMac {
		values = append(values, int(b))
	}

	// Client MAC bytes
	for _, b := range clientMac {
		values = append(values, int(b))
	}

	// 2026 specific trailer
	values = append(values, Payload2026Trailer...)

	// Convert to bytes
	payload := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(payload[i*4:], uint32(v))
	}

	return payload
}

func padTo16(data []byte) []byte {
	padding := 16 - (len(data) % 16)
	if padding == 16 {
		return data
	}
	return append(data, make([]byte, padding)...)
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
	fmt.Printf("ok (version %d)\n", ver)

	fmt.Print("step [3] check login auth: ")
	switch ver {
	case VersionV1:
		if err = f.checkLoginAuthVersion("", false); err != nil {
			return
		}
	case VersionV2:
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
	case VersionV3:
		if mac == nil {
			err = errors.New("device requires a client MAC (SendInfo V3)")
			return
		}
		if err = f.sendInfoV3(*mac); err != nil {
			return
		}
		// Try different version strings
		if err = f.checkLoginAuthVersion("61", false); err != nil {
			if err = f.checkLoginAuthVersion("50", false); err != nil {
				return
			}
		}
	case Version2026:
		if mac == nil {
			err = errors.New("device requires a client MAC (SendInfo 2026)")
			return
		}
		if err = f.sendInfo2026(*mac); err != nil {
			return
		}
		if err = f.checkLoginAuthVersion("61", true); err != nil {
			if err = f.checkLoginAuthVersion("61", false); err != nil {
				if err = f.checkLoginAuthVersion("50", false); err != nil {
					return
				}
			}
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
