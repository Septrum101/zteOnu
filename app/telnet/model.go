package telnet

import (
	"net"
)

const ctrl = "\r\n"

type Telnet struct {
	user string
	pass string
	Conn net.Conn
}
