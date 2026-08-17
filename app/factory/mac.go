package factory

import (
	"errors"
	"fmt"
	"net"
)

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
