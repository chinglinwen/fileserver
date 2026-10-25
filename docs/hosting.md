# Hosting

This server is a read-only Go fileserver. Access is by an unguessable URL. Directory listing is disabled.

## Running the server

The server binds only to the loopback address by default (`-addr 127.0.0.1`). It listens on `-addr:-port` and serves files under `-path`.

Example:

```
./fileserver \
  -path /srv/artifacts \
  -addr 127.0.0.1 \
  -port 9000 \
  -max-bytes 67108864 \
  -rate 20
```

Bearer authentication is implemented but off by default. A GET of an existing regular file returns 200 with no token. A request for `/` or any directory returns 404. Path traversal is rejected. Only regular files under the size cap are served, and requests are rate-limited per client IP.

To require a bearer token, pass `-require-auth` and `-secret-file`. With `-require-auth`, the process exits non-zero if `-secret-file` is missing, empty after trimming one trailing newline, or not a regular file with mode 0600.

## URL scheme

Public access uses the scheme:

```
https://fs.clwen.com/<exact-relative-path>
```

The exact relative path is the path under the served root. Anyone who has that exact URL can download the file. No directory listings are provided; missing or directory paths return 404.

## Client fetch

No Authorization header is required:

```
curl -D - -o /dev/null \
  https://fs.clwen.com/path/to/file
```

## Optional bearer auth

1. Write a secret file with mode 0600.
2. Start the server with `-require-auth -secret-file /path/to/secret`.
3. Restart the server after rewriting the secret. The server reads the secret once at startup.

Clients then send:

```
Authorization: Bearer <secret>
```

## Rollback steps

To stop serving via the current deployment:

- Stop the local service:

```
launchctl bootout gui/$(id -u)/com.clwen.fileserver-artifacts
```

- Remove the Cloudflare tunnel ingress for fs.clwen.com on tunnel m4.
- Delete the DNS CNAME for fs.clwen.com.

After rollback, clients will receive connection or DNS errors for that hostname.
