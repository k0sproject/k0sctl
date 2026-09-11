#!/usr/bin/env python3
"""A small HTTP file server for the k0sctl files smoke test.

It serves the files in ROOT with the response headers k0sctl compares between
applies to decide whether a download is still needed, and it honors the open
ended range requests curl makes when it continues an interrupted download.

Two knobs make the interesting cases reachable from a shell script:

  - a path under /noetag/ is served without an ETag, which is what makes k0sctl
    fall back to comparing Last-Modified
  - a <file>.drop marker beside a served file cuts the next transfer of that
    file short and closes the connection, which is what an interrupted download
    looks like to curl. The marker is consumed by the transfer that trips over
    it, so the attempt that follows can succeed.

Every request is appended to LOG as "<method> <path> <status>", with a trailing
" dropped" for a transfer that was cut short, for the test to assert on.
"""

import email.utils
import http.server
import os
import socketserver
import threading

# The defaults are where the smoke test puts things on the server machine; the
# environment overrides exist so that the script can be exercised locally.
ROOT = os.environ.get("FILESRV_ROOT", "/srv/files")
LOG = os.environ.get("FILESRV_LOG", "/srv/requests.log")
ADDRESS = ("0.0.0.0", int(os.environ.get("FILESRV_PORT", "8080")))

# How much of a transfer is delivered before a dropped one gives up.
DROP_RATIO = 0.4
CHUNK = 64 * 1024

log_lock = threading.Lock()


def record(line):
    with log_lock, open(LOG, "a") as f:
        f.write(line + "\n")


def parse_range(header, size):
    """Return the first byte the client asked for, or None when the range can
    not be served. Only the forms curl and wget send are understood."""
    if not header.startswith("bytes="):
        return None
    spec = header[len("bytes="):].split(",")[0].strip()
    first = spec.split("-")[0]
    if not first.isdigit():
        return None
    start = int(first)
    if start >= size:
        return None
    return start


def consume_drop_marker(name):
    """Report whether this transfer should be cut short, clearing the marker so
    that only one transfer is affected by it."""
    marker = name + ".drop"
    try:
        os.remove(marker)
    except FileNotFoundError:
        return False
    return True


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "k0sctl-smoke-filesrv/1"

    def log_message(self, fmt, *args):
        # The request log this test reads is the one record() writes.
        pass

    def resolve(self):
        """Return the path on disk for the request and whether it gets an ETag,
        or (None, ...) when the request is not for a served file."""
        path = self.path.split("?", 1)[0]
        etag = True
        if path.startswith("/noetag/"):
            path = path[len("/noetag/"):]
            etag = False
        elif path.startswith("/files/"):
            path = path[len("/files/"):]
        else:
            return None, etag
        # Only plain names directly under ROOT are served, which is all the test
        # asks for and leaves no room for a traversal.
        if not path or "/" in path or path.startswith("."):
            return None, etag
        return os.path.join(ROOT, path), etag

    def do_HEAD(self):
        self.serve(with_body=False)

    def do_GET(self):
        self.serve(with_body=True)

    def serve(self, with_body):
        name, with_etag = self.resolve()
        if name is None or not os.path.isfile(name):
            self.respond(404)
            return

        stat = os.stat(name)
        start = 0
        if header := self.headers.get("Range"):
            start = parse_range(header, stat.st_size)
            if start is None:
                self.send_response(416)
                self.send_header("Content-Range", "bytes */%d" % stat.st_size)
                self.send_header("Content-Length", "0")
                self.end_headers()
                record("%s %s 416" % (self.command, self.path))
                return

        status = 206 if start else 200
        length = stat.st_size - start
        self.send_response(status)
        self.send_header("Content-Type", "application/octet-stream")
        self.send_header("Content-Length", str(length))
        self.send_header("Accept-Ranges", "bytes")
        self.send_header("Last-Modified", email.utils.formatdate(stat.st_mtime, usegmt=True))
        if with_etag:
            self.send_header("ETag", '"%x-%x"' % (int(stat.st_mtime), stat.st_size))
        if status == 206:
            self.send_header("Content-Range", "bytes %d-%d/%d" % (start, stat.st_size - 1, stat.st_size))
        self.end_headers()

        if not with_body:
            record("%s %s %d" % (self.command, self.path, status))
            return

        drop = consume_drop_marker(name)
        limit = int(length * DROP_RATIO) if drop else length
        sent = 0
        with open(name, "rb") as f:
            f.seek(start)
            while sent < limit:
                chunk = f.read(min(CHUNK, limit - sent))
                if not chunk:
                    break
                self.wfile.write(chunk)
                sent += len(chunk)

        if drop:
            # The body is short of the Content-Length that was promised and the
            # connection goes away with it, so the client is left with a partial
            # file and an error.
            self.close_connection = True
            record("%s %s %d dropped" % (self.command, self.path, status))
            return
        record("%s %s %d" % (self.command, self.path, status))

    def respond(self, status):
        self.send_response(status)
        self.send_header("Content-Length", "0")
        self.end_headers()
        record("%s %s %d" % (self.command, self.path, status))


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


if __name__ == "__main__":
    os.makedirs(ROOT, exist_ok=True)
    with Server(ADDRESS, Handler) as httpd:
        httpd.serve_forever()
