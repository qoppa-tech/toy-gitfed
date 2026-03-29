//! Git Smart HTTP server.
//!
//! Delegates to the `git http-backend` CGI process for all pack negotiation.
//! This layer is responsible for:
//!
//!   1. Parsing the request URL and routing to the correct service.
//!   2. Setting CGI environment variables expected by git-http-backend(1).
//!   3. Forwarding the request body to the backend and the response back.
//!
//! Supported endpoints:
//!   GET  /<repo>/info/refs?service=git-upload-pack
//!   GET  /<repo>/info/refs?service=git-receive-pack
//!   POST /<repo>/git-upload-pack
//!   POST /<repo>/git-receive-pack

const std = @import("std");
const Allocator = std.mem.Allocator;

pub const Config = struct {
    /// Absolute path to directory holding bare git repositories.
    repos_dir: []const u8,
    /// TCP address to listen on.
    address: std.net.Address,
};

pub const Server = struct {
    allocator: Allocator,
    config: Config,
    net_server: std.net.Server,

    pub fn init(allocator: Allocator, config: Config) !Server {
        const net_server = try config.address.listen(.{ .reuse_address = true });
        return .{
            .allocator = allocator,
            .config = config,
            .net_server = net_server,
        };
    }

    pub fn deinit(self: *Server) void {
        self.net_server.deinit();
    }

    /// Block, accepting and handling connections indefinitely.
    pub fn serve(self: *Server) !void {
        std.log.info("git http-backend listening on {f}", .{self.net_server.listen_address});
        while (true) {
            const conn = try self.net_server.accept();
            self.handleConnection(conn) catch |err| {
                std.log.err("connection error: {s}", .{@errorName(err)});
            };
        }
    }

    fn handleConnection(self: *Server, conn: std.net.Server.Connection) !void {
        defer conn.stream.close();

        // Buffers for the stream reader and writer must outlive the HTTP server.
        var reader_buf: [16 * 1024]u8 = undefined;
        var writer_buf: [16 * 1024]u8 = undefined;
        var stream_reader = std.net.Stream.Reader.init(conn.stream, &reader_buf);
        var stream_writer = std.net.Stream.Writer.init(conn.stream, &writer_buf);

        var http_srv = std.http.Server.init(stream_reader.interface(), &stream_writer.interface);

        while (true) {
            var req = http_srv.receiveHead() catch |err| switch (err) {
                error.HttpConnectionClosing => return,
                else => return err,
            };
            self.handleRequest(&req) catch |err| {
                std.log.err("request error: {s}", .{@errorName(err)});
                req.respond("Internal Server Error\n", .{ .status = .internal_server_error }) catch {};
                return;
            };
        }
    }

    fn handleRequest(self: *Server, req: *std.http.Server.Request) !void {
        const parsed = parseGitUrl(req.head.target) orelse {
            return req.respond("Not Found\n", .{ .status = .not_found });
        };

        const want: std.http.Method = switch (parsed.service) {
            .info_refs => .GET,
            .upload_pack, .receive_pack => .POST,
        };
        if (req.head.method != want) {
            return req.respond("Method Not Allowed\n", .{ .status = .method_not_allowed });
        }

        // Verify the repository exists on disk
        const repo_path = try std.fs.path.join(self.allocator, &.{ self.config.repos_dir, parsed.repo });
        defer self.allocator.free(repo_path);
        std.fs.accessAbsolute(repo_path, .{}) catch {
            return req.respond("Repository Not Found\n", .{ .status = .not_found });
        };

        // Read request body for POST endpoints
        var body: []const u8 = "";
        var body_buf: ?[]u8 = null;
        defer if (body_buf) |b| self.allocator.free(b);

        if (req.head.method == .POST) {
            body_buf = try self.readBody(req);
            body = body_buf.?;
        }

        // Invoke git http-backend and relay the CGI response
        const cgi_out = try self.runGitBackend(parsed, body);
        defer self.allocator.free(cgi_out);

        try self.sendCgiResponse(req, cgi_out, parsed);
    }

    fn readBody(self: *Server, req: *std.http.Server.Request) ![]u8 {
        var body_io_buf: [8192]u8 = undefined;
        const body_reader = req.readerExpectNone(&body_io_buf);
        return body_reader.allocRemaining(self.allocator, .unlimited);
    }

    fn runGitBackend(self: *Server, parsed: ParsedUrl, body: []const u8) ![]u8 {
        var env = std.process.EnvMap.init(self.allocator);
        defer env.deinit();

        // Inherit HOME so git can locate its config and SSH keys
        if (std.process.getEnvVarOwned(self.allocator, "HOME")) |home| {
            defer self.allocator.free(home);
            try env.put("HOME", home);
        } else |_| {}

        try env.put("GIT_PROJECT_ROOT", self.config.repos_dir);
        try env.put("GIT_HTTP_EXPORT_ALL", "1");
        try env.put("REQUEST_METHOD", if (body.len > 0) "POST" else "GET");

        // PATH_INFO tells git-http-backend which repo and endpoint to serve
        const path_info = try std.fmt.allocPrint(self.allocator, "/{s}{s}", .{
            parsed.repo,
            switch (parsed.service) {
                .info_refs => "/info/refs",
                .upload_pack => "/git-upload-pack",
                .receive_pack => "/git-receive-pack",
            },
        });
        defer self.allocator.free(path_info);
        try env.put("PATH_INFO", path_info);

        if (parsed.query.len > 0) {
            try env.put("QUERY_STRING", parsed.query);
        }

        if (body.len > 0) {
            try env.put("CONTENT_TYPE", parsed.service.requestContentType());
            const len_str = try std.fmt.allocPrint(self.allocator, "{d}", .{body.len});
            defer self.allocator.free(len_str);
            try env.put("CONTENT_LENGTH", len_str);
        }

        var child = std.process.Child.init(&.{ "git", "http-backend" }, self.allocator);
        child.env_map = &env;
        child.stdin_behavior = .Pipe;
        child.stdout_behavior = .Pipe;
        child.stderr_behavior = .Inherit;
        try child.spawn();
        errdefer _ = child.kill() catch {};

        // Write request body then close stdin so git sees EOF
        if (body.len > 0) {
            try child.stdin.?.writeAll(body);
        }
        child.stdin.?.close();
        child.stdin = null;

        // Collect the CGI response (headers + body) from stdout
        var out: std.ArrayList(u8) = .empty;
        errdefer out.deinit(self.allocator);
        var tmp: [16 * 1024]u8 = undefined;
        while (true) {
            const n = try child.stdout.?.read(&tmp);
            if (n == 0) break;
            try out.appendSlice(self.allocator, tmp[0..n]);
        }
        child.stdout.?.close();
        child.stdout = null;

        _ = try child.wait();
        return out.toOwnedSlice(self.allocator);
    }

    fn sendCgiResponse(
        self: *Server,
        req: *std.http.Server.Request,
        cgi_out: []const u8,
        parsed: ParsedUrl,
    ) !void {
        const sep = findHeaderSep(cgi_out) orelse {
            std.log.err("git http-backend produced no header/body separator", .{});
            return req.respond("Bad Gateway\n", .{ .status = .bad_gateway });
        };

        const headers_block = cgi_out[0..sep.headers_end];
        const body = cgi_out[sep.body_start..];

        var status: std.http.Status = .ok;
        var content_type: []const u8 = parsed.service.responseContentType();
        var extra: std.ArrayList(std.http.Header) = .empty;
        defer extra.deinit(self.allocator);

        // Parse CGI response headers line by line
        var lines = std.mem.splitSequence(u8, headers_block, "\n");
        while (lines.next()) |raw| {
            const line = std.mem.trimRight(u8, raw, "\r");
            if (line.len == 0) continue;
            const colon = std.mem.indexOfScalar(u8, line, ':') orelse continue;
            const name = std.mem.trim(u8, line[0..colon], " ");
            const value = std.mem.trim(u8, line[colon + 1 ..], " ");

            if (std.ascii.eqlIgnoreCase(name, "status")) {
                // CGI Status header looks like "200 OK" or "404 Not Found"
                if (value.len >= 3) {
                    const code = std.fmt.parseUnsigned(u10, value[0..3], 10) catch 200;
                    status = @enumFromInt(code);
                }
            } else if (std.ascii.eqlIgnoreCase(name, "content-type")) {
                content_type = value;
            } else {
                // Forward other CGI headers (Cache-Control, Pragma, etc.)
                try extra.append(self.allocator, .{ .name = name, .value = value });
            }
        }
        try extra.append(self.allocator, .{ .name = "Content-Type", .value = content_type });

        try req.respond(body, .{
            .status = status,
            .extra_headers = extra.items,
        });
    }
};

