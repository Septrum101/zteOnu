package factory

import (
	"errors"
	"fmt"
	"net"
)

// LocalMAC returns the hardware MAC address of the given local interface. It
// is an error when the interface is empty, does not exist, or has no 6-byte
// MAC.
func LocalMAC(iface string) ([6]byte, error) {
	if iface == "" {
		return [6]byte{}, errors.New("--iface is required")
	}
	return hardwareMAC(iface)
}

// ClientMAC returns the MAC used to derive the SendInfo payload: a custom
// --mac wins, then --iface, and finally the interface the route to the ONU
// picks, auto-detected by dialMAC.
func (f *Factory) ClientMAC() ([6]byte, error) {
	if f.mac != "" {
		hw, err := net.ParseMAC(f.mac)
		if err != nil {
			return [6]byte{}, fmt.Errorf("invalid MAC %q: %w", f.mac, err)
		}
		if len(hw) != 6 {
			return [6]byte{}, fmt.Errorf("MAC %q must be 6 bytes, got %d", f.mac, len(hw))
		}
		var m [6]byte
		copy(m[:], hw)
		return m, nil
	}
	if f.iface != "" {
		return LocalMAC(f.iface)
	}
	m, err := dialMAC(f.ip)
	if err != nil {
		return [6]byte{}, fmt.Errorf("neither --iface nor --mac given and auto-detection failed: %w", err)
	}
	return m, nil
}

// hardwareMAC returns the MAC of the interface as seen by net.Interfaces.
func hardwareMAC(name string) ([6]byte, error) {
	i, err := net.InterfaceByName(name)
	if err != nil {
		return [6]byte{}, err
	}
	if len(i.HardwareAddr) != 6 {
		return [6]byte{}, fmt.Errorf("interface %q has no usable 6-byte MAC address", name)
	}
	var m [6]byte
	copy(m[:], i.HardwareAddr)
	return m, nil
}

// dialMAC determines the interface that will carry traffic to dstIP by
// dialing a UDP socket (route lookup without sending any packet), reading the
// chosen source address and mapping it back to the owning interface's
// hardware MAC.
func dialMAC(dstIP string) ([6]byte, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(dstIP, "53"))
	if err != nil {
		return [6]byte{}, fmt.Errorf("no route to %s: %w", dstIP, err)
	}
	defer conn.Close()
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return [6]byte{}, errors.New("unexpected local address type from UDP dial")
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return [6]byte{}, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || !ipn.IP.Equal(local.IP) {
				continue
			}
			return hardwareMAC(i.Name)
		}
	}
	return [6]byte{}, fmt.Errorf("no interface owns the source address %s", local.IP)
}
