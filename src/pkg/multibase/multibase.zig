//! Multibase decoding and multicodec key extraction.
//!
//! Multibase is a self-describing encoding: the first character of an encoded
//! string identifies which encoding was applied to the rest.
//!
//! Supported prefixes:
//!   z  base58btc
//!   f  hex lowercase
//!   F  hex uppercase
//!   u  base64url no-padding
//!   m  base64 standard with padding
//!   M  base64 standard no-padding
//!
//! After multibase-decoding, the raw bytes may carry a multicodec varint
//! prefix that identifies the key type:
//!   0xed 0x01  → Ed25519 public key  (32 payload bytes)
//!   0xec 0x01  → Ed25519 private key
//!   0x1205     → secp256k1 compressed public key  (33 payload bytes)
//!   0x1200     → P-256 public key
//!
//! The identity layer calls `decodeEd25519PubKey` which handles the full
//! `publicKeyMultibase` → `std.crypto.sign.Ed25519.PublicKey` pipeline.

const std = @import("std");
const Allocator = std.mem.Allocator;
const Ed25519 = std.crypto.sign.Ed25519;

pub const Encoding = enum {
    base58btc,
    hex_lower,
    hex_upper,
    base64url,
    base64std,
    base64std_nopad,
};

/// Decoded multibase payload. `data` is heap-allocated; the caller must free it.
pub const DecodeResult = struct {
    data: []u8,
    encoding: Encoding,
};

pub const KeyType = enum {
    ed25519_pub,
    ed25519_priv,
    secp256k1_pub,
    p256_pub,
    unknown,
};

/// Base58btc alphabet (Bitcoin variant).
const BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

/// Decode a base58btc-encoded string to raw bytes.
///
/// Leading `'1'` characters each map to one `0x00` byte in the output.
/// Returns `error.InvalidBase58Char` for any character not in the alphabet.
/// Caller frees the returned slice.
pub fn base58Decode(allocator: Allocator, encoded: []const u8) ![]u8 {
    // Count leading '1' → each becomes a leading 0x00 byte.
    var leading_zeros: usize = 0;
    for (encoded) |c| {
        if (c != '1') break;
        leading_zeros += 1;
    }

    // Working buffer large enough to accumulate the decoded integer.
    // Upper bound: input length is a safe over-estimate.
    const buf = try allocator.alloc(u8, encoded.len);
    defer allocator.free(buf);
    @memset(buf, 0);

    // Multiply-and-add over buf (big-endian big-integer arithmetic).
    for (encoded) |c| {
        const val: usize = blk: {
            for (BASE58_ALPHABET, 0..) |a, i| {
                if (a == c) break :blk i;
            }
            return error.InvalidBase58Char;
        };
        var carry: usize = val;
        var i: usize = buf.len;
        while (i > 0) {
            i -= 1;
            carry += 58 * @as(usize, buf[i]);
            buf[i] = @intCast(carry % 256);
            carry /= 256;
        }
    }

    // Strip leading zero bytes that were part of the arithmetic, then
    // re-prepend the count of leading '1' characters as 0x00 bytes.
    var start: usize = 0;
    while (start < buf.len and buf[start] == 0) : (start += 1) {}

    const result_len = leading_zeros + (buf.len - start);
    const result = try allocator.alloc(u8, result_len);
    @memset(result[0..leading_zeros], 0);
    @memcpy(result[leading_zeros..], buf[start..]);
    return result;
}

/// Encode raw bytes as a base58btc string.
///
/// Leading `0x00` bytes each become a leading `'1'` character.
/// Caller frees the returned slice.
pub fn base58Encode(allocator: Allocator, data: []const u8) ![]u8 {
    // Count leading 0x00 → each becomes '1'.
    var leading_zeros: usize = 0;
    for (data) |b| {
        if (b != 0) break;
        leading_zeros += 1;
    }

    // Working copy that we repeatedly divide by 58 in-place.
    const work = try allocator.alloc(u8, data.len);
    defer allocator.free(work);
    @memcpy(work, data);

    // Upper bound on output length: ceil(data.len × log₅₈(256)) + leading_zeros.
    // 1.37 × data.len + 2 is generous.
    const max_out = data.len * 2 + leading_zeros + 2;
    const tmp = try allocator.alloc(u8, max_out);
    defer allocator.free(tmp);
    var tmp_len: usize = 0;

    // Repeatedly divide work by 58; collect remainders in reverse order.
    while (!isAllZero(work)) {
        var carry: usize = 0;
        for (work) |*b| {
            carry = carry * 256 + @as(usize, b.*);
            b.* = @intCast(carry / 58);
            carry %= 58;
        }
        tmp[tmp_len] = BASE58_ALPHABET[carry];
        tmp_len += 1;
    }

    // Append a '1' for each leading zero byte.
    for (0..leading_zeros) |_| {
        tmp[tmp_len] = '1';
        tmp_len += 1;
    }

    // The digits were collected LSB-first; reverse to get the correct string.
    std.mem.reverse(u8, tmp[0..tmp_len]);
    return allocator.dupe(u8, tmp[0..tmp_len]);
}

