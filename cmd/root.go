package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"

	"github.com/stich86/zteOnu/app/factory"
	"github.com/stich86/zteOnu/app/telnet"
	"github.com/stich86/zteOnu/version"
)

var (
	// Used for flags.
	user       string
	passwd     string
	ip         string
	port       int
	permTelnet bool
	telnetPort int
	newMode    bool
	SecLvl	   int
	userList   []string
	passwdList []string
	defaultUsers = []string{"admin", "factorymode", "CMCCAdmin", "CUAdmin", "telecomadmin", "cqadmin", "user", "admin", "cuadmin", "lnadmin", "useradmin"}
	defaultPasswds = []string{"admin", "nE%jA@5b", "aDm8H%MdA", "CUAdmin", "nE7jA%5m", "cqunicom", "1620@CTCC", "1620@CUcc", "admintelecom", "cuadmin", "lnadmin"}

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
	rootCmd.PersistentFlags().StringVarP(&user, "user", "u", "", "factory mode auth username")
	rootCmd.PersistentFlags().StringVarP(&passwd, "pass", "p", "", "factory mode auth password")
	rootCmd.PersistentFlags().StringVarP(&ip, "ip", "i", "192.168.1.1", "ONU ip address")
	rootCmd.PersistentFlags().IntVar(&port, "port", 80, "ONU http port")
	rootCmd.PersistentFlags().BoolVar(&permTelnet, "telnet", false, "permanent telnet (user: root, pass: Zte521)")
	rootCmd.PersistentFlags().IntVar(&SecLvl, "seclvl", 2, "Security level for telnet access, if you got \"Permission Denied\", try 3.\nuse with --telnet flag")
	rootCmd.PersistentFlags().IntVar(&telnetPort, "tp", 23, "ONU telnet port")
	rootCmd.PersistentFlags().BoolVar(&newMode, "new", false, "use new method to open telnet, MAC address must set to 00:07:29:55:35:57")
}

func run() error {
	version.Show()

	if newMode {
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		magicMac, err := net.ParseMAC("00:07:29:55:35:57")
		if err != nil {
			return err
		}

		var isMagicMac bool
		for _, i := range interfaces {
			if i.HardwareAddr != nil && bytes.Equal(i.HardwareAddr, magicMac) {
				isMagicMac = true
				break
			}
		}

		if !isMagicMac {
			return errors.New("MAC address is not set to 00:07:29:55:35:57")
		}
	}

    // User default lists if user\pass not passed
    if user == "" {
        userList = defaultUsers
    } else {
        userList = []string{user}
    }
    if passwd == "" {
        passwdList = defaultPasswds
    } else {
        passwdList = []string{passwd} 
    }
    // Check list size
	if len(userList) != len(passwdList) {
		return errors.New("Users and Passwords list should have same lenght")
	}

    var tlUser string 
    var tlPass string

	for i := 0; i < len(userList); i++ {
		var err error
		tlUser, tlPass, err = factory.New(userList[i], passwdList[i], ip, port).Handle()
		if err != nil {
	            fmt.Printf("Cannot authenticate with user %s: %v\n", userList[i], err)
	            continue 
		}
		fmt.Printf("Success autheticated with user: %s\n", userList[i])
		break
	}

	if permTelnet {
		// create telnet conn
		t, err := telnet.New(tlUser, tlPass, ip, telnetPort)
		if err != nil {
			return err
		}
		defer t.Conn.Close()

		// handle permanent telnet
		if err := t.PermTelnet(SecLvl); err != nil {
			return err
		} else {
			fmt.Println("Permanent Telnet succeed\r\nUser: root, Pass: Zte521")
		}

		// reboot device
		fmt.Println("wait reboot..")
		time.Sleep(time.Second)
		if err := t.Reboot(); err != nil {
			return err
		}
	} else {
		if tlUser != "" && tlPass != "" {
 		   fmt.Printf("Temporary User: %s\nTemporary Pass: %s\n", tlUser, tlPass)
		}	
	}
	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
