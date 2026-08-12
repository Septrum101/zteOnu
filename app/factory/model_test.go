package factory

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// capturedRefMAC is the MAC the reference payload below was captured for.
var capturedRefMAC = [6]byte{0x00, 0x07, 0x29, 0x55, 0x35, 0x57}

// capturedRefPayloadB64 is the originally captured SendInfo magic payload for
// capturedRefMAC. Kept as a cross-check: it must pass the device-side
// verification (its values are valid preimages), which proves the VM algorithm
// this package now implements matches reality.
const capturedRefPayloadB64 = "AAAAAGAIAACTBwAAOggAALoAAACQBwAAxAcAAMoGAACVBAAATggAAM0BAAAnCA=="

// TestCapturedReferencePayloadValid checks that the originally captured blob is
// accepted by the device-side verification replicated in verifyPayload.
func TestCapturedReferencePayloadValid(t *testing.T) {
	orig, err := base64.StdEncoding.DecodeString(capturedRefPayloadB64)
	if err != nil {
		t.Fatal(err)
	}
	if len(orig) != 46 {
		t.Fatalf("captured payload length %d, want 46", len(orig))
	}
	if !verifyPayload(orig, capturedRefMAC) {
		t.Fatal("originally captured payload fails device-side verification")
	}
}

// TestMacToMagicBytesReference verifies that the payload computed for the
// reference MAC is accepted by the device-side verification.
func TestMacToMagicBytesReference(t *testing.T) {
	got := MacToMagicBytes(capturedRefMAC)
	if len(got) != 46 {
		t.Fatalf("unexpected payload length: %d, want 46", len(got))
	}
	if !verifyPayload(got, capturedRefMAC) {
		t.Fatal("generated payload for reference MAC fails device-side verification")
	}
}

// TestMacToMagicBytesDeterministic verifies the output is stable for a given MAC.
func TestMacToMagicBytesDeterministic(t *testing.T) {
	mac := [6]byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}
	a := MacToMagicBytes(mac)
	b := MacToMagicBytes(mac)
	if !bytes.Equal(a, b) {
		t.Fatal("MacToMagicBytes is not deterministic for a fixed MAC")
	}
	if len(a) != 46 {
		t.Fatalf("unexpected payload length: %d, want 46", len(a))
	}
}

// TestMacToMagicBytesOtherMACs verifies that payloads generated for a variety
// of MACs (including edge cases) pass the device-side verification, i.e. the
// first 6-byte group of the recomputed byte_buf equals the MAC.
func TestMacToMagicBytesOtherMACs(t *testing.T) {
	cases := [][6]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
		{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01},
		{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		{0x00, 0x07, 0x29, 0x55, 0x35, 0x57},
	}
	for _, mac := range cases {
		if !verifyPayload(MacToMagicBytes(mac), mac) {
			t.Errorf("MacToMagicBytes(%x) fails device-side verification", mac)
		}
	}
}
