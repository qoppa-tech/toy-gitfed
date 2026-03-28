//! Zlib inflate/deflate helpers for Git objects and packfile entries.
//!
//! Note on compression: `std.compress.flate.Compress` is not yet complete in
//! Zig 0.15.2. The `compress` function therefore emits deflate store blocks
//! (BTYPE=00), which is valid RFC 1950 zlib that any conforming decompressor
//! will accept. The `decompress` path is fully functional and can decode both
//! store and compressed streams.

const std = @import("std");
const Allocator = std.mem.Allocator;
const flate = std.compress.flate;

/// Inflate zlib-compressed bytes. Returns an owned slice; caller must free.
///
/// Reads the decompressor output in chunks to avoid materialising the full
/// stream in memory before handing it to the caller.
pub fn decompress(allocator: Allocator, compressed: []const u8) ![]u8 {
    var in: std.Io.Reader = .fixed(compressed);
    return decompressStream(&in, allocator);
}

/// Deflate raw bytes to a zlib-wrapped stream. Returns an owned slice;
/// caller must free.
///
/// Output uses deflate store blocks (BTYPE=00): no LZ77 compression is
/// applied, but the result is fully conformant RFC 1950 zlib with a valid
/// Adler-32 checksum. Once `std.compress.flate.Compress` matures this
/// function can be upgraded to real DEFLATE without changing the public API.
pub fn compress(allocator: Allocator, data: []const u8) ![]u8 {
    // Upper bound: 2 header + ceil(len/65535)*(5+65535) + 4 footer.
    var out = std.ArrayList(u8){};
    errdefer out.deinit(allocator);

    // RFC 1950 §2.2  CMF=0x78 (CM=8 deflate, CINFO=7 → 32K window)
    // FLG=0x01: FLEVEL=00 (fastest), FDICT=0, FCHECK=1  (0x7801 % 31 == 0)
    try out.appendSlice(allocator, &[_]u8{ 0x78, 0x01 });

    if (data.len == 0) {
        // Single empty store block: BFINAL=1, BTYPE=00, LEN=0, NLEN=~0
        try out.appendSlice(allocator, &[_]u8{ 0x01, 0x00, 0x00, 0xff, 0xff });
    } else {
        var offset: usize = 0;
        while (offset < data.len) {
            const chunk_len: u16 = @intCast(@min(data.len - offset, 65535));
            const is_final = (offset + chunk_len >= data.len);

            // BFINAL (1 bit) | BTYPE=00 (2 bits), packed in one byte
            try out.append(allocator, if (is_final) 0x01 else 0x00);

            var lb: [2]u8 = undefined;
            std.mem.writeInt(u16, &lb, chunk_len, .little);
            try out.appendSlice(allocator, &lb);

            var nlb: [2]u8 = undefined;
            std.mem.writeInt(u16, &nlb, ~chunk_len, .little);
            try out.appendSlice(allocator, &nlb);

            try out.appendSlice(allocator, data[offset..][0..chunk_len]);
            offset += chunk_len;
        }
    }

    // RFC 1950 §2.2  Adler-32 of the uncompressed data, big-endian.
    var ck: [4]u8 = undefined;
    std.mem.writeInt(u32, &ck, std.hash.Adler32.hash(data), .big);
    try out.appendSlice(allocator, &ck);

    return out.toOwnedSlice(allocator);
}

/// Inflate zlib data from an `std.Io.Reader`. Returns an owned slice;
/// caller must free.
///
/// Use this when the compressed data is already being streamed (e.g. from a
/// network socket or file) to avoid a redundant copy into a temporary buffer.
pub fn decompressStream(reader: *std.Io.Reader, allocator: Allocator) ![]u8 {
    var dc: flate.Decompress = .init(reader, .zlib, &.{});
    var out: std.Io.Writer.Allocating = .init(allocator);
    errdefer out.deinit();
    _ = try dc.reader.streamRemaining(&out.writer);
    return out.toOwnedSlice();
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test "roundtrip: compress then decompress" {
    const allocator = std.testing.allocator;
    const original = "The quick brown fox jumps over the lazy dog";

    const compressed = try compress(allocator, original);
    defer allocator.free(compressed);

    const restored = try decompress(allocator, compressed);
    defer allocator.free(restored);

    try std.testing.expectEqualSlices(u8, original, restored);
}

test "decompress known Git zlib blob" {
    // python3: zlib.compress(b"hello") → bytes confirmed against CPython
    const compressed = "\x78\x9c\xcb\x48\xcd\xc9\xc9\x07\x00\x06\x2c\x02\x15";
    const allocator = std.testing.allocator;

    const out = try decompress(allocator, compressed);
    defer allocator.free(out);

    try std.testing.expectEqualSlices(u8, "hello", out);
}

test "compress and decompress empty input" {
    const allocator = std.testing.allocator;

    const compressed = try compress(allocator, "");
    defer allocator.free(compressed);

    const restored = try decompress(allocator, compressed);
    defer allocator.free(restored);

    try std.testing.expectEqualSlices(u8, "", restored);
}

test "large input roundtrip exercises multi-block path" {
    // 64 KiB + 1 byte forces two store blocks (limit is 65535 per block).
    const allocator = std.testing.allocator;
    const size = 64 * 1024 + 1;

    const data = try allocator.alloc(u8, size);
    defer allocator.free(data);
    for (data, 0..) |*b, i| b.* = @truncate(i);

    const compressed = try compress(allocator, data);
    defer allocator.free(compressed);

    const restored = try decompress(allocator, compressed);
    defer allocator.free(restored);

    try std.testing.expectEqualSlices(u8, data, restored);
}

test "decompressStream from Io.Reader" {
    const allocator = std.testing.allocator;
    const original = "streaming test data";

    const compressed = try compress(allocator, original);
    defer allocator.free(compressed);

    var reader: std.Io.Reader = .fixed(compressed);
    const restored = try decompressStream(&reader, allocator);
    defer allocator.free(restored);

    try std.testing.expectEqualSlices(u8, original, restored);
}
