// Package pktline implements Git pkt-line framing (git-protocol v0/v1/v2).
//
// A pkt-line is a 4-hex-digit length prefix (inclusive of those 4 bytes)
// followed by a payload. Three magic values carry no payload:
//
//	0000  flush packet      – stream/section delimiter
//	0001  delimiter packet  – section separator (protocol v2)
//	0002  response-end      – protocol v2 end-of-response
//
// Minimum data packet length: 5  (4-byte prefix + 1 byte payload)
// Maximum data packet length: 65520  (0xFFF0)
//
// This module is transport-layer only; it has no knowledge of Git semantics.
package pktline

import (
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	// FlushPkt is written to the wire to delimit a request/response stream.
	FlushPkt = "0000"
	// DelimPkt is a section separator used in protocol v2.
	DelimPkt = "0001"
	// ResponseEndPkt is a response-end marker used in protocol v2.
	ResponseEndPkt = "0002"

	// MaxPktLen is the maximum total wire length of a single data packet.
	MaxPktLen = 65520
)

// PacketType identifies the kind of pkt-line frame.
type PacketType int

const (
	Data PacketType = iota
	Flush
	Delimiter
	ResponseEnd
)

// Packet is a decoded pkt-line frame.
type Packet struct {
	Type PacketType
	// Data payload by itself
	Payload []byte
	// Total consumed data
	Consumed int
}

var (
	ErrPacketTooLarge   = errors.New("packet too large")
	ErrIncomplete       = errors.New("incomplete packet")
	ErrInvalidLength    = errors.New("invalid length")
	ErrInvalidCharacter = errors.New("invalid character")
	ErrEmptyPacket      = errors.New("empty packet")
)

// Encode data into pkt-line protocol frame and returns the wire bytes.
func Encode(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyPacket
	}

	total := len(data) + 4
	if total > MaxPktLen {
		return nil, ErrPacketTooLarge
	}
	out := make([]byte, total)
	copy(out[:4], fmt.Sprintf("%04x", total))
	copy(out[4:], data)
	return out, nil
}

// EncodeLine writes a length-prefixed pkt-line frame to w.
func EncodeLine(w io.Writer, data []byte) error {
	total := len(data) + 4

	if total > MaxPktLen {
		return ErrPacketTooLarge
	}

	if total == 0 {
		return ErrEmptyPacket
	}

	prefix := fmt.Sprintf("%04x", total)
	if _, err := w.Write([]byte(prefix)); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// EncodeFlush writes the 4-byte flush packet "0000" to w.
func EncodeFlush(w io.Writer) error {
	_, err := w.Write([]byte(FlushPkt))
	return err
}

// Decode decodes one pkt-line from the front of data.
// The returned Packet.Payload is a sub-slice of data; no allocation is performed.
func Decode(data []byte) (Packet, error) {
	if len(data) < 4 {
		return Packet{}, ErrIncomplete
	}

	rawLen, err := strconv.ParseUint(string(data[:4]), 16, 16)
	if err != nil {
		return Packet{}, ErrInvalidCharacter
	}

	switch rawLen {
	case 0:
		return Packet{Type: Flush, Consumed: 4}, nil
	case 1:
		return Packet{Type: Delimiter, Consumed: 4}, nil
	case 2:
		return Packet{Type: ResponseEnd, Consumed: 4}, nil
	case 3:
		return Packet{}, ErrInvalidLength
	case 4:
		return Packet{}, ErrEmptyPacket
	}

	if len(data) < int(rawLen) {
		return Packet{}, ErrIncomplete
	}

	return Packet{
		Type:     Data,
		Payload:  data[4:rawLen],
		Consumed: int(rawLen),
	}, nil
}

// PktLineStream is a streaming pkt-line iterator over an io.Reader.
type PktLineStream struct {
	reader io.Reader
	buf    [MaxPktLen]byte
}

// NewPacketIterator creates a new PacketIterator.
func NewPacketIterator(r io.Reader) *PktLineStream {
	return &PktLineStream{reader: r}
}

// Next reads the next packet from the stream. Returns nil at EOF.
func (it *PktLineStream) Next() (*Packet, error) {
	// Read exactly 4 bytes for the length prefix.
	rawPktLen, err := io.ReadFull(it.reader, it.buf[:4])

	if err == io.EOF && rawPktLen == 0 {
		// stream ended prematurely
		return nil, nil // clean EOF
	}

	if err != nil {
		return nil, err
	}

	pktDataLen, err := strconv.ParseUint(string(it.buf[:4]), 16, 16)
	if err != nil {
		return nil, ErrInvalidLength
	}

	switch pktDataLen {
	case 0:
		return &Packet{Type: Flush, Consumed: 4}, nil
	case 1:
		return &Packet{Type: Delimiter, Consumed: 4}, nil
	case 2:
		return &Packet{Type: ResponseEnd, Consumed: 4}, nil
	case 3:
		return nil, ErrInvalidLength
	case 4:
		// A package with data len of 4 (a.k.a. empty data) is invalid
		return nil, ErrEmptyPacket
	}

	// Reads the buffer skipping the first 4 bytes of len data
	if _, err := io.ReadFull(it.reader, it.buf[4:pktDataLen]); err != nil {
		return nil, err
	}

	return &Packet{
		Type:     Data,
		Payload:  it.buf[4:pktDataLen],
		Consumed: int(rawPktLen),
	}, nil
}
