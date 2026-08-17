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

// OpenTempTelnet walks every candidate client MAC, runs the webFac flow for
// each and verifies the granted temp credentials with a real telnet login.
// The HTTP flow returns credentials even for a MAC the device will not honor,
// so only a MAC whose credentials actually log in is accepted; when none do,
// the returned connection is nil and the error is the last failure.
func OpenTempTelnet(opts Options) (*telnet.Telnet, string, string, error) {
	fac := factory.New(opts.User, opts.Pass, opts.IP, opts.HTTPPort, opts.Iface, opts.Mac)

	macs, err := fac.ClientMACs()
	if err != nil {
		return nil, "", "", err
	}

	var lastErr error
	for _, mac := range macs {
		fmt.Println(strings.Repeat("-", 35))
		label := net.HardwareAddr(mac[:]).String()

		tlUser, tlPass, err := fac.HandleMAC(mac)
		if err != nil {
			lastErr = err
			fmt.Printf("[%s] factory flow failed: %v\n", label, err)
			continue
		}
		fmt.Printf("[%s] temp user: %s, pass: %s\n", label, tlUser, tlPass)

		t, err := telnet.New(tlUser, tlPass, opts.IP, opts.TelnetPort)
		if err != nil {
			lastErr = err
			fmt.Printf("[%s] telnet not reachable: %v\n", label, err)
			continue
		}
		if lerr := t.Login(); lerr != nil {
			lastErr = lerr
			t.Conn.Close()
			fmt.Printf("[%s] telnet verification failed: %v\n", label, lerr)
			continue
		}
		fmt.Println(strings.Repeat("-", 35))
		return t, tlUser, tlPass, nil
	}

	fmt.Println(strings.Repeat("-", 35))
	return nil, "", "", fmt.Errorf("temp telnet could not be verified with any MAC: %w", lastErr)
}
