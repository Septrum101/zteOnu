package onu

import (
	"fmt"
	"net"
	"strings"

	"github.com/septrum101/zteOnu/app/factory"
	"github.com/septrum101/zteOnu/app/telnet"
)

// Options carries the device and client settings for OpenTempTelnet.
type Options struct {
	User       string
	Pass       string
	IP         string
	HTTPPort   int
	TelnetPort int
	Iface      string
	Mac        string
}

// OpenTempTelnet runs the webFac flow for the client MAC selected by --iface,
// --mac or the route-based auto-detection and verifies the granted temp
// credentials with a real telnet login. The HTTP flow returns credentials even
// when the MAC is not honored, so the run only succeeds if the credentials
// actually log in; on failure the returned connection is nil.
func OpenTempTelnet(opts Options) (*telnet.Telnet, string, string, error) {
	fac := factory.New(opts.User, opts.Pass, opts.IP, opts.HTTPPort, opts.Iface, opts.Mac)

	mac, err := fac.ClientMAC()
	if err != nil {
		return nil, "", "", err
	}
	label := net.HardwareAddr(mac[:]).String()

	fmt.Println(strings.Repeat("-", 35))
	tlUser, tlPass, err := fac.HandleMAC(mac)
	if err != nil {
		return nil, "", "", fmt.Errorf("[%s] factory flow failed: %w", label, err)
	}
	fmt.Printf("[%s] temp user: %s, pass: %s\n", label, tlUser, tlPass)

	t, err := telnet.New(tlUser, tlPass, opts.IP, opts.TelnetPort)
	if err != nil {
		return nil, "", "", fmt.Errorf("[%s] telnet not reachable: %w", label, err)
	}
	if lerr := t.Login(); lerr != nil {
		t.Conn.Close()
		return nil, "", "", fmt.Errorf("[%s] telnet verification failed: %w", label, lerr)
	}
	fmt.Println(strings.Repeat("-", 35))
	return t, tlUser, tlPass, nil
}
