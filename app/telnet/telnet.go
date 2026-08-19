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
	dialAttempts   = 5
	dialInterval   = 1 * time.Second
	readTimeout    = 10 * time.Second

	// The device only starts the actual shutdown a while after the reboot
	// command returns, so the connection must be held open until the device
	// closes it; closing the session ourselves can abort the reboot.
	rebootCloseTimeout = 12 * time.Second

	// The device drops the current session when telnetd is killed through the
	// program manager, so the connection must be held open until the close
	// announces that the kill took effect and pc has taken over respawning.
	restartCloseTimeout = 12 * time.Second
)

// ctrl terminates every command line sent to the device shell.
const ctrl = "\r\n"

// Telnet is an interactive shell connection to the ONU. The unexported user
// and pass are the credentials telnetd expects (see Login); Conn is the raw
// connection.
type Telnet struct {
	user string
	pass string
	Conn net.Conn
}

// shellPrompts are matched against device output to detect that a command has
// finished and the shell is ready for the next one.
var shellPrompts = []string{"#", "$"}

func New(user string, pass string, ip string, port int) (*Telnet, error) {
	return NewRetry(user, pass, ip, port, dialAttempts, dialInterval)
}

// NewRetry is New with a custom retry budget, used to verify a telnetd that has
// just been restarted in place: the pc supervisor can take a while to respawn
// the daemon, longer than the default budget covers.
func NewRetry(user string, pass string, ip string, port int, attempts int, interval time.Duration) (*Telnet, error) {
	var lastErr error
	for range attempts {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), connectTimeout)
		if err == nil {
			return &Telnet{
				user: user,
				pass: pass,
				Conn: conn,
			}, nil
		}
		lastErr = err
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("telnet service did not come up within %s: %w", time.Duration(attempts)*interval, lastErr)
}

// Login performs the interactive telnet login with the credentials the
// client was created with. It returns nil once the shell prompt is reached,
// so a nil return proves the credentials are currently accepted.
func (t *Telnet) Login() error {
	return t.loginTelnet()
}

// Solidify writes the permanent telnet settings to the device DB and saves
// them. The connection must already be logged in (see Login); each command is
// confirmed by the shell prompt, and the prompt after "DB save" means the
// flash write has finished.
func (t *Telnet) Solidify() error {
	return t.modifyDB()
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

// RestartTelnetd restarts the telnetd service in place through the device's
// program manager (`sendcmd -pc`): the running telnetd is killed and pc
// respawns it, which applies the currently saved DB settings without a reboot.
// Killing telnetd drops the current session, so like Reboot the connection is
// held open until the device closes it.
func (t *Telnet) RestartTelnetd() error {
	out, err := t.runOutput("sendcmd -pc show", readTimeout)
	if err != nil {
		return fmt.Errorf("could not read managed programs: %w", err)
	}
	pid, err := parseTelnetdPID(out)
	if err != nil {
		return err
	}
	if err := t.sendCmd(fmt.Sprintf("sendcmd -pc kill %d", pid)); err != nil {
		return err
	}
	return t.waitForClose(restartCloseTimeout)
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

// runOutput sends a shell command and returns its device output, with the
// echoed command and telnet control bytes stripped. Only output received after
// the command is returned; waiting for the echoed command text first makes the
// match robust against a leftover prompt from the previous command.
func (t *Telnet) runOutput(cmd string, timeout time.Duration) (string, error) {
	// drain any output left over from a previous command so a stale prompt
	// cannot satisfy the match before the echo arrives
	_ = t.Conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	drainBuf := make([]byte, 1024)
	for {
		_, err := t.Conn.Read(drainBuf)
		if err != nil {
			break
		}
	}

	if err := t.sendCmd(cmd); err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	var raw bytes.Buffer
	buf := make([]byte, 1024)
	for {
		out := filterTelnet(raw.Bytes())
		if strings.Contains(out, cmd) && matchAny(out, shellPrompts) {
			break
		}
		if !time.Now().Before(deadline) {
			return "", fmt.Errorf("timeout waiting for the result of %q: %q",
				cmd, truncate(out, 128))
		}
		_ = t.Conn.SetReadDeadline(deadline)
		n, err := t.Conn.Read(buf)
		if n > 0 {
			raw.Write(buf[:n])
		}
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return "", fmt.Errorf("%w (device output: %q)", err, truncate(filterTelnet(raw.Bytes()), 128))
		}
	}
	out := strings.TrimPrefix(filterTelnet(raw.Bytes()), cmd)
	out = strings.TrimLeft(out, "\r\n")
	return out, nil
}

// parseTelnetdPID extracts the current telnetd pid from a `sendcmd -pc show`
// table, which has the columns "Name APPID pid inst ...".
func parseTelnetdPID(out string) (int, error) {
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "telnetd" {
			pid, err := strconv.Atoi(f[2])
			if err != nil {
				return 0, fmt.Errorf("invalid telnetd pid %q: %w", f[2], err)
			}
			return pid, nil
		}
	}
	return 0, errors.New("telnetd not found in `sendcmd -pc show` output")
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
