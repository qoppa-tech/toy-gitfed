// Package multibase implements multibase decoding and multicodec key extraction.
//
// Multibase is a self-describing encoding: the first character of an encoded
// string identifies which encoding was applied to the rest.
//
// Supported prefixes:
//
//	z  base58btc
//	f  hex lowercase
//	F  hex uppercase
//	u  base64url no-padding
//	m  base64 standard with padding
//	M  base64 standard no-padding
//
// After multibase-decoding, the raw bytes may carry a multicodec varint
// prefix that identifies the key type:
//
//	0xed 0x01  → Ed25519 public key  (32 payload bytes)
//	0xec 0x01  → Ed25519 private key
//	0x1205     → secp256k1 compressed public key  (33 payload bytes)
//	0x1200     → P-256 public key
package multibase

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
)

// Encoding identifies which multibase encoding was used.
type Encoding int

const (
	Base58btc Encoding = iota
	HexLower
	HexUpper
	Base64URL
	Base64Std
	Base64StdNoPad
)

// KeyType identifies a multicodec key type.
type KeyType int

const (
	Ed25519Pub  KeyType = iota
	Ed25519Priv
	Secp256k1Pub
	P256Pub
	Unknown
)

// DecodeResult holds the decoded multibase payload and identified encoding.
type DecodeResult struct {
	Data     []byte
	Encoding Encoding
}

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnsupportedEncoding = errors.New("unsupported encoding")
	ErrInvalidBase58Char  = errors.New("invalid base58 character")
	ErrUnsupportedKeyType = errors.New("unsupported key type")
	ErrInvalidKeyLength   = errors.New("invalid key length")
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58Decode decodes a base58btc-encoded string to raw bytes.
// Leading '1' characters each map to one 0x00 byte in the output.
func Base58Decode(encoded string) ([]byte, error) {
	if len(encoded) == 0 {
		return []byte{}, nil
	}

	// Count leading '1' → each becomes a leading 0x00 byte.
	leadingZeros := 0
	for _, c := range encoded {
		if c != '1' {
			break
		}
		leadingZeros++
	}

	// Big-integer arithmetic: accumulate value.
	result := big.NewInt(0)
	base := big.NewInt(58)
	for _, c := range encoded {
		idx := indexOf(base58Alphabet, byte(c))
		if idx < 0 {
			return nil, ErrInvalidBase58Char
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}

	decoded := result.Bytes()

	// Prepend leading zero bytes.
	out := make([]byte, leadingZeros+len(decoded))
	copy(out[leadingZeros:], decoded)
	return out, nil
}

// Base58Encode encodes raw bytes as a base58btc string.
// Leading 0x00 bytes each become a leading '1' character.
func Base58Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Count leading 0x00 → each becomes '1'.
	leadingZeros := 0
	for _, b := range data {
		if b != 0 {
			break
		}
		leadingZeros++
	}

	// Big-integer division.
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	mod := new(big.Int)

	var encoded []byte
	zero := big.NewInt(0)
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		encoded = append(encoded, base58Alphabet[mod.Int64()])
	}

	// Append '1' for each leading zero byte.
	for i := 0; i < leadingZeros; i++ {
		encoded = append(encoded, '1')
	}

	// Reverse.
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}

	return string(encoded)
}

// Decode decodes a multibase-encoded string.
// The first character identifies the encoding.
func Decode(encoded string) (DecodeResult, error) {
	if len(encoded) == 0 {
		return DecodeResult{}, ErrInvalidInput
	}

	prefix := encoded[0]
	payload := encoded[1:]

	switch prefix {
	case 'z':
		data, err := Base58Decode(payload)
		if err != nil {
			return DecodeResult{}, err
		}
		return DecodeResult{Data: data, Encoding: Base58btc}, nil
	case 'f':
		data, err := hex.DecodeString(payload)
		if err != nil {
			return DecodeResult{}, ErrInvalidInput
		}
		return DecodeResult{Data: data, Encoding: HexLower}, nil
	case 'F':
		data, err := hex.DecodeString(payload)
		if err != nil {
			return DecodeResult{}, ErrInvalidInput
		}
		return DecodeResult{Data: data, Encoding: HexUpper}, nil
	case 'u':
		data, err := base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return DecodeResult{}, ErrInvalidInput
		}
		return DecodeResult{Data: data, Encoding: Base64URL}, nil
	case 'm':
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return DecodeResult{}, ErrInvalidInput
		}
		return DecodeResult{Data: data, Encoding: Base64Std}, nil
	case 'M':
		data, err := base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return DecodeResult{}, ErrInvalidInput
		}
		return DecodeResult{Data: data, Encoding: Base64StdNoPad}, nil
	default:
		return DecodeResult{}, ErrUnsupportedEncoding
	}
}

// ExtractEd25519PubKey extracts the raw 32-byte Ed25519 public key from a
// multicodec-prefixed buffer (expects 0xed 0x01 prefix + 32 bytes).
func ExtractEd25519PubKey(multicodecBytes []byte) ([32]byte, error) {
	var out [32]byte
	if len(multicodecBytes) < 2 || multicodecBytes[0] != 0xed || multicodecBytes[1] != 0x01 {
		return out, ErrUnsupportedKeyType
	}
	payload := multicodecBytes[2:]
	if len(payload) != 32 {
		return out, ErrInvalidKeyLength
	}
	copy(out[:], payload)
	return out, nil
}

// ExtractKeyType identifies the key type from a multicodec-prefixed buffer.
func ExtractKeyType(data []byte) (KeyType, error) {
	if len(data) < 2 {
		return Unknown, ErrInvalidInput
	}
	b0, b1 := data[0], data[1]
	switch {
	case b0 == 0xed && b1 == 0x01:
		return Ed25519Pub, nil
	case b0 == 0xec && b1 == 0x01:
		return Ed25519Priv, nil
	case b0 == 0x12 && b1 == 0x05:
		return Secp256k1Pub, nil
	case b0 == 0x12 && b1 == 0x00:
		return P256Pub, nil
	default:
		return Unknown, nil
	}
}

// DecodeEd25519PubKey decodes a publicKeyMultibase field from a DID document
// into an Ed25519 public key.
//
// Flow: multibase string → raw bytes → strip 0xed01 multicodec prefix → 32-byte key
func DecodeEd25519PubKey(multibaseStr string) (ed25519.PublicKey, error) {
	result, err := Decode(multibaseStr)
	if err != nil {
		return nil, err
	}
	keyBytes, err := ExtractEd25519PubKey(result.Data)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(keyBytes[:]), nil
}

func indexOf(alphabet string, c byte) int {
	for i := 0; i < len(alphabet); i++ {
		if alphabet[i] == c {
			return i
		}
	}
	return -1
}