/// Decode a multibase-encoded string.
///
/// The first character identifies the encoding. Returns `error.InvalidInput`
/// for an empty string, `error.UnsupportedEncoding` for an unknown prefix.
/// Caller frees `result.data`.
pub fn decode(allocator: Allocator, encoded: []const u8) !DecodeResult {
    if (encoded.len == 0) return error.InvalidInput;
    const prefix = encoded[0];
    const payload = encoded[1..];

    switch (prefix) {
        'z' => {
            const data = try base58Decode(allocator, payload);
            return .{ .data = data, .encoding = .base58btc };
        },
        'f' => {
            const data = try hexDecode(allocator, payload);
            return .{ .data = data, .encoding = .hex_lower };
        },
        'F' => {
            const data = try hexDecode(allocator, payload);
            return .{ .data = data, .encoding = .hex_upper };
        },
        'u' => {
            const data = try base64Decode(allocator, payload, std.base64.url_safe_no_pad.Decoder);
            return .{ .data = data, .encoding = .base64url };
        },
        'm' => {
            const data = try base64Decode(allocator, payload, std.base64.standard.Decoder);
            return .{ .data = data, .encoding = .base64std };
        },
        'M' => {
            const data = try base64Decode(allocator, payload, std.base64.standard_no_pad.Decoder);
            return .{ .data = data, .encoding = .base64std_nopad };
        },
        else => return error.UnsupportedEncoding,
    }
}

/// Extract the raw 32-byte Ed25519 public key from a multicodec-prefixed buffer.
///
/// Expects the buffer to start with `0xed 0x01` followed by exactly 32 bytes.
/// Returns `error.UnsupportedKeyType` if the prefix is wrong.
/// Returns `error.InvalidKeyLength` if the payload is not exactly 32 bytes.
pub fn extractEd25519PubKey(multicodec_bytes: []const u8) ![32]u8 {
    if (multicodec_bytes.len < 2 or
        multicodec_bytes[0] != 0xed or
        multicodec_bytes[1] != 0x01)
    {
        return error.UnsupportedKeyType;
    }
    const payload = multicodec_bytes[2..];
    if (payload.len != 32) return error.InvalidKeyLength;
    return payload[0..32].*;
}

/// Identify the key type encoded in a multicodec-prefixed buffer.
///
/// Returns `.unknown` rather than an error for unrecognised codes, allowing
/// callers to skip unsupported key types without treating them as fatal.
pub fn extractKeyType(data: []const u8) !KeyType {
    if (data.len < 2) return error.InvalidInput;
    const b0 = data[0];
    const b1 = data[1];
    // Two-byte varint codes used by did:key / Multicodec registry.
    if (b0 == 0xed and b1 == 0x01) return .ed25519_pub;
    if (b0 == 0xec and b1 == 0x01) return .ed25519_priv;
    if (b0 == 0x12 and b1 == 0x05) return .secp256k1_pub;
    if (b0 == 0x12 and b1 == 0x00) return .p256_pub;
    return .unknown;
}

/// Decode a `publicKeyMultibase` field from a DID document into an Ed25519
/// public key usable directly with `std.crypto.sign.Ed25519`.
///
/// Decoding flow:
///   multibase string → raw bytes → strip 0xed01 multicodec prefix → 32-byte key
///
/// All intermediate allocations are freed before returning. The returned
/// `Ed25519.PublicKey` is a value type (no heap allocation).
pub fn decodeEd25519PubKey(allocator: Allocator, multibase_str: []const u8) !Ed25519.PublicKey {
    const result = try decode(allocator, multibase_str);
    defer allocator.free(result.data);
    const key_bytes = try extractEd25519PubKey(result.data);
    return Ed25519.PublicKey.fromBytes(key_bytes);
}

