package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonOK(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestIsHomebrewAndWriteAccess(t *testing.T) {
	if !isHomebrew("/opt/Homebrew/bin/postgres-tui") {
		t.Fatal("Homebrew")
	}
	if !isHomebrew("/usr/local/Cellar/postgres-tui/1.0/bin/postgres-tui") {
		t.Fatal("Cellar")
	}
	if isHomebrew("/usr/local/bin/postgres-tui") {
		t.Fatal("plain path")
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "bin")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkWriteAccess(p); err != nil {
		t.Fatal(err)
	}
	if err := checkWriteAccess(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing write error")
	}
}

func TestFetchLatestVersion(t *testing.T) {
	old := httpClient
	t.Cleanup(func() { httpClient = old })

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`{"tag_name":"v1.2.3"}`), nil
	})}
	ver, err := fetchLatestVersion()
	if err != nil || ver != "v1.2.3" {
		t.Fatal(ver, err)
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	if _, err := fetchLatestVersion(); err == nil {
		t.Fatal("404")
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`notjson`), nil
	})}
	if _, err := fetchLatestVersion(); err == nil {
		t.Fatal("json")
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`{"tag_name":""}`), nil
	})}
	if _, err := fetchLatestVersion(); err == nil {
		t.Fatal("empty tag")
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("net")
	})}
	if _, err := fetchLatestVersion(); err == nil {
		t.Fatal("net")
	}

	oldBase := githubAPIBase
	githubAPIBase = "://bad"
	t.Cleanup(func() { githubAPIBase = oldBase })
	httpClient = old
	if _, err := fetchLatestVersion(); err == nil {
		t.Fatal("bad url")
	}
	githubAPIBase = oldBase
}

func TestArchiveNameWindows(t *testing.T) {
	got := archiveName("1.0.0", "windows", "amd64")
	if !strings.Contains(got, "Windows") || !strings.Contains(got, "x86_64") {
		t.Fatal(got)
	}
	// unknown os/arch passthrough
	got = archiveName("1.0.0", "freebsd", "386")
	if !strings.Contains(got, "freebsd") || !strings.Contains(got, "386") {
		t.Fatal(got)
	}
}

