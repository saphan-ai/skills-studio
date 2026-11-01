package skillfmt

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// DecodeHash returns the 32 digest bytes of a canonical hash in lowercase hex.
func DecodeHash(hexHash string) ([]byte, error) {
	b, err := hex.DecodeString(hexHash)
	if err != nil || len(b) != 32 {
		return nil, errors.New("not a 64-digit hex sha256")
	}
	return b, nil
}

// decodeBytes reads file content as raw bytes of the wanted length, or as hex,
// or as standard base64 (surrounding white space ignored).
func decodeBytes(data []byte, want int) ([]byte, bool) {
	if len(data) == want {
		return data, true
	}
	text := strings.TrimSpace(string(data))
	if b, err := hex.DecodeString(text); err == nil && len(b) == want {
		return b, true
	}
	if b, err := base64.StdEncoding.DecodeString(text); err == nil && len(b) == want {
		return b, true
	}
	return nil, false
}

// ParseSignature reads a signature file: 64 raw bytes, or 128 hex digits, or
// standard base64 of 64 bytes.
func ParseSignature(data []byte) ([]byte, *Violation) {
	if b, ok := decodeBytes(data, ed25519.SignatureSize); ok {
		return b, nil
	}
	return nil, &Violation{Rule: RuleSignatureMalformed, Message: "the signature file is not 64 raw bytes, 128 hex digits or base64 of 64 bytes"}
}

// ParsePublicKey reads a public key file: 32 raw bytes, 64 hex digits, standard
// base64 of 32 bytes, or a PEM "PUBLIC KEY" block holding an ed25519 key.
func ParsePublicKey(data []byte) (ed25519.PublicKey, *Violation) {
	if block, _ := pem.Decode(data); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if pk, ok := key.(ed25519.PublicKey); err == nil && ok {
			return checkNotSmallOrder(pk)
		}
		return nil, &Violation{Rule: RulePubkeyMalformed, Message: "the PEM block is not an ed25519 public key"}
	}
	if b, ok := decodeBytes(data, ed25519.PublicKeySize); ok {
		return checkNotSmallOrder(ed25519.PublicKey(b))
	}
	return nil, &Violation{Rule: RulePubkeyMalformed, Message: "the public key file is not 32 raw bytes, 64 hex digits, base64 of 32 bytes or a PEM public key"}
}

func checkNotSmallOrder(pk ed25519.PublicKey) (ed25519.PublicKey, *Violation) {
	if isSmallOrder(pk) {
		return nil, &Violation{Rule: RulePubkeyMalformed, Message: "the public key is a point of small order, which verifies signatures nobody made with it"}
	}
	return pk, nil
}

// CheckSignature verifies an ed25519 signature over the 32 digest bytes of the
// canonical hash. It verifies that signature and nothing else.
func CheckSignature(hexHash string, sigFile, keyFile []byte) *Violation {
	digest, err := DecodeHash(hexHash)
	if err != nil {
		return &Violation{Rule: RuleSignatureInvalid, Message: fmt.Sprintf("canonical hash unusable: %v", err)}
	}
	sig, v := ParseSignature(sigFile)
	if v != nil {
		return v
	}
	key, v := ParsePublicKey(keyFile)
	if v != nil {
		return v
	}
	if !ed25519.Verify(key, digest, sig) {
		return &Violation{Rule: RuleSignatureInvalid, Message: "the signature does not verify over the canonical hash with this public key"}
	}
	return nil
}