fn hexDecode(allocator: Allocator, hex: []const u8) ![]u8 {
    if (hex.len % 2 != 0) return error.InvalidInput;
    const out = try allocator.alloc(u8, hex.len / 2);
    errdefer allocator.free(out);
    _ = std.fmt.hexToBytes(out, hex) catch return error.InvalidInput;
    return out;
}

fn base64Decode(
    allocator: Allocator,
    payload: []const u8,
    decoder: std.base64.Base64Decoder,
) ![]u8 {
    const size = decoder.calcSizeForSlice(payload) catch return error.InvalidInput;
    const out = try allocator.alloc(u8, size);
    errdefer allocator.free(out);
    decoder.decode(out, payload) catch return error.InvalidInput;
    return out;
}

fn isAllZero(buf: []const u8) bool {
    for (buf) |b| if (b != 0) return false;
    return true;
}

test "base58 roundtrip: encode then decode" {
    const allocator = std.testing.allocator;
    const original = [_]u8{ 0xed, 0x01 } ++ [_]u8{0x42} ** 32;
    const enc = try base58Encode(allocator, &original);
    defer allocator.free(enc);
    const dec = try base58Decode(allocator, enc);
    defer allocator.free(dec);
    try std.testing.expectEqualSlices(u8, &original, dec);
}

test "base58 decode known vector: empty string → empty bytes" {
    const allocator = std.testing.allocator;
    const r = try base58Decode(allocator, "");
    defer allocator.free(r);
    try std.testing.expectEqual(@as(usize, 0), r.len);
}

test "base58 decode known vector: '1' → [0x00]" {
    const allocator = std.testing.allocator;
    const r = try base58Decode(allocator, "1");
    defer allocator.free(r);
    try std.testing.expectEqualSlices(u8, &[_]u8{0x00}, r);
}

test "base58 decode known vector: '2g' → [0x61]" {
    const allocator = std.testing.allocator;
    const r = try base58Decode(allocator, "2g");
    defer allocator.free(r);
    try std.testing.expectEqualSlices(u8, &[_]u8{0x61}, r);
}

test "base58 decode: leading zeros '111' → [0,0,0]" {
    const allocator = std.testing.allocator;
    const r = try base58Decode(allocator, "111");
    defer allocator.free(r);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0, 0, 0 }, r);
}

test "base58 encode: [0x61] → '2g'" {
    const allocator = std.testing.allocator;
    const r = try base58Encode(allocator, &[_]u8{0x61});
    defer allocator.free(r);
    try std.testing.expectEqualSlices(u8, "2g", r);
}

test "base58 encode: [] → ''" {
    const allocator = std.testing.allocator;
    const r = try base58Encode(allocator, &[_]u8{});
    defer allocator.free(r);
    try std.testing.expectEqual(@as(usize, 0), r.len);
}

test "base58 encode: [0x00] → '1'" {
    const allocator = std.testing.allocator;
    const r = try base58Encode(allocator, &[_]u8{0x00});
    defer allocator.free(r);
    try std.testing.expectEqualSlices(u8, "1", r);
}

test "base58 invalid char '0' → error.InvalidBase58Char" {
    try std.testing.expectError(
        error.InvalidBase58Char,
        base58Decode(std.testing.allocator, "0abc"),
    );
}

test "base58 invalid char 'O' → error.InvalidBase58Char" {
    try std.testing.expectError(
        error.InvalidBase58Char,
        base58Decode(std.testing.allocator, "Oabc"),
    );
}

test "base58 invalid char 'I' → error.InvalidBase58Char" {
    try std.testing.expectError(
        error.InvalidBase58Char,
        base58Decode(std.testing.allocator, "Iabc"),
    );
}

test "base58 invalid char 'l' → error.InvalidBase58Char" {
    try std.testing.expectError(
        error.InvalidBase58Char,
        base58Decode(std.testing.allocator, "labc"),
    );
}

