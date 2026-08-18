package onu

import (
	"fmt"
	"time"

	"github.com/septrum101/zteOnu/app/telnet"
)

// SolidifyAndReboot writes the permanent telnet settings on t - which must
// already be logged in, see OpenTempTelnet - and reboots the device. The
// reboot is safe because Solidify waits for the shell prompt after "DB save",
// i.e. the flash write has completed.
func SolidifyAndReboot(t *telnet.Telnet) error {
	if err := t.Solidify(); err != nil {
		return err
	}
	fmt.Println("Permanent Telnet succeed\r\nuser: root, pass: Zte521")

	fmt.Println("wait reboot..")
	if err := t.Reboot(); err != nil {
		return err
	}
	fmt.Println("device is rebooting")
	return nil
}

// SolidifyAndRestart writes the permanent telnet settings on t - which must
// already be logged in, see OpenTempTelnet - and applies them in place by
// restarting the telnetd service through the device's program manager, without
// rebooting. Restarting drops the current session, so the result is verified
// with a fresh login using the permanent credentials.
func SolidifyAndRestart(t *telnet.Telnet, ip string, telnetPort int) error {
	if err := t.Solidify(); err != nil {
		return err
	}
	fmt.Println("Permanent Telnet saved\r\nuser: root, pass: Zte521")

	fmt.Println("restarting telnetd in place (no reboot)..")
	if err := t.RestartTelnetd(); err != nil {
		return err
	}
	fmt.Println("telnetd restarted, verifying permanent telnet..")

	// pc sometimes takes a while to respawn telnetd, so be more patient here
	// than in the initial login (30s budget) before declaring the restart bad.
	v, err := telnet.NewRetry("root", "Zte521", ip, telnetPort, 15, 2*time.Second)
	if err != nil {
		return fmt.Errorf("telnetd did not come back up after restart: %w", err)
	}
	defer v.Conn.Close()
	if err := v.Login(); err != nil {
		return fmt.Errorf("permanent telnet verification failed after restart: %w", err)
	}
	fmt.Println("permanent telnet verified after in-place restart")
	return nil
}
