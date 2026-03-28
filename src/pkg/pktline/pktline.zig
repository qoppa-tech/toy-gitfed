//! Git pkt-line framing (git-protocol v0/v1/v2).
//!
//! A pkt-line is a 4-hex-digit length prefix (inclusive of those 4 bytes)
//! followed by a payload. Three magic values carry no payload:
//!
//!   0000  flush packet      – stream/section delimiter
//!   0001  delimiter packet  – section separator (protocol v2)
//!   0002  response-end      – protocol v2 end-of-response
//!
//! Minimum data packet length: 5  (4-byte prefix + 1 byte payload)
//! Maximum data packet length: 65520  (0xFFF0)
//!
//! This module is transport-layer only; it has no knowledge of Git semantics.

const std = @import("std");
const Allocator = std.mem.Allocator;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// Written to the wire to delimit a request/response stream.
pub const flush_pkt = "0000";
/// Section separator used in protocol v2.
pub const delim_pkt = "0001";
/// Response-end marker used in protocol v2.
pub const response_end_pkt = "0002";

/// Maximum total wire length of a single data packet (4 hex + payload).
pub const max_pkt_len: usize = 65520;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

pub const PacketType = enum { data, flush, delimiter, response_end };

/// A decoded pkt-line frame.
pub const Packet = struct {
    pkt_type: PacketType,
    /// Raw payload bytes. Empty for special packets (flush/delimiter/response-end).
    /// Points into the caller-supplied input buffer – no allocation is performed.
    payload: []const u8,
    /// How many bytes were consumed from the front of the input buffer.
    consumed: usize,
};

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

pub const EncodeError = error{
    /// The payload would produce a packet longer than 65520 bytes.
    PacketTooLarge,
    OutOfMemory,
};

pub const DecodeError = error{
    /// Fewer than 4 bytes available, or declared length exceeds available data.
    Incomplete,
    /// Declared length is 1, 2, or 3 – impossible values.
    InvalidLength,
    /// The hex prefix contains non-hex characters.
    InvalidCharacter,
    Overflow,
};

// ---------------------------------------------------------------------------
// Encode
// ---------------------------------------------------------------------------

/// Encode `data` as a pkt-line frame. Returns an owned slice; caller must free.
///
/// Returns `error.PacketTooLarge` if `data.len + 4 > 65520`.
pub fn encode(allocator: Allocator, data: []const u8) EncodeError![]u8 {
    const total = data.len + 4;
    if (total > max_pkt_len) return error.PacketTooLarge;

    const out = try allocator.alloc(u8, total);
    _ = std.fmt.bufPrint(out[0..4], "{x:0>4}", .{total}) catch unreachable;
    @memcpy(out[4..], data);
    return out;
}

/// Write the 4-byte flush packet `"0000"` to `writer`.
pub fn encodeFlush(writer: anytype) !void {
    try writer.writeAll(flush_pkt);
}

/// Write a length-prefixed pkt-line frame to `writer` without allocating.
///
/// Returns `error.PacketTooLarge` if `data.len + 4 > 65520`.
pub fn encodeLine(writer: anytype, data: []const u8) !void {
    const total = data.len + 4;
    if (total > max_pkt_len) return error.PacketTooLarge;

    var prefix: [4]u8 = undefined;
    _ = std.fmt.bufPrint(&prefix, "{x:0>4}", .{total}) catch unreachable;
    try writer.writeAll(&prefix);
    try writer.writeAll(data);
}

// ---------------------------------------------------------------------------
// Decode
// ---------------------------------------------------------------------------

/// Decode one pkt-line from the front of `data`.
///
/// The returned `Packet.payload` is a sub-slice of `data`; no allocation is
/// performed. Advance your read position by `Packet.consumed` before calling
/// `decode` again.
pub fn decode(data: []const u8) DecodeError!Packet {
    if (data.len < 4) return error.Incomplete;

    const raw_len = std.fmt.parseUnsigned(u16, data[0..4], 16) catch |e| switch (e) {
        error.InvalidCharacter => return error.InvalidCharacter,
        error.Overflow => return error.Overflow,
    };

    // Special zero-payload packets
    switch (raw_len) {
        0 => return .{ .pkt_type = .flush, .payload = "", .consumed = 4 },
        1 => return .{ .pkt_type = .delimiter, .payload = "", .consumed = 4 },
        2 => return .{ .pkt_type = .response_end, .payload = "", .consumed = 4 },
        3 => return error.InvalidLength,
        else => {},
    }

    // raw_len >= 4: data packet
    if (data.len < raw_len) return error.Incomplete;

    return .{
        .pkt_type = .data,
        .payload = data[4..raw_len],
        .consumed = raw_len,
    };
}