const GitService = enum {
    info_refs,
    upload_pack,
    receive_pack,

    /// Default Content-Type for the HTTP response (CGI output overrides this).
    fn responseContentType(self: GitService) []const u8 {
        return switch (self) {
            .info_refs => "application/x-git-upload-pack-advertisement",
            .upload_pack => "application/x-git-upload-pack-result",
            .receive_pack => "application/x-git-receive-pack-result",
        };
    }

    /// Content-Type sent in the CGI CONTENT_TYPE env var for POST requests.
    fn requestContentType(self: GitService) []const u8 {
        return switch (self) {
            .upload_pack => "application/x-git-upload-pack-request",
            .receive_pack => "application/x-git-receive-pack-request",
            .info_refs => "application/octet-stream",
        };
    }
};

const ParsedUrl = struct {
    /// Slice into the request target — no allocation needed.
    repo: []const u8,
    service: GitService,
    /// Raw query string (e.g. "service=git-upload-pack"), may be empty.
    query: []const u8,
};

/// Parse a Git Smart HTTP URL target into its components.
///
/// Accepted forms:
///   /<repo>/info/refs?service=git-upload-pack
///   /<repo>/info/refs?service=git-receive-pack
///   /<repo>/git-upload-pack
///   /<repo>/git-receive-pack
fn parseGitUrl(target: []const u8) ?ParsedUrl {
    var path = target;
    var query: []const u8 = "";

    if (std.mem.indexOfScalar(u8, target, '?')) |qi| {
        path = target[0..qi];
        query = target[qi + 1 ..];
    }

    if (path.len < 2 or path[0] != '/') return null;
    const p = path[1..]; // strip leading '/'

    if (std.mem.endsWith(u8, p, "/info/refs")) {
        const repo = p[0 .. p.len - "/info/refs".len];
        if (repo.len == 0) return null;
        // Smart HTTP requires a service= query parameter
        if (!std.mem.startsWith(u8, query, "service=git-")) return null;
        return .{ .repo = repo, .service = .info_refs, .query = query };
    }

    if (std.mem.endsWith(u8, p, "/git-upload-pack")) {
        const repo = p[0 .. p.len - "/git-upload-pack".len];
        if (repo.len == 0) return null;
        return .{ .repo = repo, .service = .upload_pack, .query = "" };
    }

    if (std.mem.endsWith(u8, p, "/git-receive-pack")) {
        const repo = p[0 .. p.len - "/git-receive-pack".len];
        if (repo.len == 0) return null;
        return .{ .repo = repo, .service = .receive_pack, .query = "" };
    }

    return null;
}

