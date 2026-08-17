package onu

import (
	"fmt"

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
