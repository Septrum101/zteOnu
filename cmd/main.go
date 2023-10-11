package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/thank243/zteOnu/app/factory"
	"github.com/thank243/zteOnu/app/telnet"
	"github.com/thank243/zteOnu/version"
)

func main() {
	version.Show()

	user := flag.String("u", "telecomadmin", "factory mode auth username")
	passwd := flag.String("p", "nE7jA%5m", "factory mode auth password")
	ip := flag.String("i", "192.168.1.1", "ONU ip address")
	port := flag.Int("port", 8080, "ONU http port")
	permTelnet := flag.Bool("telnet", false, "permanent telnet (user: root, pass: Zte521)")
	telnetPort := flag.Int("tp", 23, "ONU telnet port")
	flag.Parse()

	fac := factory.New(*user, *passwd, *ip, *port)

	tlUser, tlPass, err := fac.Handle()
	if err != nil {
		fmt.Println(err)
		return
	}

	if *permTelnet {
		// create telnet conn
		t, err := telnet.New(tlUser, tlPass, *ip, *telnetPort)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer t.Conn.Close()

		// handle permanent telnet
		if err := t.PermTelnet(); err != nil {
			fmt.Println(err)
			return
		} else {
			fmt.Println("Permanent Telnet succeed\r\nuser: root, pass: Zte521")
		}

		// reboot device
		fmt.Println("wait reboot..")
		time.Sleep(time.Second)
		if err := t.Reboot(); err != nil {
			fmt.Println(err)
			return
		}
	} else {
		fmt.Printf("user: %s\npass: %s", tlUser, tlPass)
	}
}
