# fileserver

A minimal read-only download fileserver. Downloads need no token unless `-require-auth` is set. Directory listing is disabled.

# Usage

```
Usage of ./fileserver:
  -addr string
        Listen address host. (default "127.0.0.1")
  -author
        Show author.
  -logfile string
        log filename and path (default "fs.log")
  -logmaxage int
        log max age (days) (default 28)
  -logmaxbackups int
        log max backups number (default 3)
  -logmaxsize int
        log max size (megabytes) (default 500)
  -max-bytes int
        Maximum file size to serve in bytes. (default 67108864)
  -path string
        File server path. (default ".")
  -port string
        Port number. (default "9000")
  -rate float
        Rate limit requests/sec per client IP (0 disables); burst 40. (default 20)
  -require-auth
        Require Authorization: Bearer <secret>. Off by default.
  -secret-file string
        Path to bearer secret file (required only with -require-auth; must be mode 0600).
  -v    Show version.
```

# Example usage

```
./fileserver -path /srv/artifacts -addr 127.0.0.1 -port 9000 -max-bytes 67108864 -rate 20 &
```

Access is by an unguessable URL. A request for `/` or a directory returns 404.

Optional bearer auth:

```
./fileserver -path /srv/artifacts -addr 127.0.0.1 -port 9000 -require-auth -secret-file /etc/fileserver/secret -max-bytes 67108864 -rate 20 &
```

With `-require-auth`, the server refuses to start if `-secret-file` is missing, empty, or not a regular file with mode 0600. The secret file is never committed.

Build:

```
go build -o fileserver .
```

# Download

See the release page.
