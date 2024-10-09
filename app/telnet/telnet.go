package telnet

import (
	"fmt"
	"net"
	"strings"
	"strconv"
)

func New(user string, pass string, ip string, port int) (*Telnet, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	t := &Telnet{
		user: user,
		pass: pass,
		Conn: conn,
	}

	return t, nil
}

func (t *Telnet) PermTelnet(SecLvl int) error {
	if err := t.loginTelnet(); err != nil {
		return err
	}

	if err := t.modifyDB(SecLvl); err != nil {
		return err
	}

	if err := t.modifyFW(); err != nil {
		return err
	}

	return nil
}

func (t *Telnet) loginTelnet() error {
	return t.sendCmd(t.user, t.pass)
}

func (t *Telnet) modifyDB(SecLvl int) error {
	// set DB data
	prefix := "sendcmd 1 DB set TelnetCfg 0 "
	lanEnable := prefix + "Lan_Enable 1"
	tsLanUser := prefix + "TSLan_UName root"
	tsLanPwd := prefix + "TSLan_UPwd Zte521"
	maxConn := prefix + "Max_Con_Num 3"
	initSecLvl := prefix + "InitSecLvl " + strconv.Itoa(SecLvl)

	// save DB
	save := "sendcmd 1 DB save"

	if err := t.sendCmd(lanEnable, tsLanUser, tsLanPwd, maxConn, initSecLvl, save); err != nil {
		return err
	}

	return nil
}

func (t *Telnet) modifyFW() error {
	// set DB data
	prefix := "sendcmd 1 DB addr FWSC 0"
	viewName := prefix + "ViewName IGD.FWSc.FWSC1"
	enable := prefix + "Enable 1"
	intName := prefix + "INCName LAN"
	intViewName := prefix + "INCViewName IGD.LD1"
	service := prefix + "Servise 8"
	filter := prefix + "FilterTarget 1"

	// save DB
	save := "sendcmd 1 DB save"

	if err := t.sendCmd(prefix, viewName, enable, intName, intViewName, service, filter, save); err != nil {
		return err
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

func (t *Telnet) Reboot() error {
	return t.sendCmd("reboot")
}