func TestDownloadFile(t *testing.T) {
	old := httpClient
	t.Cleanup(func() { httpClient = old })

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data")), Header: make(http.Header)}, nil
	})}
	dest := filepath.Join(t.TempDir(), "f")
	if err := downloadFile("http://x/y", dest); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "data" {
		t.Fatal(string(b))
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	if err := downloadFile("http://x", dest); err == nil {
		t.Fatal("500")
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("net")
	})}
	if err := downloadFile("http://x", dest); err == nil {
		t.Fatal("net")
	}

	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("x")), Header: make(http.Header)}, nil
	})}
	if err := downloadFile("http://x", filepath.Join(t.TempDir(), "no", "such")); err == nil {
		t.Fatal("create")
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	content := []byte("archive-bytes")
	arch := filepath.Join(dir, "postgres-tui_1.0.0_Darwin_arm64.tar.gz")
	if err := os.WriteFile(arch, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	cs := filepath.Join(dir, "checksums.txt")
	name := filepath.Base(arch)
	if err := os.WriteFile(cs, []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(arch, cs, name); err != nil {
		t.Fatal(err)
	}
	// suffix match with path prefix
	if err := os.WriteFile(cs, []byte(fmt.Sprintf("%s  dist/%s\n", hex.EncodeToString(sum[:]), name)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(arch, cs, name); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(arch, cs, "missing.tar.gz"); err == nil {
		t.Fatal("not found")
	}
	if err := verifyChecksum(arch, filepath.Join(dir, "nope"), name); err == nil {
		t.Fatal("cs missing")
	}
	if err := os.WriteFile(cs, []byte(fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), name)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(arch, cs, name); err == nil {
		t.Fatal("mismatch")
	}
	if err := verifyChecksum(filepath.Join(dir, "missing"), cs, name); err == nil {
		t.Fatal("arch missing")
	}
}

func buildPGTarGz(t *testing.T, name, payload string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "dir/" + name, Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractAndInstallBinary(t *testing.T) {
	dir := t.TempDir()
	data := buildPGTarGz(t, "postgres-tui", "#!/bin/sh\n")
	arch := filepath.Join(dir, "a.tar.gz")
	if err := os.WriteFile(arch, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bin, err := extractBinary(arch, dir)
	if err != nil || !strings.HasSuffix(bin, "postgres-tui") {
		t.Fatal(bin, err)
	}

	// exe name
	data = buildPGTarGz(t, "postgres-tui.exe", "MZ")
	arch2 := filepath.Join(dir, "b.tar.gz")
	if err := os.WriteFile(arch2, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bin, err = extractBinary(arch2, dir)
	if err != nil || !strings.HasSuffix(bin, "postgres-tui.exe") {
		t.Fatal(bin, err)
	}

	if _, err := extractBinary(filepath.Join(dir, "nope"), dir); err == nil {
		t.Fatal("missing")
	}
	plain := filepath.Join(dir, "plain")
	_ = os.WriteFile(plain, []byte("notgzip"), 0o600)
	if _, err := extractBinary(plain, dir); err == nil {
		t.Fatal("gzip")
	}

	// no binary in archive
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "readme", Mode: 0644, Size: 1})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	emptyArch := filepath.Join(dir, "empty.tar.gz")
	_ = os.WriteFile(emptyArch, buf.Bytes(), 0o600)
	if _, err := extractBinary(emptyArch, dir); err == nil {
		t.Fatal("not found")
	}

	// install
	src := filepath.Join(dir, "srcbin")
	_ = os.WriteFile(src, []byte("newbin"), 0o644)
	dest := filepath.Join(dir, "destbin")
	if err := installBinary(src, dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "newbin" {
		t.Fatal(string(got))
	}

	if err := installBinary(filepath.Join(dir, "missing"), dest); err == nil {
		t.Fatal("open src")
	}

	oldCT := osCreateTemp
	t.Cleanup(func() { osCreateTemp = oldCT })
	osCreateTemp = func(string, string) (*os.File, error) { return nil, fmt.Errorf("tmp") }
	if err := installBinary(src, dest); err == nil {
		t.Fatal("tmp")
	}
	osCreateTemp = oldCT

	oldCopy := ioCopy
	t.Cleanup(func() { ioCopy = oldCopy })
	ioCopy = func(dst io.Writer, src io.Reader) (int64, error) { return 0, fmt.Errorf("copy") }
	if err := installBinary(src, dest); err == nil {
		t.Fatal("copy")
	}
	ioCopy = oldCopy

	oldChmod := fileChmod
	t.Cleanup(func() { fileChmod = oldChmod })
	fileChmod = func(*os.File, os.FileMode) error { return fmt.Errorf("chmod") }
	if err := installBinary(src, dest); err == nil {
		t.Fatal("chmod")
	}
	fileChmod = oldChmod

	oldClose := fileClose
	t.Cleanup(func() { fileClose = oldClose })
	fileClose = func(*os.File) error { return fmt.Errorf("close") }
	if err := installBinary(src, dest); err == nil {
		t.Fatal("close")
	}
	fileClose = oldClose

	// extractBinary copy fail
	data2 := buildPGTarGz(t, "postgres-tui", "bin")
	arch3 := filepath.Join(dir, "c.tar.gz")
	_ = os.WriteFile(arch3, data2, 0o600)
	ioCopy = func(dst io.Writer, src io.Reader) (int64, error) { return 0, fmt.Errorf("copy") }
	if _, err := extractBinary(arch3, dir); err == nil {
		t.Fatal("extract copy")
	}
	ioCopy = oldCopy

	// extractBinary openfile fail
	oldOpen := osOpenFile
	t.Cleanup(func() { osOpenFile = oldOpen })
	osOpenFile = func(string, int, os.FileMode) (*os.File, error) { return nil, fmt.Errorf("open") }
	if _, err := extractBinary(arch3, dir); err == nil {
		t.Fatal("extract open")
	}
	osOpenFile = oldOpen

	// corrupt tar inside gzip
	var cbuf bytes.Buffer
	cgz := gzip.NewWriter(&cbuf)
	_, _ = cgz.Write([]byte("not-a-tar"))
	_ = cgz.Close()
	corrupt := filepath.Join(dir, "corrupt.tar.gz")
	_ = os.WriteFile(corrupt, cbuf.Bytes(), 0o600)
	if _, err := extractBinary(corrupt, dir); err == nil {
		t.Fatal("tar")
	}

	// verifyChecksum ioCopy fail
	content := []byte("archive-bytes")
	archCS := filepath.Join(dir, "cs.tar.gz")
	_ = os.WriteFile(archCS, content, 0o600)
	sum := sha256.Sum256(content)
	csPath := filepath.Join(dir, "cs.txt")
	_ = os.WriteFile(csPath, []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), "cs.tar.gz")), 0o600)
	ioCopy = func(dst io.Writer, src io.Reader) (int64, error) { return 0, fmt.Errorf("hash") }
	if err := verifyChecksum(archCS, csPath, "cs.tar.gz"); err == nil {
		t.Fatal("hash")
	}
	ioCopy = oldCopy
}

func TestRunUpdate(t *testing.T) {
	if err := runUpdate("dev"); err == nil {
		t.Fatal("dev")
	}
	if err := runUpdate("not-semver"); err == nil {
		t.Fatal("semver")
	}

	dir := t.TempDir()
	oldExec := osExecutable
	oldClient := httpClient
	oldMk := osMkdirTemp
	oldHome := osUserHomeDir
	t.Cleanup(func() {
		osExecutable = oldExec
		httpClient = oldClient
		osMkdirTemp = oldMk
		osUserHomeDir = oldHome
	})

	// executable error
	osExecutable = func() (string, error) { return "", fmt.Errorf("exec") }
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("exec")
	}

	// eval symlinks fail
	osExecutable = func() (string, error) { return filepath.Join(dir, "missing-exec"), nil }
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("eval")
	}

	// homebrew path
	hb := filepath.Join(dir, "Homebrew", "bin")
	_ = os.MkdirAll(hb, 0o755)
	hbBin := filepath.Join(hb, "postgres-tui")
	_ = os.WriteFile(hbBin, []byte("x"), 0o755)
	osExecutable = func() (string, error) { return hbBin, nil }
	if err := runUpdate("1.0.0"); err == nil || !strings.Contains(err.Error(), "Homebrew") {
		t.Fatal(err)
	}

	// writable path, already up to date
	bin := filepath.Join(dir, "postgres-tui")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	osExecutable = func() (string, error) { return bin, nil }
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`{"tag_name":"v1.0.0"}`), nil
	})}
	if err := runUpdate("1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := runUpdate("v1.0.0"); err != nil {
		t.Fatal(err)
	}

	// fetch fail
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("net")
	})}
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("fetch")
	}

	// mkdirtemp fail after fetch
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`{"tag_name":"v9.9.9"}`), nil
	})}
	osMkdirTemp = func(string, string) (string, error) { return "", fmt.Errorf("mk") }
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("mk")
	}
	osMkdirTemp = oldMk

	// download archive fail
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		return nil, fmt.Errorf("dl")
	})}
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("dl")
	}

	// download checksums fail
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		path := r.URL.Path
		if strings.Contains(path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		if strings.Contains(path, "checksums") {
			return nil, fmt.Errorf("cs")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("x")), Header: make(http.Header)}, nil
	})}
	if err := runUpdate("1.0.0"); err == nil || !strings.Contains(err.Error(), "checksums") {
		t.Fatal(err)
	}

	// checksum mismatch
	badTar := buildPGTarGz(t, "postgres-tui", "x")
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		path := r.URL.Path
		if strings.Contains(path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		if strings.Contains(path, "checksums") {
			body := fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), archiveName("9.9.9", runtime.GOOS, runtime.GOARCH))
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(badTar)), Header: make(http.Header)}, nil
	})}
	if err := runUpdate("1.0.0"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}

	// extract fail (archive has no binary)
	var emptyBuf bytes.Buffer
	egz := gzip.NewWriter(&emptyBuf)
	etw := tar.NewWriter(egz)
	_ = etw.WriteHeader(&tar.Header{Name: "readme", Mode: 0644, Size: 1})
	_, _ = etw.Write([]byte("x"))
	_ = etw.Close()
	_ = egz.Close()
	emptyTar := emptyBuf.Bytes()
	emptySum := sha256.Sum256(emptyTar)
	emptyArchName := archiveName("9.9.9", runtime.GOOS, runtime.GOARCH)
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		path := r.URL.Path
		if strings.Contains(path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		if strings.Contains(path, "checksums") {
			body := fmt.Sprintf("%s  %s\n", hex.EncodeToString(emptySum[:]), emptyArchName)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(emptyTar)), Header: make(http.Header)}, nil
	})}
	if err := runUpdate("1.0.0"); err == nil || !strings.Contains(err.Error(), "binary not found") {
		t.Fatal(err)
	}

	// install fail
	okTar := buildPGTarGz(t, "postgres-tui", "bin")
	okSum := sha256.Sum256(okTar)
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		path := r.URL.Path
		if strings.Contains(path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		if strings.Contains(path, "checksums") {
			body := fmt.Sprintf("%s  %s\n", hex.EncodeToString(okSum[:]), emptyArchName)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(okTar)), Header: make(http.Header)}, nil
	})}
	oldCT := osCreateTemp
	osCreateTemp = func(string, string) (*os.File, error) { return nil, fmt.Errorf("installtmp") }
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("install")
	}
	osCreateTemp = oldCT

	// full success path
	ioCopy = io.Copy
	payload := "binary-content"
	tarBytes := buildPGTarGz(t, "postgres-tui", payload)
	sum := sha256.Sum256(tarBytes)
	archName := archiveName("9.9.9", runtime.GOOS, runtime.GOARCH)
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		path := r.URL.Path
		if strings.Contains(path, "latest") {
			return jsonOK(`{"tag_name":"v9.9.9"}`), nil
		}
		if strings.Contains(path, "checksums") {
			body := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archName)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		if strings.Contains(path, archName) || strings.HasSuffix(path, ".tar.gz") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(tarBytes)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	if err := runUpdate("1.0.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(bin)
	if err != nil || string(got) != payload {
		t.Fatalf("installed %q err=%v", got, err)
	}

	// no write access -> install to ~/.local/bin
	roDir := filepath.Join(dir, "ro")
	_ = os.MkdirAll(roDir, 0o555)
	roBin := filepath.Join(roDir, "postgres-tui")
	// can't create file in 0555 dir on some systems — use open-only fail via path that exists but is not writable
	// create file then chmod directory read-only after
	_ = os.Chmod(roDir, 0o755)
	_ = os.WriteFile(roBin, []byte("old"), 0o400)
	_ = os.Chmod(roBin, 0o400)
	osExecutable = func() (string, error) { return roBin, nil }
	home := filepath.Join(dir, "home")
	_ = os.MkdirAll(home, 0o755)
	osUserHomeDir = func() (string, error) { return home, nil }
	// reopen: checkWriteAccess opens O_WRONLY — 0400 should fail for non-owner? we're owner so 0400 still allows owner write on unix... use 0444
	_ = os.Chmod(roBin, 0o444)
	// On some FS owner can still write; force osOpenFile failure
	oldOpen := osOpenFile
	osOpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		if name == roBin {
			return nil, fmt.Errorf("permission denied")
		}
		return oldOpen(name, flag, perm)
	}
	t.Cleanup(func() { osOpenFile = oldOpen })
	if err := runUpdate("1.0.0"); err != nil {
		t.Fatal(err)
	}
	localBin := filepath.Join(home, ".local", "bin", "postgres-tui")
	if _, err := os.Stat(localBin); err != nil {
		t.Fatal(err)
	}

	// write access fail + home fail
	osUserHomeDir = func() (string, error) { return "", fmt.Errorf("nohome") }
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("home")
	}

	// write access fail + mkdir local bin fail
	osUserHomeDir = func() (string, error) { return filepath.Join(dir, "filehome"), nil }
	_ = os.WriteFile(filepath.Join(dir, "filehome"), []byte("x"), 0o600)
	if err := runUpdate("1.0.0"); err == nil {
		t.Fatal("mkdir local")
	}
	osOpenFile = oldOpen
	osExecutable = oldExec
}
