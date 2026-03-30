package crypto

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("commit abc123\nauthor alice\n")
	sig := Sign(msg, kp.SecretKey)
	if !Verify(msg, sig, kp.PublicKey) {
		t.Fatal("verify failed")
	}
}

func TestTamperedMessageFailsVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign([]byte("original message"), kp.SecretKey)
	if Verify([]byte("tampered message"), sig, kp.PublicKey) {
		t.Fatal("expected verify to fail")
	}
}

func TestTamperedSignatureFailsVerify(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("commit content")
	sig := Sign(msg, kp.SecretKey)
	sig[0] ^= 0xff
	if Verify(msg, sig, kp.PublicKey) {
		t.Fatal("expected verify to fail")
	}
}

func TestWrongKeyFailsVerify(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("some data")
	sig := Sign(msg, kp1.SecretKey)
	if Verify(msg, sig, kp2.PublicKey) {
		t.Fatal("expected verify to fail")
	}
}

func TestDeterministicSameSeedSameKeyPair(t *testing.T) {
	var seed [SeedSize]byte
	for i := range seed {
		seed[i] = 0x42
	}
	kp1 := KeyPairFromSeed(seed)
	kp2 := KeyPairFromSeed(seed)
	if kp1.PublicKey != kp2.PublicKey {
		t.Fatal("public keys differ")
	}
	if kp1.SecretKey != kp2.SecretKey {
		t.Fatal("secret keys differ")
	}
}

func TestDifferentSeedsDifferentKeyPairs(t *testing.T) {
	var seed1, seed2 [SeedSize]byte
	for i := range seed1 {
		seed1[i] = 0x01
		seed2[i] = 0x02
	}
	kp1 := KeyPairFromSeed(seed1)
	kp2 := KeyPairFromSeed(seed2)
	if kp1.PublicKey == kp2.PublicKey {
		t.Fatal("public keys should differ")
	}
}

func TestSignatureHexRoundtrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign([]byte("roundtrip"), kp.SecretKey)
	h := SignatureToHex(sig)
	recovered, err := SignatureFromHex(h)
	if err != nil {
		t.Fatal(err)
	}
	if sig != recovered {
		t.Fatal("signatures differ")
	}
}

func TestSignatureBase64Roundtrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign([]byte("base64 roundtrip"), kp.SecretKey)
	b64 := SignatureToBase64(sig)
	recovered, err := SignatureFromBase64(b64)
	if err != nil {
		t.Fatal(err)
	}
	if sig != recovered {
		t.Fatal("signatures differ")
	}
}

func TestPublicKeyHexRoundtrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	h := PublicKeyToHex(kp.PublicKey)
	recovered, err := PublicKeyFromHex(h)
	if err != nil {
		t.Fatal(err)
	}
	if kp.PublicKey != recovered {
		t.Fatal("keys differ")
	}
}

func TestSignatureFromHexWrongLength(t *testing.T) {
	_, err := SignatureFromHex("deadbeef")
	if err != ErrInvalidHexLength {
		t.Fatalf("expected ErrInvalidHexLength, got %v", err)
	}
}

func TestSignatureFromHexInvalidChars(t *testing.T) {
	bad := strings.Repeat("zz", 64) // 128 chars, non-hex
	_, err := SignatureFromHex(bad)
	if err != ErrInvalidHexChar {
		t.Fatalf("expected ErrInvalidHexChar, got %v", err)
	}
}

func TestSignatureFromBase64Invalid(t *testing.T) {
	_, err := SignatureFromBase64("not!base64!!!")
	if err != ErrInvalidBase64 {
		t.Fatalf("expected ErrInvalidBase64, got %v", err)
	}
}

func TestPublicKeyFromHexWrongLength(t *testing.T) {
	_, err := PublicKeyFromHex("aabb")
	if err != ErrInvalidHexLength {
		t.Fatalf("expected ErrInvalidHexLength, got %v", err)
	}
}

func TestPublicKeyFromHexInvalidChars(t *testing.T) {
	bad := strings.Repeat("zz", 32) // 64 chars, non-hex
	_, err := PublicKeyFromHex(bad)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSignEmptyMessage(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign([]byte{}, kp.SecretKey)
	if !Verify([]byte{}, sig, kp.PublicKey) {
		t.Fatal("verify failed for empty message")
	}
}

func TestSignLargeMessage(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := bytes.Repeat([]byte{0xab}, 1024*1024)
	sig := Sign(msg, kp.SecretKey)
	if !Verify(msg, sig, kp.PublicKey) {
		t.Fatal("verify failed for large message")
	}
}

func TestRFC8032TestVector1(t *testing.T) {
	seedHex := "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"
	sigHex := "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b"

	seedBytes, _ := hex.DecodeString(seedHex)
	var seed [SeedSize]byte
	copy(seed[:], seedBytes)

	expectedSigBytes, _ := hex.DecodeString(sigHex)
	var expectedSig [SignatureSize]byte
	copy(expectedSig[:], expectedSigBytes)

	kp := KeyPairFromSeed(seed)
	sig := Sign([]byte{}, kp.SecretKey)

	if sig != expectedSig {
		t.Fatalf("signature mismatch:\n got  %x\n want %x", sig, expectedSig)
	}

	if !Verify([]byte{}, sig, kp.PublicKey) {
		t.Fatal("verify failed")
	}
}

func TestKeyPairFromSeedConvenience(t *testing.T) {
	var seed [SeedSize]byte
	for i := range seed {
		seed[i] = 0x7f
	}
	kp1 := KeyPairFromSeed(seed)
	kp2 := KeyPairFromSeed(seed)
	if kp1.PublicKey != kp2.PublicKey {
		t.Fatal("public keys differ")
	}
}
