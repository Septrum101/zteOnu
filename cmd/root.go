package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thank243/zteOnu/app/factory"
	"github.com/thank243/zteOnu/app/telnet"
	"github.com/thank243/zteOnu/version"
)

var (
	// Used for flags.
	user       string
	passwd     string
	ip         string
	port       int
	permTelnet bool
	telnetPort int

	rootCmd = &cobra.Command{
		Use: "zteOnu",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				fmt.Println(err)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&user, "user", "u", "telecomadmin", "factory mode auth username")
	rootCmd.PersistentFlags().StringVarP(&passwd, "pass", "p", "nE7jA%5m", "factory mode auth password")
	rootCmd.PersistentFlags().StringVarP(&ip, "ip", "i", "192.168.1.1", "ONU ip address")
	rootCmd.PersistentFlags().IntVar(&port, "port", 8080, "ONU http port")
	rootCmd.PersistentFlags().BoolVar(&permTelnet, "telnet", false, "permanent telnet (user: root, pass: Zte521)")
	rootCmd.PersistentFlags().IntVar(&port, "tp", 23, "ONU telnet port")
}

func run() error {
	version.Show()

	tlUser, tlPass, err := factory.New(user, passwd, ip, port).Handle()
	if err != nil {
		return err
	}

	if permTelnet {
		// create telnet conn
		t, err := telnet.New(tlUser, tlPass, ip, telnetPort)
		if err != nil {
			return err
		}
		defer t.Conn.Close()

		// handle permanent telnet
		if err := t.PermTelnet(); err != nil {
			return err
		} else {
			fmt.Println("Permanent Telnet succeed\r\nuser: root, pass: Zte521")
		}

		// reboot device
		fmt.Println("wait reboot..")
		time.Sleep(time.Second)
		if err := t.Reboot(); err != nil {
			return err
		}
	} else {
		fmt.Printf("user: %s\npass: %s", tlUser, tlPass)
	}

	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
