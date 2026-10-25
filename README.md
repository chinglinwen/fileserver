# fileserver

A minimal read-only download fileserver with mandatory bearer authentication.

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
  -secret-file string
        Path to bearer secret file (required; must be mode 0600).
  -v    Show version.
```

# Example usage

```
./fileserver -path /srv/artifacts -addr 127.0.0.1 -port 9000 -secret-file /etc/fileserver/secret -max-bytes 67108864 -rate 20 &
```

The server refuses to start if -secret-file is missing, empty, or not a regular file with mode 0600.

The secret file must be mode 0600 and is never committed.

Build:

```
go build -o fileserver .
```

# Download

See the release page.