test "multibase decode: 'z' prefix dispatches to base58btc" {
    const allocator = std.testing.allocator;
    // Encode a known byte sequence and verify dispatch.
    const raw = [_]u8{ 0x01, 0x02, 0x03 };
    const b58 = try base58Encode(allocator, &raw);
    defer allocator.free(b58);
    const multibase_str = try std.mem.concat(allocator, u8, &.{ "z", b58 });
    defer allocator.free(multibase_str);

    const r = try decode(allocator, multibase_str);
    defer allocator.free(r.data);
    try std.testing.expectEqual(Encoding.base58btc, r.encoding);
    try std.testing.expectEqualSlices(u8, &raw, r.data);
}

test "multibase decode: 'f' prefix hex → 'hello'" {
    const allocator = std.testing.allocator;
    // 'f' + lowercase hex for "hello"
    const r = try decode(allocator, "f68656c6c6f");
    defer allocator.free(r.data);
    try std.testing.expectEqual(Encoding.hex_lower, r.encoding);
    try std.testing.expectEqualSlices(u8, "hello", r.data);
}

test "multibase decode: unknown prefix → error.UnsupportedEncoding" {
    try std.testing.expectError(
        error.UnsupportedEncoding,
        decode(std.testing.allocator, "x48656c6c6f"),
    );
}

test "multibase decode: empty string → error.InvalidInput" {
    try std.testing.expectError(
        error.InvalidInput,
        decode(std.testing.allocator, ""),
    );
}

test "multicodec extractEd25519PubKey: valid" {
    const raw = [_]u8{ 0xed, 0x01 } ++ [_]u8{0xab} ** 32;
    const key = try extractEd25519PubKey(&raw);
    try std.testing.expectEqualSlices(u8, &([_]u8{0xab} ** 32), &key);
}

test "multicodec extractEd25519PubKey: wrong prefix → error.UnsupportedKeyType" {
    const raw = [_]u8{ 0x00, 0x00 } ++ [_]u8{0} ** 32;
    try std.testing.expectError(error.UnsupportedKeyType, extractEd25519PubKey(&raw));
}

test "multicodec extractEd25519PubKey: short payload → error.InvalidKeyLength" {
    const raw = [_]u8{ 0xed, 0x01 } ++ [_]u8{0} ** 31; // 31 bytes, not 32
    try std.testing.expectError(error.InvalidKeyLength, extractEd25519PubKey(&raw));
}

test "multicodec extractKeyType: known types" {
    try std.testing.expectEqual(KeyType.ed25519_pub, try extractKeyType(&[_]u8{ 0xed, 0x01, 0, 0 }));
    try std.testing.expectEqual(KeyType.ed25519_priv, try extractKeyType(&[_]u8{ 0xec, 0x01, 0, 0 }));
    try std.testing.expectEqual(KeyType.secp256k1_pub, try extractKeyType(&[_]u8{ 0x12, 0x05, 0, 0 }));
    try std.testing.expectEqual(KeyType.p256_pub, try extractKeyType(&[_]u8{ 0x12, 0x00, 0, 0 }));
    try std.testing.expectEqual(KeyType.unknown, try extractKeyType(&[_]u8{ 0xff, 0xff, 0, 0 }));
}

test "full pipeline: real did:key ed25519 multibase string" {
    const allocator = std.testing.allocator;
    // Taken from the did:key method spec example key.
    const multibase = "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK";
    const pk = try decodeEd25519PubKey(allocator, multibase);
    // The decoded public key must be exactly 32 bytes (value type assertion).
    try std.testing.expectEqual(@as(usize, 32), pk.toBytes().len);
}

test "full pipeline roundtrip: generate keypair → multibase encode → decode back" {
    const allocator = std.testing.allocator;

    // Generate a fresh key pair.
    const stdlib_kp = Ed25519.KeyPair.generate();
    const pk_bytes: [32]u8 = stdlib_kp.public_key.toBytes();

    // Encode as multicodec bytes: 0xed01 ++ raw public key.
    const multicodec = [_]u8{ 0xed, 0x01 } ++ pk_bytes;

    // Base58-encode and prepend the 'z' multibase prefix.
    const b58 = try base58Encode(allocator, &multicodec);
    defer allocator.free(b58);
    const multibase_str = try std.mem.concat(allocator, u8, &.{ "z", b58 });
    defer allocator.free(multibase_str);

    // Decode back through the full pipeline.
    const recovered_pk = try decodeEd25519PubKey(allocator, multibase_str);
    try std.testing.expectEqualSlices(u8, &pk_bytes, &recovered_pk.toBytes());
}
