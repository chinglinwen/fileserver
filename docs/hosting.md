# Hosting

This server is a read-only Go fileserver intended to serve artifacts behind a mandatory shared secret.

## Running the server

The server binds only to the loopback address by default (`-addr 127.0.0.1`). It listens on `-addr:-port` and serves files under `-path`.

Example:

```
./fileserver \
  -path /srv/artifacts \
  -addr 127.0.0.1 \
  -port 9000 \
  -secret-file /etc/fileserver/secret \
  -max-bytes 67108864 \
  -rate 20
```

Auth is mandatory. The process exits non-zero if `-secret-file` is missing, empty after trimming one trailing newline, or not a regular file with mode 0600.

## URL scheme

Public access uses the scheme:

```
https://artifacts-zflow.clwen.com/<exact-relative-path>
```

The exact relative path is the path under the served root. No directory listings are provided; missing or directory paths return 404.

## Client fetch

Clients must send an Authorization header with a bearer token:

```
Authorization: Bearer <secret>
```

The secret is read from a file the operator holds (mode 0600). Example using curl with a local secret file:

```
curl -H "Authorization: Bearer $(cat /path/to/secret | tr -d '\n')" \
  https://artifacts-zflow.clwen.com/path/to/file
```

## Rotating the secret

1. Rewrite the secret file with a new value, keeping mode 0600.
2. Restart the server process.

The server reads the secret once at startup.

## Rollback steps

To stop serving via the current deployment:

- Stop the local service:

```
launchctl bootout gui/$(id -u)/com.clwen.fileserver-artifacts
```

- Remove the Cloudflare tunnel ingress for artifacts-zflow.clwen.com on tunnel m4.
- Delete the DNS CNAME for artifacts-zflow.clwen.com.

After rollback, clients will receive connection or DNS errors for that hostname.
