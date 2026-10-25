package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeSecretFile(t *testing.T, dir string) (string, string) {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	sec := hex.EncodeToString(secret)
	p := filepath.Join(dir, "secret")
	if err := os.WriteFile(p, []byte(sec+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return p, sec
}

func newTestServer(t *testing.T, root, secret string, maxBytes int64, rate float64) *httptest.Server {
	t.Helper()
	return newTestServerAuth(t, root, secret, secret != "", maxBytes, rate)
}

func newTestServerAuth(t *testing.T, root, secret string, requireAuth bool, maxBytes int64, rate float64) *httptest.Server {
	t.Helper()
	s := newFileServer(root, secret, requireAuth, maxBytes, rate)
	h := logHandler(s.ServeHTTP)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestNoAuthByDefault(t *testing.T) {
	dir := t.TempDir()
	content := []byte("public by exact url\n")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}
	ts := newTestServerAuth(t, dir, "", false, 1<<20, 0)

	req, _ := http.NewRequest("GET", ts.URL+"/file.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, content) {
		t.Fatalf("body mismatch")
	}

	reqRoot, _ := http.NewRequest("GET", ts.URL+"/", nil)
	respRoot, err := http.DefaultClient.Do(reqRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer respRoot.Body.Close()
	if respRoot.StatusCode != 404 {
		t.Fatalf("root want 404, got %d", respRoot.StatusCode)
	}

	reqTrav, _ := http.NewRequest("GET", ts.URL+"/../file.txt", nil)
	respTrav, err := http.DefaultClient.Do(reqTrav)
	if err != nil {
		t.Fatal(err)
	}
	defer respTrav.Body.Close()
	if respTrav.StatusCode != 404 {
		t.Fatalf("traversal want 404, got %d", respTrav.StatusCode)
	}
}

func TestAuthMissing(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	req, _ := http.NewRequest("GET", ts.URL+"/file.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	_ = secPath
}

func TestAuthWrong(t *testing.T) {
	dir := t.TempDir()
	_, sec := writeSecretFile(t, dir)
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	req, _ := http.NewRequest("GET", ts.URL+"/file.txt", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthOKGetAndHead(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	content := []byte("hello world from test\n")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	// GET
	req, _ := http.NewRequest("GET", ts.URL+"/file.txt", nil)
	req.Header.Set("Authorization", "Bearer "+sec)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, content) {
		t.Fatalf("body mismatch: %q != %q", got, content)
	}

	// HEAD
	req2, _ := http.NewRequest("HEAD", ts.URL+"/file.txt", nil)
	req2.Header.Set("Authorization", "Bearer "+sec)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("head want 200, got %d", resp2.StatusCode)
	}
	if resp2.ContentLength != int64(len(content)) {
		t.Fatalf("head content-length %d want %d", resp2.ContentLength, len(content))
	}
	body2, _ := io.ReadAll(resp2.Body)
	if len(body2) != 0 {
		t.Fatalf("head must have no body, got %d bytes", len(body2))
	}
	_ = secPath
}

func TestTraversalRejects(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	out := []byte("secret")
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), out, 0644); err != nil {
		t.Fatal(err)
	}
	// create a sibling outside
	parent := filepath.Dir(dir)
	out2 := []byte("outside")
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), out2, 0644); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	cases := []string{
		"/../secret.txt",
		"/%2e%2e/secret.txt",
		"/..%2fsecret.txt",
	}
	for _, c := range cases {
		req, _ := http.NewRequest("GET", ts.URL+c, nil)
		req.Header.Set("Authorization", "Bearer "+sec)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Fatalf("%s: want 404, got %d", c, resp.StatusCode)
		}
	}
	_ = secPath
}

func TestDirectoryIs404(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	req, _ := http.NewRequest("GET", ts.URL+"/subdir", nil)
	req.Header.Set("Authorization", "Bearer "+sec)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("dir want 404, got %d", resp.StatusCode)
	}
	_ = secPath
}

func TestMaxBytes413(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	big := make([]byte, 100)
	if err := os.WriteFile(filepath.Join(dir, "big.bin"), big, 0644); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 50, 0) // max 50 bytes

	req, _ := http.NewRequest("GET", ts.URL+"/big.bin", nil)
	req.Header.Set("Authorization", "Bearer "+sec)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Fatalf("want 413, got %d", resp.StatusCode)
	}
	_ = secPath
}

func TestFileOutsideRootNotServed(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	// place a file in a sibling dir
	sib := filepath.Join(dir, "sib")
	if err := os.Mkdir(sib, 0755); err != nil {
		t.Fatal(err)
	}
	out := []byte("sibdata")
	if err := os.WriteFile(filepath.Join(sib, "sib.txt"), out, 0644); err != nil {
		t.Fatal(err)
	}
	// symlink escape: root points to dir, but we request via a symlink pointing outside
	// create a symlink inside root pointing outside (to parent of root)
	parent := filepath.Dir(dir)
	outside := filepath.Join(parent, "outside-"+filepath.Base(dir)+".txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)
	link := filepath.Join(dir, "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	req, _ := http.NewRequest("GET", ts.URL+"/escape.txt", nil)
	req.Header.Set("Authorization", "Bearer "+sec)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("symlink escape want 404, got %d", resp.StatusCode)
	}
	_ = secPath
}

func TestOtherMethod405(t *testing.T) {
	dir := t.TempDir()
	secPath, sec := writeSecretFile(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	ts := newTestServer(t, dir, sec, 1<<20, 0)

	req, _ := http.NewRequest("POST", ts.URL+"/x.txt", nil)
	req.Header.Set("Authorization", "Bearer "+sec)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("want 405, got %d", resp.StatusCode)
	}
	_ = secPath
}
