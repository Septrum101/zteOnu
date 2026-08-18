package telnet

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestFilterTelnet(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain", []byte("Login: "), "Login: "},
		{"iac command", []byte{0xFF, 0xFD, 0x01, 'x'}, "x"},
		{"iac escaped", []byte{0xFF, 0xFF, 'y'}, "\xffy"},
		{"subnegotiation", []byte{0xFF, 0xFA, 1, 2, 3, 0xFF, 0xF0, 'z'}, "z"},
		{"truncated iac", []byte{'a', 0xFF}, "a"},
		{"truncated subnegotiation", []byte{'a', 0xFF, 0xFA, 1, 2}, "a"},
	}
	for _, c := range cases {
		if got := filterTelnet(c.in); got != c.want {
			t.Errorf("%s: filterTelnet(%v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestWaitForPrompt(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		c2.Write([]byte{0xFF, 0xFD, 0x01}) // IAC WILL Echo
		c2.Write([]byte("Welcome!\r\nSC_1"))
		time.Sleep(50 * time.Millisecond)
		c2.Write([]byte(".0# "))
	}()

	tl := &Telnet{Conn: c1}
	if err := tl.waitFor(2*time.Second, shellPrompts...); err != nil {
		t.Fatalf("waitFor: %v", err)
	}
}

func TestWaitForClose(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		c2.Write([]byte("saving..\r\n")) // some final output before the drop
		time.Sleep(50 * time.Millisecond)
		c2.Close() // device closes the connection
	}()

	tl := &Telnet{Conn: c1}
	if err := tl.waitForClose(2 * time.Second); err != nil {
		t.Fatalf("waitForClose: %v", err)
	}
}

func TestWaitForCloseTimeout(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		c2.Write([]byte("no shutdown yet\r\n"))
		time.Sleep(2 * time.Second)
	}()

	tl := &Telnet{Conn: c1}
	start := time.Now()
	err := tl.waitForClose(300 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "device did not close") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waitForClose returned late: %v", elapsed)
	}
}

func TestParseTelnetdPID(t *testing.T) {
	const show = `Name             APPID  pid   inst  StartedbyName    State    EchoMsg 
plugagent        172    3637  0     cspd_misc        1        1       
telnetd          61     4102  0     cspd_misc        1        1       
tr069d           168    2205  0     cspd_misc        1        1       
cspd             1      1627  0     pc               1        1       
`
	if got, err := parseTelnetdPID(show); err != nil || got != 4102 {
		t.Fatalf("parseTelnetdPID = %d, %v; want 4102, nil", got, err)
	}
}

func TestParseTelnetdPIDNotFound(t *testing.T) {
	if _, err := parseTelnetdPID("dnsmasq          0      3092  0     dns_mgr          1        1\n"); err == nil {
		t.Fatal("expected error for output without telnetd, got nil")
	}
}

func TestWaitForTimeout(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		c2.Write([]byte("Login: ")) // a prompt that never matches
		time.Sleep(2 * time.Second)
	}()

	tl := &Telnet{Conn: c1}
	start := time.Now()
	err := tl.waitFor(300*time.Millisecond, shellPrompts...)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("waitFor returned late: %v", elapsed)
	}
}
