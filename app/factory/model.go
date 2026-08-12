package factory

import (
	"bytes"
	"fmt"

	"github.com/go-resty/resty/v2"
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

var (
	AesKeyPool = []byte{
		0x7B, 0x56, 0xB0, 0xF7, 0xDA, 0x0E, 0x68, 0x52, 0xC8, 0x19,
		0xF3, 0x2B, 0x84, 0x90, 0x79, 0xE5, 0x62, 0xF8, 0xEA, 0xD2,
		0x64, 0x93, 0x87, 0xDF, 0x73, 0xD7, 0xFB, 0xCC, 0xAA, 0xFE,
		0x75, 0x43, 0x1C, 0x29, 0xDF, 0x4C, 0x52, 0x2C, 0x6E, 0x7B,
		0x45, 0x3D, 0x1F, 0xF1, 0xDE, 0xBC, 0x27, 0x85, 0x8A, 0x45,
		0x91, 0xBE, 0x38, 0x13, 0xDE, 0x67, 0x32, 0x08, 0x54, 0x11,
		0x75, 0xF4, 0xD3, 0xB4, 0xA4, 0xB3, 0x12, 0x86, 0x67, 0x23,
		0x99, 0x4C, 0x61, 0x7F, 0xB1, 0xD2, 0x30, 0xDF, 0x47, 0xF1,
		0x76, 0x93, 0xA3, 0x8C, 0x95, 0xD3, 0x59, 0xBF, 0x87, 0x8E,
		0xF3, 0xB3, 0xE4, 0x76, 0x49, 0x88,
	}

	AesKeyPoolNew = []byte{
		0x8C, 0x23, 0x65, 0xD1, 0xFC, 0x32, 0x45, 0x37, 0x11, 0x28,
		0x71, 0x63, 0x07, 0x20, 0x69, 0x14, 0x73, 0xE7, 0xD4, 0x53,
		0x13, 0x24, 0x36, 0xC2, 0xB5, 0xE1, 0xFC, 0xCF, 0x8A, 0x9A,
		0x41, 0x89, 0x3C, 0x49, 0xCF, 0x5C, 0x72, 0x8C, 0x9E, 0xEB,
		0x75, 0x0D, 0x3F, 0xD1, 0xFE, 0xCC, 0x57, 0x65, 0x7A, 0x35,
		0x21, 0x3E, 0x68, 0x53, 0x7E, 0x97, 0x02, 0x48, 0x74, 0x71,
		0x95, 0x34, 0x53, 0x84, 0xB4, 0xC3, 0xE2, 0xD6, 0x27, 0x3D,
		0xE6, 0x5D, 0x72, 0x9C, 0xBC, 0x3D, 0x03, 0xFD, 0x76, 0xC1,
		0x9C, 0x25, 0xA8, 0x92, 0x47, 0xE4, 0x18, 0x0F, 0x24, 0x3F,
		0x4F, 0x67, 0xEC, 0x97, 0xF4, 0x99,
	}
)

type Factory struct {
	user   string
	passwd string
	ip     string
	port   int
	iface  string
	mac    string
	cli    *resty.Client
	key    []byte
}
