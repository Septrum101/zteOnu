package telnet

import (
	"net"
)

const ctrl = "\r\n"

type Telnet struct {
	user string
	pass string
	ip   string
	conn net.Conn
}