// ---------------------------------------------------------------------------
// PacketIterator
// ---------------------------------------------------------------------------

/// A streaming pkt-line iterator over any reader that exposes `.read([]u8) !usize`.
///
/// ```zig
/// var iter = pktline.PacketIterator(@TypeOf(reader)).init(reader);
/// while (try iter.next()) |pkt| { ... }
/// ```
pub fn PacketIterator(comptime ReaderType: type) type {
    return struct {
        reader: ReaderType,
        // Stack buffer large enough for the maximum packet size.
        buf: [max_pkt_len]u8 = undefined,

        const Self = @This();

        pub fn init(reader: ReaderType) Self {
            return .{ .reader = reader };
        }

        /// Read the next packet from the stream. Returns `null` at EOF
        /// (zero bytes returned on the first read of the 4-byte prefix).
        pub fn next(self: *Self) !?Packet {
            // Read exactly 4 bytes for the length prefix.
            const n = try readAtLeast(self.reader, self.buf[0..4], 4);
            if (n == 0) return null; // clean EOF before any prefix bytes

            const raw_len = std.fmt.parseUnsigned(u16, self.buf[0..4], 16) catch
                return error.InvalidLength;

            // Special packets: no payload follows.
            switch (raw_len) {
                0 => return .{ .pkt_type = .flush, .payload = "", .consumed = 4 },
                1 => return .{ .pkt_type = .delimiter, .payload = "", .consumed = 4 },
                2 => return .{ .pkt_type = .response_end, .payload = "", .consumed = 4 },
                3 => return error.InvalidLength,
                else => {},
            }

            // Data packet: read the remaining (raw_len - 4) payload bytes.
            const payload_len = raw_len - 4;
            try readExact(self.reader, self.buf[4..][0..payload_len]);

            return .{
                .pkt_type = .data,
                // Slice points into self.buf – valid until the next call to next().
                .payload = self.buf[4..][0..payload_len],
                .consumed = raw_len,
            };
        }
    };
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/// Read at least `min` bytes into `buf`, returning the total read.
/// Returns 0 only when EOF is reached before any bytes are read.
fn readAtLeast(reader: anytype, buf: []u8, min: usize) !usize {
    var total: usize = 0;
    while (total < min) {
        const n = try reader.read(buf[total..]);
        if (n == 0) {
            if (total == 0) return 0; // EOF before first byte
            return error.EndOfStream;
        }
        total += n;
    }
    return total;
}

/// Read exactly `buf.len` bytes, returning `error.EndOfStream` on short read.
fn readExact(reader: anytype, buf: []u8) !void {
    var i: usize = 0;
    while (i < buf.len) {
        const n = try reader.read(buf[i..]);
        if (n == 0) return error.EndOfStream;
        i += n;
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test "encode hello\\n → 000ahello\\n" {
    const allocator = std.testing.allocator;
    const pkt = try encode(allocator, "hello\n");
    defer allocator.free(pkt);
    try std.testing.expectEqualSlices(u8, "000ahello\n", pkt);
}

test "decode 000ahello\\n" {
    const pkt = try decode("000ahello\n");
    try std.testing.expectEqual(PacketType.data, pkt.pkt_type);
    try std.testing.expectEqualSlices(u8, "hello\n", pkt.payload);
    try std.testing.expectEqual(@as(usize, 10), pkt.consumed);
}

test "decode flush packet 0000" {
    const pkt = try decode("0000");
    try std.testing.expectEqual(PacketType.flush, pkt.pkt_type);
    try std.testing.expectEqualSlices(u8, "", pkt.payload);
    try std.testing.expectEqual(@as(usize, 4), pkt.consumed);
}

test "decode delimiter packet 0001" {
    const pkt = try decode("0001");
    try std.testing.expectEqual(PacketType.delimiter, pkt.pkt_type);
    try std.testing.expectEqual(@as(usize, 4), pkt.consumed);
}

test "decode response_end packet 0002" {
    const pkt = try decode("0002");
    try std.testing.expectEqual(PacketType.response_end, pkt.pkt_type);
    try std.testing.expectEqual(@as(usize, 4), pkt.consumed);
}

test "encode/decode roundtrip" {
    const allocator = std.testing.allocator;
    const original = "want abc123\n";

    const wire = try encode(allocator, original);
    defer allocator.free(wire);

    const pkt = try decode(wire);
    try std.testing.expectEqual(PacketType.data, pkt.pkt_type);
    try std.testing.expectEqualSlices(u8, original, pkt.payload);
}

test "multiple packets in buffer via consumed offset" {
    const buf = "0009first\n000bsecond\n0000";
    var offset: usize = 0;

    const p1 = try decode(buf[offset..]);
    try std.testing.expectEqualSlices(u8, "first\n", p1.payload);
    offset += p1.consumed;

    const p2 = try decode(buf[offset..]);
    try std.testing.expectEqualSlices(u8, "second\n", p2.payload);
    offset += p2.consumed;

    const p3 = try decode(buf[offset..]);
    try std.testing.expectEqual(PacketType.flush, p3.pkt_type);
}

test "empty payload encodes as 0004" {
    const allocator = std.testing.allocator;
    const pkt = try encode(allocator, "");
    defer allocator.free(pkt);
    try std.testing.expectEqualSlices(u8, "0004", pkt);

    const decoded = try decode("0004");
    try std.testing.expectEqual(PacketType.data, decoded.pkt_type);
    try std.testing.expectEqualSlices(u8, "", decoded.payload);
    try std.testing.expectEqual(@as(usize, 4), decoded.consumed);
}

test "max size packet encodes and decodes" {
    const allocator = std.testing.allocator;
    const payload = try allocator.alloc(u8, max_pkt_len - 4);
    defer allocator.free(payload);
    @memset(payload, 0xab);

    const wire = try encode(allocator, payload);
    defer allocator.free(wire);

    try std.testing.expectEqual(max_pkt_len, wire.len);

    const pkt = try decode(wire);
    try std.testing.expectEqual(PacketType.data, pkt.pkt_type);
    try std.testing.expectEqualSlices(u8, payload, pkt.payload);
    try std.testing.expectEqual(max_pkt_len, pkt.consumed);
}

test "encode returns PacketTooLarge for oversized payload" {
    const allocator = std.testing.allocator;
    const payload = try allocator.alloc(u8, max_pkt_len - 3); // total = max_pkt_len + 1
    defer allocator.free(payload);
    try std.testing.expectError(error.PacketTooLarge, encode(allocator, payload));
}

test "decode returns Incomplete for short input" {
    try std.testing.expectError(error.Incomplete, decode("00"));
    try std.testing.expectError(error.Incomplete, decode("000a")); // says 10 bytes, only 4 available
}

test "decode returns InvalidLength for length 1, 2, 3" {
    // 0003 is impossible (minimum data pkt is 0005 or special 0000–0002)
    try std.testing.expectError(error.InvalidLength, decode("0003anything"));
}

test "PacketIterator reads 3 packets then flush" {
    // Build wire: "000ahello\n" + "000bworld!\n" + "0006end\n" + "0000"
    const wire = "000ahello\n" ++ "000bworld!\n" ++ "0009end\n" ++ "0000";
    var fbs = std.io.fixedBufferStream(wire);
    var iter = PacketIterator(@TypeOf(fbs.reader())).init(fbs.reader());

    const p1 = (try iter.next()) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqual(PacketType.data, p1.pkt_type);
    try std.testing.expectEqualSlices(u8, "hello\n", p1.payload);

    const p2 = (try iter.next()) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqual(PacketType.data, p2.pkt_type);
    try std.testing.expectEqualSlices(u8, "world!\n", p2.payload);

    const p3 = (try iter.next()) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqual(PacketType.data, p3.pkt_type);
    try std.testing.expectEqualSlices(u8, "end\n", p3.payload);

    const p4 = (try iter.next()) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqual(PacketType.flush, p4.pkt_type);

    // After flush, stream should be exhausted → null
    const p5 = try iter.next();
    try std.testing.expectEqual(@as(?Packet, null), p5);
}

test "encodeLine and encodeFlush write to writer" {
    var buf: [64]u8 = undefined;
    var fbs = std.io.fixedBufferStream(&buf);
    const w = fbs.writer();

    try encodeLine(w, "hello\n");
    try encodeFlush(w);

    const written = fbs.getWritten();
    try std.testing.expectEqualSlices(u8, "000ahello\n0000", written);
}
