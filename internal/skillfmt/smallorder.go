package skillfmt

import (
	"encoding/hex"
	"math/big"
)

// A public key of small order makes an ed25519 verifier accept a signature that
// the holder of no key made: the identity point with signature 01 00..00 ‖ 00..00
// verifies over every message. ParsePublicKey refuses these keys.
//
// The curve's group has order 8·ℓ, so exactly eight points P satisfy 8P = identity.
// A point is fixed by its y coordinate and a sign bit, and the eight points have
// only five distinct y values (mod p = 2^255 − 19):
//
//	y = 0       the two points of order 4
//	y = 1       the identity (order 1)
//	y = p − 1   the point of order 2
//	y = y8, p − y8   the four points of order 8
//
// Source of the list: libsodium's `has_small_order` blacklist
// (src/libsodium/crypto_core/ed25519/ref10/ed25519_ref10.c), which lists the
// canonical encodings of these points plus the non-canonical ones y = p and
// y = p + 1. The two order-8 values below are read from that list from memory of
// it — marked UNVERIFIED as a citation — and are NOT trusted on that account:
// TestSmallOrderListIsExactlyTheTorsionPoints recovers x for each y, checks
// 8P = identity with big-integer arithmetic of the curve's addition law, and
// requires exactly eight distinct points. A wrong constant fails that test.
//
// A key is compared on y alone, reduced mod p, with the sign bit ignored: that
// covers the canonical encodings, the non-canonical ones (y ≥ p, which a lax
// decoder reduces) and either sign bit.
var smallOrderY = func() []*big.Int {
	var out []*big.Int
	for _, h := range []string{
		"0000000000000000000000000000000000000000000000000000000000000000", // y = 0
		"0100000000000000000000000000000000000000000000000000000000000000", // y = 1
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f", // y = p − 1
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05", // y8
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a", // p − y8
	} {
		b, _ := hex.DecodeString(h)
		out = append(out, littleEndian(b))
	}
	return out
}()

var fieldPrime = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))

func littleEndian(b []byte) *big.Int {
	be := make([]byte, len(b))
	for i := range b {
		be[len(b)-1-i] = b[i]
	}
	return new(big.Int).SetBytes(be)
}

// isSmallOrder reports whether a 32-byte ed25519 public key encodes a point of
// small order, in any canonical or non-canonical spelling.
func isSmallOrder(key []byte) bool {
	b := append([]byte(nil), key...)
	b[31] &^= 0x80 // the sign bit does not change y
	y := littleEndian(b)
	y.Mod(y, fieldPrime)
	for _, s := range smallOrderY {
		if y.Cmp(s) == 0 {
			return true
		}
	}
	return false
}
