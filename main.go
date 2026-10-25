/*
fileserver is a read-only file server.

It serves files under a given path over HTTP on loopback by default.
Downloads require no token unless -require-auth is set. With that flag,
clients must send Authorization: Bearer <secret>, and the secret is read
from a mode-0600 file at startup.

See README.md and docs/hosting.md.
*/
package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/natefinch/lumberjack"
)

func main() {
	port := flag.String("port", "9000", "Port number.")
	path := flag.String("path", ".", "File server path.")
	addr := flag.String("addr", "127.0.0.1", "Listen address host.")
	secretFile := flag.String("secret-file", "", "Path to bearer secret file (required only with -require-auth; must be mode 0600).")
	requireAuth := flag.Bool("require-auth", false, "Require Authorization: Bearer <secret>. Off by default.")
	maxBytes := flag.Int64("max-bytes", 67108864, "Maximum file size to serve in bytes.")
	rate := flag.Float64("rate", 20, "Rate limit requests/sec per client IP (0 disables); burst 40.")
	version := flag.Bool("v", false, "Show version.")
	author := flag.Bool("author", false, "Show author.")

	logFile := flag.String("logfile", "fs.log", "log filename and path")
	logMaxSize := flag.Int("logmaxsize", 500, "log max size (megabytes)")
	logMaxAge := flag.Int("logmaxage", 28, "log max age (days)")
	logMaxBackups := flag.Int("logmaxbackups", 3, "log max backups number")

	flag.Parse()

	if *version {
		fmt.Println("fileserver version=3.0, 2025-10")
		os.Exit(0)
	}
	if *author {
		fmt.Println("Author is Wen Zhenglin")
		os.Exit(0)
	}

	sec := ""
	if *requireAuth {
		if *secretFile == "" {
			fmt.Fprintln(os.Stderr, "-secret-file is required when -require-auth is set")
			os.Exit(1)
		}

		// Read secret once; trim a single trailing newline.
		data, err := os.ReadFile(*secretFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read secret-file: %v\n", err)
			os.Exit(1)
		}
		sec = string(data)
		if len(sec) > 0 && sec[len(sec)-1] == '\n' {
			sec = sec[:len(sec)-1]
		}
		if sec == "" {
			fmt.Fprintln(os.Stderr, "secret-file is empty after trimming trailing newline")
			os.Exit(1)
		}

		// Enforce mode 0600 and regular file (no symlinks).
		info, err := os.Lstat(*secretFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to stat secret-file: %v\n", err)
			os.Exit(1)
		}
		if (info.Mode()&os.ModeSymlink) != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			fmt.Fprintln(os.Stderr, "secret-file must be a regular file with mode 0600")
			os.Exit(1)
		}
	}

	// Setup logging (lumberjack) similar to previous behavior.
	log.SetOutput(&lumberjack.Logger{
		Filename:   *logFile,
		MaxSize:    *logMaxSize,
		MaxBackups: *logMaxBackups,
		MaxAge:     *logMaxAge,
	})

	srv := newFileServer(*path, sec, *requireAuth, *maxBytes, *rate)

	addrPort := net.JoinHostPort(*addr, *port)
	log.Printf("Starting service at %s", addrPort)

	h := logHandler(srv.ServeHTTP)
	log.Fatal(http.ListenAndServe(addrPort, h))
}

// fileServer implements the secured file serving logic.
type fileServer struct {
	root     string
	rootReal string
	secret      string
	requireAuth bool
	maxBytes    int64
	rate        float64
	burst       int

	limiters map[string]*ipLimiter
	limMu    sync.Mutex
}

func newFileServer(rootDir, secret string, requireAuth bool, maxBytes int64, rate float64) *fileServer {
	s := &fileServer{
		secret:      secret,
		requireAuth: requireAuth,
		maxBytes:    maxBytes,
		rate:        rate,
		burst:       40,
		limiters:    make(map[string]*ipLimiter),
	}
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		abs = rootDir
	}
	s.root = abs
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		s.rootReal = real
	} else {
		s.rootReal = abs
	}
	return s
}

func (s *fileServer) getLimiter(ip string) *ipLimiter {
	s.limMu.Lock()
	defer s.limMu.Unlock()
	l, ok := s.limiters[ip]
	if !ok {
		l = newIPLimiter(s.rate, s.burst)
		s.limiters[ip] = l
	}
	return l
}

func (s *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.rate > 0 {
		lim := s.getLimiter(ip)
		if !lim.allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
	}

	if s.requireAuth {
		authz := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authz, bearerPrefix) {
			writeUnauthorized(w)
			return
		}
		tok := strings.TrimPrefix(authz, bearerPrefix)
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.secret)) != 1 {
			writeUnauthorized(w)
			return
		}
	}

	s.serveContent(w, r)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	io.WriteString(w, "unauthorized")
}

func (s *fileServer) serveContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	orig := r.URL.Path
	if strings.Contains(orig, "..") {
		http.NotFound(w, r)
		return
	}
	clean := path.Clean(orig)
	if clean == "" || clean == "/" || clean == "." {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}

	fsPath := filepath.Join(s.root, filepath.FromSlash(rel))

	fi, err := os.Stat(fsPath)
	if err != nil || !fi.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	if fi.Size() > s.maxBytes {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	resolved, err := filepath.EvalSymlinks(fsPath)
	if err != nil || !isUnder(resolved, s.rootReal) {
		http.NotFound(w, r)
		return
	}

	ct := mime.TypeByExtension(filepath.Ext(fsPath))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	f, err := os.Open(fsPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

func isUnder(candidate, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	return ip
}

// ipLimiter is a minimal token-bucket limiter (no external rate dep).
type ipLimiter struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
	rate   float64
	burst  float64
}

func newIPLimiter(rps float64, burst int) *ipLimiter {
	b := float64(burst)
	return &ipLimiter{
		tokens: b,
		last:   time.Now(),
		rate:   rps,
		burst:  b,
	}
}

func (l *ipLimiter) allow() bool {
	if l.rate <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.last = now
	if l.tokens >= 1 {
		l.tokens -= 1
		return true
	}
	return false
}

// loggingResponseWriter records status and bytes written for the access log.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	if lrw.status == 0 {
		lrw.status = code
	}
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.status == 0 {
		lrw.status = 200
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytes += int64(n)
	return n, err
}

func logHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w}
		fn(lrw, r)
		ip := clientIP(r)
		log.Printf("%s %s %s %s %d %d", time.Now().Format(time.RFC3339), ip, r.Method, r.URL.Path, lrw.status, lrw.bytes)
	}
}
