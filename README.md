# zteOnu

This is a fork from original [project](https://github.com/Septrum101/zteOnu)

Type `./zteonu -h` for help

# What's different from original one

- Added all known user/password combinations in a loop; the binary will attempt all of them to enable Telnet.
- Added the --seclvl parameter (default: 2) to change the Telnet access level and avoid the "Access Denied" error.
- Added firewall configuration when enabling permanent Telnet access.
- Removed login retries to prevent account lock-out.
- Changed to use the default HTTP port 80 instead of 8080.
