package zlib

import (
	"bytes"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	original := []byte("The quick brown fox jumps over the lazy dog")
	compressed, err := Compress(original)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, restored) {
		t.Fatalf("mismatch: %q != %q", original, restored)
	}
}

func TestDecompressKnownBlob(t *testing.T) {
	// python3: zlib.compress(b"hello")
	compressed := []byte{0x78, 0x9c, 0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, 0x06, 0x2c, 0x02, 0x15}
	out, err := Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Fatalf("got %q, want %q", string(out), "hello")
	}
}

func TestCompressDecompressEmpty(t *testing.T) {
	compressed, err := Compress([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 0 {
		t.Fatalf("expected empty, got %x", restored)
	}
}

func TestLargeInputRoundtrip(t *testing.T) {
	size := 64*1024 + 1
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	compressed, err := Compress(data)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decompress(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, restored) {
		t.Fatal("mismatch on large input")
	}
}

func TestDecompressStream(t *testing.T) {
	original := []byte("streaming test data")
	compressed, err := Compress(original)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecompressStream(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, restored) {
		t.Fatalf("mismatch: %q != %q", original, restored)
	}
}
