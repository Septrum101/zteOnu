# zteOnu

A tool to open the factory / telnet mode on ZTE ONU devices via the `webFac`
interface. Type `./zteonu -h` for help.

## Build

```bash
go build -o zteonu .
```

## Usage

```bash
# Open temp telnet; the client MAC is auto-detected from the route to the ONU
./zteonu -i 192.168.1.1

# Pin the MAC to a specific network interface instead
./zteonu -i 192.168.1.1 --iface en0

# Use a custom client MAC for the SendInfo payload (overrides everything)
./zteonu -i 192.168.1.1 -m 00:07:29:55:35:57

# Enable permanent telnet (user: root, pass: Zte521) by restarting telnetd in place, without rebooting
./zteonu -i 192.168.1.1 --telnet

# Same, but apply the settings by rebooting the device instead
./zteonu -i 192.168.1.1 --telnet-restart
```

When neither `--iface` nor `--mac` is given, the client MAC is auto-detected: the tool dials a UDP socket to the ONU
(route lookup only, no packet is sent), reads the chosen source address and fills in the MAC of the interface that owns
it. Every run opens the temporary factory telnet through the `webFac` flow and then **verifies it with an actual telnet
login using the temp credentials**. The flow completing over HTTP is not proof the device accepts them - a mismatched
client MAC still yields credentials, but the telnet they authorize does not work. `--telnet` only decides whether, after
the verification succeeds, the permanent telnet settings are written and applied by **restarting the
`telnetd` service in place, without rebooting**; without it the tool just prints the verified temp credentials and
exits. The in-place restart goes through the device's program manager (`sendcmd -pc kill <pid>`, which the `pc`
supervisor answers by respawning telnetd) and is verified with a fresh `root`/`Zte521` login. `--telnet-restart`
writes the same permanent settings but applies them by rebooting the device; the two flags are mutually exclusive.

## Flags

| Flag               | Short | Default        | Description                                                                                                                                                                 |
|--------------------|-------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--user`           | `-u`  | `telecomadmin` | factory mode auth username                                                                                                                                                  |
| `--pass`           | `-p`  | `nE7jA%5m`     | factory mode auth password                                                                                                                                                  |
| `--ip`             | `-i`  | `192.168.1.1`  | ONU ip address                                                                                                                                                              |
| `--port`           |       | `8080`         | ONU http port                                                                                                                                                               |
| `--telnet`         |       | `false`        | permanent telnet (user: `root`, pass: `Zte521`) applied by restarting the `telnetd` service in place, without rebooting; only applied after a temp telnet login is verified |
| `--telnet-restart` |       | `false`        | permanent telnet (user: `root`, pass: `Zte521`) applied by rebooting the device; mutually exclusive with `--telnet`                                                         |
| `--tp`             |       | `23`           | ONU telnet port                                                                                                                                                             |
| `--iface`          |       | `""`           | network interface whose MAC to use (default: auto-detected from the route to the ONU)                                                                                       |
| `--mac`            | `-m`  | `""`           | custom client MAC for the `SendInfo` payload (e.g. `00:07:29:55:35:57`); overrides `--iface` and auto-detection                                                             |

## Notes on the client MAC

The `SendInfo` payload encodes the MAC address of a local network interface (see `app/factory`). The device only
authorizes MAC addresses it accepts, so:

- Without `--iface` or `--mac` the interface that routes to the ONU is auto-detected and its MAC is used.
- The MAC must be one the device accepts. Historically the device accepted `00:07:29:55:35:57`; supply it via `--mac`
  or spoof the interface MAC (or use a device that accepts the current MAC) so the payload matches what the device
  expects.

The payload transformation is derived from reverse-engineering the device's verification VM: the 46-byte payload is 12
little-endian `uint16` values (`info=12`), each packed as 2 data bytes + 2 zero bytes (the last value has no trailing
zeros). For each value `w` the device computes `w^1271 mod 2537` and keeps the low byte; the 12 resulting bytes are
grouped by 6 and compared against the client MAC. The first six values are therefore chosen as preimages of the MAC
bytes, i.e. `v` such that `(v^1271 mod 2537) & 0xff`
equals the MAC byte; the remaining six are filler.
