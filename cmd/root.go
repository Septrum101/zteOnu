package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/septrum101/zteOnu/app/onu"
	"github.com/septrum101/zteOnu/version"
)

var (
	// Used for flags.
	user          string
	passwd        string
	ip            string
	port          int
	telnet        bool // write permanent telnet settings, apply by in-place telnetd restart
	telnetRestart bool // write permanent telnet settings, apply by device reboot
	telnetPort    int
	iface         string
	mac           string

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
	rootCmd.PersistentFlags().BoolVar(&telnet, "telnet", false, "permanent telnet (user: root, pass: Zte521) applied by restarting the telnetd service in place, without rebooting; only applied after a temp telnet login is verified")
	rootCmd.PersistentFlags().BoolVar(&telnetRestart, "telnet-restart", false, "permanent telnet (user: root, pass: Zte521) applied by rebooting the device")
	rootCmd.PersistentFlags().IntVar(&telnetPort, "tp", 23, "ONU telnet port")
	rootCmd.PersistentFlags().StringVar(&iface, "iface", "", "network interface whose MAC to use (default: auto-detected from the route to the ONU)")
	rootCmd.PersistentFlags().StringVarP(&mac, "mac", "m", "", "custom client MAC address for the SendInfo payload (e.g. 00:07:29:55:35:57); overrides --iface and auto-detection")
}

func run() error {
	version.Show()

	if telnet && telnetRestart {
		return errors.New("--telnet (in-place restart) and --telnet-restart (reboot) are mutually exclusive")
	}

	t, _, _, err := onu.OpenTempTelnet(onu.Options{
		User:       user,
		Pass:       passwd,
		IP:         ip,
		HTTPPort:   port,
		TelnetPort: telnetPort,
		Iface:      iface,
		Mac:        mac,
	})
	if err != nil {
		return err
	}
	defer t.Conn.Close()
	fmt.Println("telnet verified, temp factory telnet is open")

	if telnet {
		return onu.SolidifyAndRestart(t, ip, telnetPort)
	}
	if telnetRestart {
		return onu.SolidifyAndReboot(t)
	}
	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
