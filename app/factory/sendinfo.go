package factory

import (
	"bytes"
	"fmt"
)

// Device-side SendInfo verification, reversed from the webFacCheckClientInfo
// VM (see zte_payload.py):
//
//  1. info must be "<N>|<payload>", with N%6==0 and N<=512.
//  2. Each 4-byte group of the payload is read as a little-endian value w and
//     the VM computes acc = 1; then repeats 1271 times acc = (acc * w) % 2537,
//     i.e. acc = w^1271 mod 2537. byte_buf[k] = acc & 0xff.
//  3. byte_buf is grouped by 6 bytes; if any group equals the client MAC, the
//     request is authorized.
//
// The payload is therefore 46 bytes = 12 little-endian uint16 values
// ("info=12"), each packed as 2 data bytes + 2 zero bytes (the 12th value has
// no trailing zero bytes). The first 6 values are modular-exponentiation
// preimages of the 6 MAC bytes: for each MAC byte m we pick a value v with
// (v^1271 mod 2537) & 0xff == m. The remaining 6 values are filler that does
// not take part in the MAC match.
const (
	mod = 0x9E9     // 2537
	exp = 0x4F8 - 1 // 1271 (the VM counter counts down from 0x4f8 to 1)
)

// power returns w^exp mod mod, equivalent to the device VM's multiply loop.
func power(w uint32) byte {
	w %= mod
	acc := uint32(1)
	for e := exp; e > 0; e >>= 1 {
		if e&1 == 1 {
			acc = (acc * w) % mod
		}
		w = (w * w) % mod
	}
	return byte(acc & 0xff)
}

// revTable maps every byte value to the preimages v in [0, mod) satisfying
// power(v) == that byte, in ascending order.
var revTable [256][]uint16

func init() {
	var buckets [256][]uint16
	for v := range uint16(mod) {
		b := power(uint32(v))
		buckets[b] = append(buckets[b], v)
	}
	for b := range 256 {
		if len(buckets[b]) == 0 {
			panic(fmt.Sprintf("no preimage for byte %02x", b))
		}
		revTable[b] = buckets[b]
	}
}

// MacToMagicBytes builds the 46-byte SendInfo magic payload that encodes the
// given client MAC. The first six uint16 values are the smallest preimages of
// the six MAC bytes; the remaining six are fixed filler values that do not
// affect the MAC check.
func MacToMagicBytes(mac [6]byte) []byte {
	vals := make([]uint16, 12)
	for i, b := range mac {
		vals[i] = revTable[b][0]
	}
	out := make([]byte, 0, 46)
	for _, v := range vals[:11] {
		out = append(out, byte(v), byte(v>>8), 0, 0)
	}
	return append(out, byte(vals[11]), byte(vals[11]>>8))
}

// verifyPayload replicates the device-side check: split the payload into
// 4-byte groups, apply power() to each, group the resulting bytes by 6, and
// report whether any group equals mac.
func verifyPayload(payload []byte, mac [6]byte) bool {
	var bb [12]byte
	for k := range 12 {
		var w uint32
		for i := range 4 {
			if idx := 4*k + i; idx < len(payload) {
				w |= uint32(payload[idx]) << (8 * i)
			}
		}
		bb[k] = power(w)
	}
	for i := range 12 / 6 {
		if bytes.Equal(bb[6*i:6*i+6], mac[:]) {
			return true
		}
	}
	return false
}
