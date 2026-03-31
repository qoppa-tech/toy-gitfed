package pktline

import (
	"bytes"
	"testing"
)

func TestEncodeHello(t *testing.T) {
	pkt, err := Encode([]byte("hello\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(pkt) != "000ahello\n" {
		t.Fatalf("got %q, want %q", string(pkt), "000ahello\\n")
	}
}

func TestDecodeHello(t *testing.T) {
	pkt, err := Decode([]byte("000ahello\n"))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != Data {
		t.Fatalf("type = %v, want Data", pkt.Type)
	}
	if string(pkt.Payload) != "hello\n" {
		t.Fatalf("payload = %q, want %q", string(pkt.Payload), "hello\\n")
	}
	if pkt.Consumed != 10 {
		t.Fatalf("consumed = %d, want 10", pkt.Consumed)
	}
}

func TestDecodeFlush(t *testing.T) {
	pkt, err := Decode([]byte("0000"))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != Flush {
		t.Fatalf("type = %v, want Flush", pkt.Type)
	}
	if pkt.Consumed != 4 {
		t.Fatalf("consumed = %d, want 4", pkt.Consumed)
	}
}

func TestDecodeDelimiter(t *testing.T) {
	pkt, err := Decode([]byte("0001"))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != Delimiter {
		t.Fatalf("type = %v, want Delimiter", pkt.Type)
	}
}

func TestDecodeResponseEnd(t *testing.T) {
	pkt, err := Decode([]byte("0002"))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != ResponseEnd {
		t.Fatalf("type = %v, want ResponseEnd", pkt.Type)
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	original := []byte("want abc123\n")
	wire, err := Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != Data {
		t.Fatalf("type = %v, want Data", pkt.Type)
	}
	if !bytes.Equal(pkt.Payload, original) {
		t.Fatalf("payload mismatch")
	}
}

func TestMultiplePacketsViaConsumed(t *testing.T) {
	buf := []byte("000afirst\n000bsecond\n0000")
	offset := 0

	p1, err := Decode(buf[offset:])
	if err != nil {
		t.Fatal(err)
	}
	if string(p1.Payload) != "first\n" {
		t.Fatalf("p1 payload = %q", string(p1.Payload))
	}
	offset += p1.Consumed

	p2, err := Decode(buf[offset:])
	if err != nil {
		t.Fatal(err)
	}
	if string(p2.Payload) != "second\n" {
		t.Fatalf("p2 payload = %q", string(p2.Payload))
	}
	offset += p2.Consumed

	p3, err := Decode(buf[offset:])
	if err != nil {
		t.Fatal(err)
	}
	if p3.Type != Flush {
		t.Fatalf("p3 type = %v, want Flush", p3.Type)
	}
}

func TestEmptyPayloadEncodesAs0004(t *testing.T) {
	if _, err := Encode([]byte{}); err == nil {
		t.Fatal("Empty data is not allowed")
	}
}

func TestMaxSizePacket(t *testing.T) {
	payload := bytes.Repeat([]byte{0xab}, MaxPktLen-4)
	wire, err := Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) != MaxPktLen {
		t.Fatalf("wire len = %d, want %d", len(wire), MaxPktLen)
	}
	pkt, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Type != Data {
		t.Fatalf("type = %v, want Data", pkt.Type)
	}
	if !bytes.Equal(pkt.Payload, payload) {
		t.Fatal("payload mismatch")
	}
}

func TestEncodePacketTooLarge(t *testing.T) {
	payload := make([]byte, MaxPktLen-3) // total = MaxPktLen + 1
	_, err := Encode(payload)
	if err != ErrPacketTooLarge {
		t.Fatalf("expected ErrPacketTooLarge, got %v", err)
	}
}

func TestDecodeIncomplete(t *testing.T) {
	_, err := Decode([]byte("00"))
	if err != ErrIncomplete {
		t.Fatalf("expected ErrIncomplete, got %v", err)
	}

	// Says 10 bytes, only 4 available.
	_, err = Decode([]byte("000a"))
	if err != ErrIncomplete {
		t.Fatalf("expected ErrIncomplete, got %v", err)
	}
}

func TestDecodeInvalidLength3(t *testing.T) {
	_, err := Decode([]byte("0003anything"))
	if err != ErrInvalidLength {
		t.Fatalf("expected ErrInvalidLength, got %v", err)
	}
}

func TestPacketIterator(t *testing.T) {
	wire := []byte("000ahello\n" + "000bworld!\n" + "0008end\n" + "0000")
	r := bytes.NewReader(wire)
	iter := NewPacketIterator(r)

	p1, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if p1 == nil || p1.Type != Data || string(p1.Payload) != "hello\n" {
		t.Fatalf("p1 unexpected: %+v, data: %s", p1, p1.Payload)
	}

	p2, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if p2 == nil || p2.Type != Data || string(p2.Payload) != "world!\n" {
		t.Fatalf("p2 unexpected: %+v", p2)
	}

	p3, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if p3 == nil || p3.Type != Data || string(p3.Payload) != "end\n" {
		t.Fatalf("p3 unexpected: %+v", p3)
	}

	p4, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if p4 == nil || p4.Type != Flush {
		t.Fatalf("p4 expected Flush, got %+v", p4)
	}

	p5, err := iter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if p5 != nil {
		t.Fatalf("expected nil at EOF, got %+v", p5)
	}
}

func TestEncodeLineAndFlush(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeLine(&buf, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := EncodeFlush(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "000ahello\n0000" {
		t.Fatalf("got %q, want %q", buf.String(), "000ahello\\n0000")
	}
}
