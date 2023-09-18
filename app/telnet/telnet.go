package telnet

import (
	"fmt"
	"net"
	"time"
)

func New(user string, pass string, ip string) *Telnet {
	return &Telnet{
		user: user,
		pass: pass,
		ip:   ip,
	}
}

func (t *Telnet) PermTelnet() error {
	conn, err := net.Dial("tcp", t.ip+":telnet")
	if err != nil {
		return err
	}
	defer conn.Close()
	t.conn = conn

	if err := t.loginTelnet(); err != nil {
		return err
	}

	if err := t.modifyDB(); err != nil {
		return err
	}

	return nil
}

func (t *Telnet) loginTelnet() error {
	ctrl := "\r\n"
	cmd := []byte(t.user + ctrl + t.pass + ctrl)

	return t.sendCmd(cmd)
}

func (t *Telnet) modifyDB() error {

	prefix := "sendcmd 1 DB set TelnetCfg 0 "
	lanEnable := prefix + "Lan_Enable 1" + ctrl
	tsLanUser := prefix + "TSLan_UName root" + ctrl
	tsLanPwd := prefix + "TSLan_UPwd Zte521" + ctrl
	maxConn := prefix + "Max_Con_Num 3" + ctrl
	initSecLvl := prefix + "InitSecLvl 3" + ctrl

	save := "sendcmd 1 DB save" + ctrl

	cmd := []byte(lanEnable + tsLanUser + tsLanPwd + maxConn + initSecLvl + initSecLvl + save)
	if err := t.sendCmd(cmd); err != nil {
		return err
	}
	fmt.Println("Permanent Telnet succeed, wait reboot..")
	fmt.Println("user: root, pass: Zte521")
	time.Sleep(time.Second)

	return t.Reboot()
}

func (t *Telnet) sendCmd(cmd []byte) error {
	n, err := t.conn.Write(cmd)
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
	return t.sendCmd([]byte("reboot" + ctrl))
}