const HeaderSep = struct {
    headers_end: usize,
    body_start: usize,
};

fn findHeaderSep(data: []const u8) ?HeaderSep {
    if (std.mem.indexOf(u8, data, "\r\n\r\n")) |i| {
        return .{ .headers_end = i, .body_start = i + 4 };
    }
    if (std.mem.indexOf(u8, data, "\n\n")) |i| {
        return .{ .headers_end = i, .body_start = i + 2 };
    }
    return null;
}

test "parseGitUrl: info/refs upload-pack" {
    const p = parseGitUrl("/myrepo.git/info/refs?service=git-upload-pack") orelse
        return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("myrepo.git", p.repo);
    try std.testing.expectEqual(GitService.info_refs, p.service);
    try std.testing.expectEqualStrings("service=git-upload-pack", p.query);
}

test "parseGitUrl: info/refs receive-pack" {
    const p = parseGitUrl("/myrepo.git/info/refs?service=git-receive-pack") orelse
        return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("myrepo.git", p.repo);
    try std.testing.expectEqual(GitService.info_refs, p.service);
}

test "parseGitUrl: upload-pack POST endpoint" {
    const p = parseGitUrl("/myrepo.git/git-upload-pack") orelse
        return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("myrepo.git", p.repo);
    try std.testing.expectEqual(GitService.upload_pack, p.service);
}

test "parseGitUrl: receive-pack POST endpoint" {
    const p = parseGitUrl("/myrepo.git/git-receive-pack") orelse
        return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("myrepo.git", p.repo);
    try std.testing.expectEqual(GitService.receive_pack, p.service);
}

test "parseGitUrl: nested repo path" {
    const p = parseGitUrl("/org/team/repo.git/git-upload-pack") orelse
        return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("org/team/repo.git", p.repo);
}

test "parseGitUrl: rejects unknown paths" {
    try std.testing.expectEqual(@as(?ParsedUrl, null), parseGitUrl("/"));
    try std.testing.expectEqual(@as(?ParsedUrl, null), parseGitUrl("/info/refs"));
    try std.testing.expectEqual(@as(?ParsedUrl, null), parseGitUrl("/repo/objects/info/packs"));
    // info/refs without service= query is dumb HTTP – not handled here
    try std.testing.expectEqual(@as(?ParsedUrl, null), parseGitUrl("/repo/info/refs"));
}

test "findHeaderSep: CRLF separators" {
    const data = "Content-Type: text/plain\r\nStatus: 200 OK\r\n\r\nbody";
    const sep = findHeaderSep(data) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("body", data[sep.body_start..]);
}

test "findHeaderSep: LF-only separators" {
    const data = "Content-Type: text/plain\n\nbody";
    const sep = findHeaderSep(data) orelse return error.TestUnexpectedNull;
    try std.testing.expectEqualStrings("body", data[sep.body_start..]);
}

test "findHeaderSep: no separator returns null" {
    try std.testing.expectEqual(@as(?HeaderSep, null), findHeaderSep("no separator here"));
}
