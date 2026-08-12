# zteOnu

A tool to open the factory / telnet mode on ZTE ONU devices via the `webFac`
interface. Type `./zteonu -h` for help.

## Build

```bash
go build -o zteonu .
```

## Usage

```bash
# Old method (default), open telnet on 192.168.1.1:8080
./zteonu -i 192.168.1.1

# New method, derive the SendInfo payload from the interface MAC
./zteonu -i 192.168.1.1 --new

# New method using a specific network interface's MAC
./zteonu -i 192.168.1.1 --new --iface en0

# New method using a custom MAC address for the SendInfo payload
./zteonu -i 192.168.1.1 --new -m 00:07:29:55:35:57

# Also enable permanent telnet (user: root, pass: Zte521)
./zteonu -i 192.168.1.1 --new --telnet
```

## Flags

| Flag       | Short | Default                   | Description                                                                          |
|------------|-------|---------------------------|--------------------------------------------------------------------------------------|
| `--user`   | `-u`  | `telecomadmin`            | factory mode auth username                                                           |
| `--pass`   | `-p`  | `nE7jA%5m`                | factory mode auth password                                                           |
| `--ip`     | `-i`  | `192.168.1.1`             | ONU ip address                                                                       |
| `--port`   |       | `8080`                    | ONU http port                                                                        |
| `--telnet` |       | `false`                   | permanent telnet (user: `root`, pass: `Zte521`)                                      |
| `--tp`     |       | `23`                      | ONU telnet port                                                                      |
| `--new`    |       | `false`                   | use the new method; the `SendInfo` payload is derived from the MAC of each local interface and tried until the device accepts one |
| `--iface`  |       | `""` (first non-loopback) | network interface to read the MAC from                                               |
| `--mac`    | `-m`  | `""`                      | custom client MAC used to derive the `SendInfo` payload (e.g. `00:07:29:55:35:57`); defaults to the interface MAC |

## Notes on `--new`

The new method sends a `SendInfo` payload that encodes the MAC address of a local network interface (see `app/factory`).
The device only authorizes MAC addresses it accepts, so:

- Every local interface MAC is tried in turn until one is accepted (the interface the ONU is reached through is the one
  the device has associated with this client). The tool only fails after all interface MACs have been rejected.
- Use `--iface` to restrict the candidates to a single interface.
- Use `--mac` to supply a custom MAC directly; this overrides the interface MAC and is the only candidate tried.
- The device MAC must be one the device accepts. Historically the device accepted `00:07:29:55:35:57`; supply it via `--mac`
  or spoof the interface MAC (or use a device that accepts the current MAC) so the payload matches what the device expects.

The payload transformation is derived from reverse-engineering the device's verification VM: the 46-byte payload is 12 little-endian `uint16` values (`info=12`), each packed as
2 data bytes + 2 zero bytes (the last value has no trailing zeros). For each value `w` the device computes `w^1271 mod 2537` and keeps the low byte; the 12 resulting bytes are
grouped by 6 and compared against the client MAC. The first six values are therefore chosen as preimages of the MAC bytes, i.e. `v` such that `(v^1271 mod 2537) & 0xff`
equals the MAC byte; the remaining six are filler.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Septrum101/zteOnu&type=Date)](https://star-history.com/#Septrum101/zteOnu&Date)
