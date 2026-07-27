package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var githubAPIBase = "https://api.github.com"

var httpClient = &http.Client{Timeout: 30 * time.Second}

var (
	osExecutable  = os.Executable
	osMkdirTemp   = os.MkdirTemp
	osUserHomeDir = os.UserHomeDir
	ioCopy        = io.Copy
	osOpenFile    = os.OpenFile
	osCreateTemp  = os.CreateTemp
)

const githubRepo = "davidbudnick/postgres-tui"

const maxDownloadSize = 256 << 20
const maxBinarySize = 128 << 20

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func runUpdate(currentVersion string) error {
	if currentVersion == "dev" || !isSemver(currentVersion) {
		return fmt.Errorf("cannot self-update a development build (version=%q); use the install script instead:\n  curl -fsSL https://raw.githubusercontent.com/davidbudnick/postgres-tui/main/install.sh | bash", currentVersion)
	}

	execPath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}

	if isHomebrew(execPath) {
		return fmt.Errorf("this binary was installed via Homebrew; please update with:\n  brew upgrade postgres-tui")
	}

	if err := checkWriteAccess(execPath); err != nil {
		home, homeErr := osUserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("cannot write to %s and could not determine home directory: %w", execPath, homeErr)
		}
		localBin := filepath.Join(home, ".local", "bin")
		if mkErr := os.MkdirAll(localBin, 0750); mkErr != nil {
			return fmt.Errorf("cannot write to %s and could not create %s: %w", execPath, localBin, mkErr)
		}
		execPath = filepath.Join(localBin, "postgres-tui")
		fmt.Printf("No write access to current location, installing to %s\n", execPath)
	}

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to fetch latest version: %w", err)
	}

	if strings.TrimPrefix(latest, "v") == strings.TrimPrefix(currentVersion, "v") {
		fmt.Printf("Already up to date (v%s).\n", strings.TrimPrefix(currentVersion, "v"))
		return nil
	}

	ver := strings.TrimPrefix(latest, "v")
	archive := archiveName(ver, runtime.GOOS, runtime.GOARCH)
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", githubRepo, latest)
	archiveURL := baseURL + "/" + archive
	checksumURL := baseURL + "/checksums.txt"

	tmpDir, err := osMkdirTemp("", "postgres-tui-update-*")
	if err != nil {
		return fmt.Errorf("could not create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archive)
	checksumPath := filepath.Join(tmpDir, "checksums.txt")

	if err := downloadFile(archiveURL, archivePath); err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	if err := downloadFile(checksumURL, checksumPath); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(archivePath, checksumPath, archive); err != nil {
		return err
	}

	binPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return err
	}
	if err := installBinary(binPath, execPath); err != nil {
		return err
	}
	fmt.Printf("Updated to %s → %s\n", latest, execPath)
	return nil
}

func isSemver(v string) bool {
	return regexp.MustCompile(`^v?\d+\.\d+\.\d+`).MatchString(v)
}

func isHomebrew(path string) bool {
	return strings.Contains(path, "Homebrew") || strings.Contains(path, "/Cellar/")
}

func checkWriteAccess(path string) error {
	f, err := osOpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

func fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, githubRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("empty tag")
	}
	return rel.TagName, nil
}

func archiveName(ver, goos, goarch string) string {
	return fmt.Sprintf("postgres-tui_%s_%s_%s.tar.gz", ver, goos, goarch)
}

func downloadFile(url, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = ioCopy(f, io.LimitReader(resp.Body, maxDownloadSize))
	return err
}

func verifyChecksum(archivePath, checksumPath, archiveName string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasSuffix(fields[len(fields)-1], archiveName) {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksum not found for %s", archiveName)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch: got %s want %s", got, want)
	}
	return nil
}

func extractBinary(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		base := filepath.Base(hdr.Name)
		if base != "postgres-tui" && base != "postgres-tui.exe" {
			continue
		}
		out := filepath.Join(destDir, base)
		of, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(of, io.LimitReader(tr, maxBinarySize)); err != nil {
			of.Close()
			return "", err
		}
		of.Close()
		return out, nil
	}
	return "", fmt.Errorf("binary not found in archive")
}

func installBinary(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := osCreateTemp(filepath.Dir(dest), ".postgres-tui-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}
