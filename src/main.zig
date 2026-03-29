const std = @import("std");
const toy_gitfed = @import("toy_gitfed");
const http = @import("api/http/http.zig");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 2) {
        std.log.err("usage: {s} <repos-dir> [port]", .{args[0]});
        std.process.exit(1);
    }

    const repos_dir = args[1];
    const port: u16 = if (args.len >= 3) try std.fmt.parseInt(u16, args[2], 10) else 8080;

    // TODO: Parse IP based on env
    const address = try std.net.Address.parseIp("0.0.0.0", port);
    var server = try http.Server.init(allocator, .{
        .repos_dir = repos_dir,
        .address = address,
    });
    defer server.deinit();

    try server.serve();
}
