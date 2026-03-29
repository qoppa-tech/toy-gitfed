package multibase

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestBase58RoundTrip(t *testing.T) {
	original := append([]byte{0xed, 0x01}, bytes.Repeat([]byte{0x42}, 32)...)
	enc := Base58Encode(original)
	dec, err := Base58Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, dec) {
		t.Fatalf("mismatch: %x != %x", original, dec)
	}
}

func TestBase58DecodeEmpty(t *testing.T) {
	r, err := Base58Decode("")
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Fatalf("expected empty, got %x", r)
	}
}

func TestBase58DecodeSingle1(t *testing.T) {
	r, err := Base58Decode("1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r, []byte{0x00}) {
		t.Fatalf("expected [0x00], got %x", r)
	}
}

func TestBase58Decode2g(t *testing.T) {
	r, err := Base58Decode("2g")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r, []byte{0x61}) {
		t.Fatalf("expected [0x61], got %x", r)
	}
}

func TestBase58DecodeLeadingZeros(t *testing.T) {
	r, err := Base58Decode("111")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r, []byte{0, 0, 0}) {
		t.Fatalf("expected [0,0,0], got %x", r)
	}
}

func TestBase58Encode0x61(t *testing.T) {
	r := Base58Encode([]byte{0x61})
	if r != "2g" {
		t.Fatalf("expected '2g', got %q", r)
	}
}

func TestBase58EncodeEmpty(t *testing.T) {
	r := Base58Encode([]byte{})
	if r != "" {
		t.Fatalf("expected empty, got %q", r)
	}
}

func TestBase58EncodeSingleZero(t *testing.T) {
	r := Base58Encode([]byte{0x00})
	if r != "1" {
		t.Fatalf("expected '1', got %q", r)
	}
}

func TestBase58InvalidChar0(t *testing.T) {
	_, err := Base58Decode("0abc")
	if err != ErrInvalidBase58Char {
		t.Fatalf("expected ErrInvalidBase58Char, got %v", err)
	}
}

func TestBase58InvalidCharO(t *testing.T) {
	_, err := Base58Decode("Oabc")
	if err != ErrInvalidBase58Char {
		t.Fatalf("expected ErrInvalidBase58Char, got %v", err)
	}
}

func TestBase58InvalidCharI(t *testing.T) {
	_, err := Base58Decode("Iabc")
	if err != ErrInvalidBase58Char {
		t.Fatalf("expected ErrInvalidBase58Char, got %v", err)
	}
}

func TestBase58InvalidCharl(t *testing.T) {
	_, err := Base58Decode("labc")
	if err != ErrInvalidBase58Char {
		t.Fatalf("expected ErrInvalidBase58Char, got %v", err)
	}
}

func TestMultibaseDecodeBase58(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03}
	b58 := Base58Encode(raw)
	multibaseStr := "z" + b58

	r, err := Decode(multibaseStr)
	if err != nil {
		t.Fatal(err)
	}
	if r.Encoding != Base58btc {
		t.Fatalf("encoding = %v, want Base58btc", r.Encoding)
	}
	if !bytes.Equal(raw, r.Data) {
		t.Fatalf("data mismatch: %x != %x", raw, r.Data)
	}
}

func TestMultibaseDecodeHexLower(t *testing.T) {
	r, err := Decode("f68656c6c6f")
	if err != nil {
		t.Fatal(err)
	}
	if r.Encoding != HexLower {
		t.Fatalf("encoding = %v, want HexLower", r.Encoding)
	}
	if string(r.Data) != "hello" {
		t.Fatalf("data = %q, want %q", string(r.Data), "hello")
	}
}

func TestMultibaseDecodeUnknownPrefix(t *testing.T) {
	_, err := Decode("x48656c6c6f")
	if err != ErrUnsupportedEncoding {
		t.Fatalf("expected ErrUnsupportedEncoding, got %v", err)
	}
}

func TestMultibaseDecodeEmpty(t *testing.T) {
	_, err := Decode("")
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestExtractEd25519PubKeyValid(t *testing.T) {
	raw := append([]byte{0xed, 0x01}, bytes.Repeat([]byte{0xab}, 32)...)
	key, err := ExtractEd25519PubKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	expected := [32]byte{}
	for i := range expected {
		expected[i] = 0xab
	}
	if key != expected {
		t.Fatal("key mismatch")
	}
}

func TestExtractEd25519PubKeyWrongPrefix(t *testing.T) {
	raw := append([]byte{0x00, 0x00}, make([]byte, 32)...)
	_, err := ExtractEd25519PubKey(raw)
	if err != ErrUnsupportedKeyType {
		t.Fatalf("expected ErrUnsupportedKeyType, got %v", err)
	}
}

func TestExtractEd25519PubKeyShortPayload(t *testing.T) {
	raw := append([]byte{0xed, 0x01}, make([]byte, 31)...)
	_, err := ExtractEd25519PubKey(raw)
	if err != ErrInvalidKeyLength {
		t.Fatalf("expected ErrInvalidKeyLength, got %v", err)
	}
}

func TestExtractKeyTypeKnown(t *testing.T) {
	cases := []struct {
		data []byte
		want KeyType
	}{
		{[]byte{0xed, 0x01, 0, 0}, Ed25519Pub},
		{[]byte{0xec, 0x01, 0, 0}, Ed25519Priv},
		{[]byte{0x12, 0x05, 0, 0}, Secp256k1Pub},
		{[]byte{0x12, 0x00, 0, 0}, P256Pub},
		{[]byte{0xff, 0xff, 0, 0}, Unknown},
	}
	for _, tc := range cases {
		got, err := ExtractKeyType(tc.data)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("ExtractKeyType(%x) = %v, want %v", tc.data, got, tc.want)
		}
	}
}

func TestFullPipelineRealDIDKey(t *testing.T) {
	multibase := "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	pk, err := DecodeEd25519PubKey(multibase)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(pk))
	}
}

func TestFullPipelineRoundtrip(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Encode as multicodec: 0xed01 + raw public key.
	multicodec := append([]byte{0xed, 0x01}, pub...)

	// Base58-encode and prepend 'z' multibase prefix.
	multibaseStr := "z" + Base58Encode(multicodec)

	// Decode back.
	recovered, err := DecodeEd25519PubKey(multibaseStr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pub, recovered) {
		t.Fatalf("key mismatch: %x != %x", pub, recovered)
	}
}
