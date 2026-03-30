// Package crypto provides ergonomic Ed25519 signing and verification.
//
// The identity layer uses this module to sign commit content and verify
// incoming signatures from peer forges.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const (
	SignatureSize = ed25519.SignatureSize  // 64
	PublicKeySize = ed25519.PublicKeySize  // 32
	SeedSize      = ed25519.SeedSize       // 32
	SecretKeySize = ed25519.PrivateKeySize // seed + public key (RFC 8032)
)

type (
	SignatureType = [SignatureSize]byte
	PublicKeyType = [PublicKeySize]byte
	SecretKeyType = [SecretKeySize]byte
	SeedType      = [SeedSize]byte
)

var (
	ErrInvalidHexLength = errors.New("invalid hex length")
	ErrInvalidHexChar   = errors.New("invalid hex character")
	ErrInvalidBase64    = errors.New("invalid base64")
)

// KeyPair holds an Ed25519 key pair as raw byte arrays.
type KeyPair struct {
	PublicKey PublicKeyType
	SecretKey SecretKeyType
}

// GenerateKeyPair generates a random Ed25519 key pair.
func GenerateKeyPair() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	var kp KeyPair
	copy(kp.PublicKey[:], pub)
	copy(kp.SecretKey[:], priv)
	return kp, nil
}

// KeyPairFromSeed derives a key pair from a 32-byte seed deterministically.
func KeyPairFromSeed(seed SeedType) KeyPair {
	priv := ed25519.NewKeyFromSeed(seed[:])
	pub := priv.Public().(ed25519.PublicKey)
	var kp KeyPair
	copy(kp.PublicKey[:], pub)
	copy(kp.SecretKey[:], priv)
	return kp
}

// Sign creates a detached Ed25519 signature for message using secretKey.
func Sign(message []byte, secretKey SecretKeyType) SignatureType {
	priv := ed25519.PrivateKey(secretKey[:])
	sig := ed25519.Sign(priv, message)
	var out SignatureType
	copy(out[:], sig)
	return out
}

// Verify checks a detached signature against message and publicKey.
// Returns true on success, false on any failure.
func Verify(message []byte, signature SignatureType, publicKey PublicKeyType) bool {
	pub := ed25519.PublicKey(publicKey[:])
	return ed25519.Verify(pub, message, signature[:])
}

// SignatureToHex encodes a 64-byte signature as a 128-character lowercase hex string.
func SignatureToHex(sig SignatureType) string {
	return hex.EncodeToString(sig[:])
}

// SignatureFromHex decodes a 128-character hex string to a 64-byte signature.
func SignatureFromHex(h string) (SignatureType, error) {
	var out SignatureType
	if len(h) != 128 {
		return out, ErrInvalidHexLength
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return out, ErrInvalidHexChar
	}
	copy(out[:], b)
	return out, nil
}

// SignatureToBase64 encodes a 64-byte signature as standard base64 with padding.
func SignatureToBase64(sig SignatureType) string {
	return base64.StdEncoding.EncodeToString(sig[:])
}

// SignatureFromBase64 decodes a standard base64 string to a 64-byte signature.
func SignatureFromBase64(encoded string) (SignatureType, error) {
	var out SignatureType
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return out, ErrInvalidBase64
	}
	if len(b) != SignatureSize {
		return out, ErrInvalidBase64
	}
	copy(out[:], b)
	return out, nil
}

// PublicKeyToHex encodes a 32-byte public key as a 64-character lowercase hex string.
func PublicKeyToHex(key PublicKeyType) string {
	return hex.EncodeToString(key[:])
}

// PublicKeyFromHex decodes a 64-character hex string to a 32-byte public key.
func PublicKeyFromHex(h string) (PublicKeyType, error) {
	var out PublicKeyType
	if len(h) != 64 {
		return out, ErrInvalidHexLength
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return out, ErrInvalidHexChar
	}
	copy(out[:], b)
	return out, nil
}
