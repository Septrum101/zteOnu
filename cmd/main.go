package main

import (
	"flag"
	"fmt"

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
	permTelnet := flag.Bool("telnet", false, "Permanent telnet (user: root, pass: Zte521)")
	flag.Parse()

	fac := factory.New(*user, *passwd, *ip, *port)

	tlUser, tlPass, err := fac.Handle()
	if err != nil {
		fmt.Println(err)
		return
	}

	if *permTelnet {
		t := telnet.New(tlUser, tlPass, *ip)
		if err := t.PermTelnet(); err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Printf("user: %s\npass: %s", tlUser, tlPass)
	}
}
