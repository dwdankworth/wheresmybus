package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultAPIURL = "https://api.github.com/repos/dwdankworth/wheresmybus/releases/latest"
	binaryName    = "wheresmybus"
)

// CheckResult contains the result of checking for an update.
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	AssetURL        string
	AssetName       string
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Check queries the GitHub Releases API for the latest release and compares
// it to currentVersion. Returns a CheckResult indicating whether an update
// is available and, if so, the download URL for the current platform.
func Check(client *http.Client, currentVersion string) (*CheckResult, error) {
	return CheckFromURL(client, defaultAPIURL, currentVersion)
}

// CheckFromURL is like Check but queries a custom URL. This is useful for testing.
func CheckFromURL(client *http.Client, apiURL, currentVersion string) (*CheckResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	result := &CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  release.TagName,
	}

	if !isNewer(currentVersion, release.TagName) {
		return result, nil
	}

	assetName, assetURL := findAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if assetURL == "" {
		return nil, fmt.Errorf("no release asset for %s/%s in %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	result.UpdateAvailable = true
	result.AssetURL = assetURL
	result.AssetName = assetName

	return result, nil
}

// Apply downloads the release archive from assetURL, extracts the binary,
// and replaces the currently running executable.
func Apply(client *http.Client, assetURL, assetName string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	return applyToPath(client, assetURL, assetName, execPath)
}

// applyToPath downloads the release archive, extracts the binary, and
// replaces the file at execPath. Separated from Apply for testability.
func applyToPath(client *http.Client, assetURL, assetName, execPath string) error {
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(assetURL)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read download: %w", err)
	}

	binName := binaryName
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	var binaryData []byte
	if strings.HasSuffix(assetName, ".zip") {
		binaryData, err = extractFromZip(archiveData, binName)
	} else {
		binaryData, err = extractFromTarGz(archiveData, binName)
	}
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	return replaceBinary(execPath, binaryData)
}

func findAsset(assets []githubAsset, goos, goarch string) (string, string) {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	suffix := fmt.Sprintf("_%s_%s.%s", goos, goarch, ext)
	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			return a.Name, a.BrowserDownloadURL
		}
	}
	return "", ""
}

func isNewer(current, latest string) bool {
	if current == "dev" || current == "" {
		return true
	}

	currentParts, ok := parseSemver(current)
	if !ok {
		return true
	}

	latestParts, ok := parseSemver(latest)
	if !ok {
		return false
	}

	return compareSemver(latestParts, currentParts) > 0
}

type semver [3]int

func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}, false
	}

	var v semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}

func compareSemver(a, b semver) int {
	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

func extractFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read binary from archive: %w", err)
			}
			return content, nil
		}
	}
	return nil, fmt.Errorf("%s not found in archive", binaryName)
}

func extractFromZip(data []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Base(f.Name) == binaryName && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open %s in zip: %w", f.Name, err)
			}
			defer func() { _ = rc.Close() }()

			content, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("read %s from zip: %w", f.Name, err)
			}
			return content, nil
		}
	}
	return nil, fmt.Errorf("%s not found in archive", binaryName)
}

func replaceBinary(execPath string, newBinary []byte) error {
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}
	mode := info.Mode().Perm()

	dir := filepath.Dir(execPath)

	tmpFile, err := os.CreateTemp(dir, "wheresmybus-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(newBinary); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("set permissions: %w", err)
	}

	// Rename current → .old, then new → current. This works on all platforms
	// including Windows where a running binary can be renamed but not overwritten.
	oldPath := execPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(execPath, oldPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup current executable: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		_ = os.Rename(oldPath, execPath) // try to restore
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	_ = os.Remove(oldPath)
	return nil
}
