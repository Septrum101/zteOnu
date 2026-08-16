package telnet

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// The factory telnet listener on some firmwares only comes up minutes
	// after the webFac flow returns, so keep dialing for a while instead of
	// failing on the first refused/filtered attempt.
	connectTimeout = 3 * time.Second
	dialAttempts   = 30
	dialInterval   = 1 * time.Second
	readTimeout    = 10 * time.Second

	// The device only starts the actual shutdown a while after the reboot
	// command returns, so the connection must be held open until the device
	// closes it; closing the session ourselves can abort the reboot.
	rebootCloseTimeout = 12 * time.Second
)

// shellPrompts are matched against device output to detect that a command has
// finished and the shell is ready for the next one.
var shellPrompts = []string{"#", "$"}

func New(user string, pass string, ip string, port int) (*Telnet, error) {
	var lastErr error
	for range dialAttempts {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), connectTimeout)
		if err == nil {
			return &Telnet{
				user: user,
				pass: pass,
				Conn: conn,
			}, nil
		}
		lastErr = err
		time.Sleep(dialInterval)
	}
	return nil, fmt.Errorf("telnet service did not come up within %s: %w", dialAttempts*dialInterval, lastErr)
}

func (t *Telnet) PermTelnet() error {
	if err := t.loginTelnet(); err != nil {
		return err
	}

	if err := t.modifyDB(); err != nil {
		return err
	}

	return nil
}

func (t *Telnet) loginTelnet() error {
	// "ogin:"/"sername:" cover both the "Login:" and "Username:" spellings
	if err := t.waitFor(readTimeout, "ogin:", "sername:"); err != nil {
		return fmt.Errorf("no login prompt: %w", err)
	}
	if err := t.sendCmd(t.user); err != nil {
		return err
	}
	if err := t.waitFor(readTimeout, "assword:"); err != nil {
		return fmt.Errorf("no password prompt: %w", err)
	}
	if err := t.sendCmd(t.pass); err != nil {
		return err
	}
	// A rejected login either re-prompts for the username or prints an error
	// and drops the connection, so waiting for the shell prompt is also the
	// login check.
	if err := t.waitFor(readTimeout, shellPrompts...); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	return nil
}

func (t *Telnet) modifyDB() error {
	// set DB data
	prefix := "sendcmd 1 DB set TelnetCfg 0 "
	commands := []string{
		prefix + "Lan_Enable 1",
		prefix + "TS_UName root",
		prefix + "TS_UPwd Zte521",
		prefix + "TSLan_UName root",
		prefix + "TSLan_UPwd Zte521",
		prefix + "Max_Con_Num 3",
		prefix + "InitSecLvl 3",
	}

	// save DB
	commands = append(commands, "sendcmd 1 DB save")

	// Commands are sent one at a time, each confirmed by the shell prompt
	// before the next one. The prompt after "DB save" means the flash write
	// has finished, which is what makes the later reboot safe.
	for _, c := range commands {
		if err := t.sendCmd(c); err != nil {
			return err
		}
		if err := t.waitFor(readTimeout, shellPrompts...); err != nil {
			return fmt.Errorf("command %q failed: %w", c, err)
		}
	}

	return nil
}

func (t *Telnet) sendCmd(commands ...string) error {
	cmd := []byte(strings.Join(commands, ctrl) + ctrl)
	n, err := t.Conn.Write(cmd)
	if err != nil {
		return err
	}

	if expected, actual := len(cmd), n; expected != actual {
		err := fmt.Errorf("transmission problem: tried sending %d bytes, but actually only sent %d bytes", expected, actual)
		return err
	}

	return nil
}

// Reboot sends the reboot command and blocks until the device closes the
// connection, which is how the shutdown announces itself. Closing the session
// ourselves right after the command can abort the reboot before it starts.
func (t *Telnet) Reboot() error {
	if err := t.sendCmd("reboot"); err != nil {
		return err
	}
	return t.waitForClose(rebootCloseTimeout)
}

// waitForClose reads until the peer closes the connection (EOF or a reset,
// both mean the device is going down) or the timeout elapses.
func (t *Telnet) waitForClose(timeout time.Duration) error {
	if err := t.Conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	buf := make([]byte, 1024)
	for {
		_, err := t.Conn.Read(buf)
		if err == nil {
			continue // device is still sending; keep waiting for the close
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return fmt.Errorf("device did not close the connection within %s; the reboot may not have started", timeout)
		}
		return nil
	}
}

// waitFor reads device output until one of the patterns appears or the timeout
// elapses. Only output received during this call is considered, and telnet
// control sequences are stripped before matching.
func (t *Telnet) waitFor(timeout time.Duration, patterns ...string) error {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 1024)
	var raw bytes.Buffer

	for {
		out := filterTelnet(raw.Bytes())
		if matchAny(out, patterns) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timeout waiting for %s, device output: %q",
				strings.Join(patterns, " or "), truncate(out, 128))
		}
		_ = t.Conn.SetReadDeadline(deadline)
		n, err := t.Conn.Read(buf)
		if n > 0 {
			raw.Write(buf[:n])
		}
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				// loop once more so data read together with the timeout is
				// still checked against the patterns
				continue
			}
			return fmt.Errorf("%w (device output: %q)", err, truncate(filterTelnet(raw.Bytes()), 128))
		}
	}
}

func matchAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Telnet in-band control bytes (RFC 854).
const (
	telnetIAC  = 0xFF
	telnetSB   = 0xFA
	telnetSE   = 0xF0
	telnetWILL = 0xFB
	telnetDONT = 0xFE
)

// filterTelnet drops telnet negotiation commands from raw device output so
// they cannot collide with the prompt patterns. It handles the cases these
// devices actually emit: 2-byte IAC commands, escaped 0xFF data, 3-byte
// WILL/WONT/DO/DONT commands and IAC SB ... IAC SE subnegotiations.
func filterTelnet(in []byte) string {
	var out []byte
	for i := 0; i < len(in); {
		if in[i] != telnetIAC {
			out = append(out, in[i])
			i++
			continue
		}
		if i+1 >= len(in) {
			break // truncated sequence
		}
		switch in[i+1] {
		case telnetIAC:
			out = append(out, telnetIAC)
			i += 2
		case telnetSB:
			j := i + 2
			for j+1 < len(in) && !(in[j] == telnetIAC && in[j+1] == telnetSE) {
				j++
			}
			i = j + 2 // past IAC SE, or past the end
		default:
			if in[i+1] >= telnetWILL && in[i+1] <= telnetDONT {
				i += 3 // 3-byte WILL/WONT/DO/DONT command
			} else {
				i += 2
			}
		}
	}
	return string(out)
}
