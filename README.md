# fileserver

A minimal download fileserver

# Usage

```
Usage of ./fileserver:
  -author
        Show author.
  -logfile string
        log filename and path (default "fs.log")
  -logmaxage int
        log max age (days) (default 28)
  -logmaxbackups int
        log max backups number (default 3)
  -logmaxsize int
        log max size(megabytes) (default 500)
  -path string
        File server path. (default ".")
  -port string
        Port number. (default "9000")
  -v    Show version.
```

# Example usage

```
./fileserver -path public -port 9001 &
```

# Download

See the release page.

