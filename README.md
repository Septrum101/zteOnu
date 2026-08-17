# zteOnu

A tool to open the factory / telnet mode on ZTE ONU devices via the `webFac`
interface. Type `./zteonu -h` for help.

## Build

```bash
go build -o zteonu .
```

## Usage

```bash
# Open temp telnet on 192.168.1.1:8080 (tries every local interface MAC)
./zteonu -i 192.168.1.1

# Restrict the candidate MACs to a specific network interface
./zteonu -i 192.168.1.1 --iface en0

# Use a custom client MAC for the SendInfo payload (the only candidate)
./zteonu -i 192.168.1.1 -m 00:07:29:55:35:57

# Also enable permanent telnet (user: root, pass: Zte521)
./zteonu -i 192.168.1.1 --telnet
```

Every run opens the temporary factory telnet through the `webFac` flow and then **verifies it with an actual telnet
login using the temp credentials**. The flow completing over HTTP is not proof the device accepts them - a mismatched
client MAC still yields credentials, but the telnet they authorize does not work. So each candidate MAC is tried in turn
(the `SendInfo` payload binds the session to a MAC), each attempt is judged by a real login, and unverified MACs fall
through to the next one; if none verify, the whole MAC pool is re-cycled a few times before giving up. `--telnet` only
decides whether, after the verification succeeds, the permanent telnet settings are written (and the device rebooted);
without it the tool just prints the verified temp credentials and exits.

## Flags

| Flag       | Short | Default                   | Description                                                                          |
|------------|-------|---------------------------|--------------------------------------------------------------------------------------|
| `--user`   | `-u`  | `telecomadmin`            | factory mode auth username                                                           |
| `--pass`   | `-p`  | `nE7jA%5m`                | factory mode auth password                                                           |
| `--ip`     | `-i`  | `192.168.1.1`             | ONU ip address                                                                       |
| `--port`   |       | `8080`                    | ONU http port                                                                        |
| `--telnet` |       | `false`                   | permanent telnet (user: `root`, pass: `Zte521`); only applied after a temp telnet login is verified          |
| `--tp`     |       | `23`                      | ONU telnet port                                                                      |
| `--iface`  |       | `""` (first non-loopback) | network interface to read the MAC from                                               |
| `--mac`    | `-m`  | `""`                      | custom client MAC used to derive the `SendInfo` payload (e.g. `00:07:29:55:35:57`); defaults to the interface MAC |

## Notes on the client MAC

The `SendInfo` payload encodes the MAC address of a local network interface (see `app/factory`).
The device only authorizes MAC addresses it accepts, so:

- Every candidate MAC is tried in turn. The tool proceeds only with a MAC whose granted credentials actually log in over
  telnet; the HTTP flow returning credentials alone is not enough (see Usage).
- Use `--iface` to restrict the candidates to a single interface.
- Use `--mac` to supply a custom MAC directly; this overrides the interface MAC and is the only candidate tried.
- The MAC must be one the device accepts. Historically the device accepted `00:07:29:55:35:57`; supply it via `--mac`
  or spoof the interface MAC (or use a device that accepts the current MAC) so the payload matches what the device expects.

The payload transformation is derived from reverse-engineering the device's verification VM: the 46-byte payload is 12 little-endian `uint16` values (`info=12`), each packed as
2 data bytes + 2 zero bytes (the last value has no trailing zeros). For each value `w` the device computes `w^1271 mod 2537` and keeps the low byte; the 12 resulting bytes are
grouped by 6 and compared against the client MAC. The first six values are therefore chosen as preimages of the MAC bytes, i.e. `v` such that `(v^1271 mod 2537) & 0xff`
equals the MAC byte; the remaining six are filler.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Septrum101/zteOnu&type=Date)](https://star-history.com/#Septrum101/zteOnu&Date)
