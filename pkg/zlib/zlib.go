// Package zlib provides inflate/deflate helpers for Git objects and packfile entries.
package zlib

import (
	"bytes"
	"compress/zlib"
	"io"
)

// Decompress inflates zlib-compressed bytes.
func Decompress(compressed []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Compress deflates raw bytes to a zlib-wrapped stream.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecompressStream inflates zlib data from an io.Reader.
func DecompressStream(r io.Reader) ([]byte, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
