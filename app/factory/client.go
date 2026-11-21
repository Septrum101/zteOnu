package factory

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// Factory drives the reverse-engineered webFac HTTP flow of the ONU.
type Factory struct {
	user       string
	passwd     string
	ip         string
	port       int
	iface      string
	mac        string
	cli        *resty.Client
	key        []byte
	clientRand int      // rand sent by client in SendSq
	serverRand int      // rand/newrand received from device
	serverMac  [6]byte  // MAC received from device in V3/2026 protocols
}

// New builds a Factory for the given device and client settings. A non-empty
// mac is the only candidate used for the SendInfo payload (see ClientMAC).
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
