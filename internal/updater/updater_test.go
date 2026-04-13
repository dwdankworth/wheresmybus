package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "dev is always older", current: "dev", latest: "v0.1.0", want: true},
		{name: "empty is always older", current: "", latest: "v0.1.0", want: true},
		{name: "same version", current: "v0.1.1", latest: "v0.1.1", want: false},
		{name: "patch newer", current: "v0.1.0", latest: "v0.1.1", want: true},
		{name: "minor newer", current: "v0.1.0", latest: "v0.2.0", want: true},
		{name: "major newer", current: "v0.1.0", latest: "v1.0.0", want: true},
		{name: "current is newer", current: "v1.0.0", latest: "v0.9.9", want: false},
		{name: "without v prefix", current: "0.1.0", latest: "0.1.1", want: true},
		{name: "mixed v prefix", current: "v0.1.0", latest: "0.1.1", want: true},
		{name: "unparseable current treated as older", current: "abc", latest: "v0.1.0", want: true},
		{name: "unparseable latest not updated to", current: "v0.1.0", latest: "abc", want: false},
		{name: "both unparseable still treats current as outdated", current: "abc", latest: "xyz", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Fatalf("isNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  semver
		ok    bool
	}{
		{input: "v1.2.3", want: semver{1, 2, 3}, ok: true},
		{input: "0.1.0", want: semver{0, 1, 0}, ok: true},
		{input: "v10.20.30", want: semver{10, 20, 30}, ok: true},
		{input: "1.2", ok: false},
		{input: "v1.2.x", ok: false},
		{input: "", ok: false},
		{input: "dev", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseSemver(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseSemver(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindAsset(t *testing.T) {
	assets := []githubAsset{
		{Name: "wheresmybus_v0.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64.tar.gz"},
		{Name: "wheresmybus_v0.2.0_linux_arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux_arm64.tar.gz"},
		{Name: "wheresmybus_v0.2.0_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_amd64.tar.gz"},
		{Name: "wheresmybus_v0.2.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64.tar.gz"},
		{Name: "wheresmybus_v0.2.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64.zip"},
		{Name: "wheresmybus_v0.2.0_windows_arm64.zip", BrowserDownloadURL: "https://example.com/windows_arm64.zip"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
	}

	tests := []struct {
		name     string
		goos     string
		goarch   string
		wantName string
		wantURL  string
	}{
		{name: "linux amd64", goos: "linux", goarch: "amd64", wantName: "wheresmybus_v0.2.0_linux_amd64.tar.gz", wantURL: "https://example.com/linux_amd64.tar.gz"},
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", wantName: "wheresmybus_v0.2.0_darwin_arm64.tar.gz", wantURL: "https://example.com/darwin_arm64.tar.gz"},
		{name: "windows amd64", goos: "windows", goarch: "amd64", wantName: "wheresmybus_v0.2.0_windows_amd64.zip", wantURL: "https://example.com/windows_amd64.zip"},
		{name: "unsupported platform", goos: "freebsd", goarch: "amd64", wantName: "", wantURL: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, url := findAsset(assets, tt.goos, tt.goarch)
			if name != tt.wantName {
				t.Fatalf("findAsset name = %q, want %q", name, tt.wantName)
			}
			if url != tt.wantURL {
				t.Fatalf("findAsset url = %q, want %q", url, tt.wantURL)
			}
		})
	}
}

func TestCheckFromURL_UpdateAvailable(t *testing.T) {
	release := githubRelease{
		TagName: "v0.2.0",
		Assets:  testAssetsForCurrentPlatform("v0.2.0"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	result, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err != nil {
		t.Fatalf("CheckFromURL returned error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Fatal("expected UpdateAvailable = true")
	}
	if result.LatestVersion != "v0.2.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v0.2.0")
	}
	if result.AssetURL == "" {
		t.Fatal("expected AssetURL to be set")
	}
}

func TestCheckFromURL_AlreadyUpToDate(t *testing.T) {
	release := githubRelease{
		TagName: "v0.1.0",
		Assets:  testAssetsForCurrentPlatform("v0.1.0"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	result, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err != nil {
		t.Fatalf("CheckFromURL returned error: %v", err)
	}
	if result.UpdateAvailable {
		t.Fatal("expected UpdateAvailable = false")
	}
	if result.LatestVersion != "v0.1.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v0.1.0")
	}
}

func TestCheckFromURL_DevBuildCanUpdate(t *testing.T) {
	release := githubRelease{
		TagName: "v0.1.0",
		Assets:  testAssetsForCurrentPlatform("v0.1.0"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	result, err := CheckFromURL(server.Client(), server.URL, "dev")
	if err != nil {
		t.Fatalf("CheckFromURL returned error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Fatal("expected dev build to have UpdateAvailable = true")
	}
}

func TestCheckFromURL_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error = %q, want mention of HTTP 500", err.Error())
	}
}

func TestCheckFromURL_NoMatchingAsset(t *testing.T) {
	release := githubRelease{
		TagName: "v0.2.0",
		Assets: []githubAsset{
			{Name: "wheresmybus_v0.2.0_freebsd_amd64.tar.gz", BrowserDownloadURL: "https://example.com/freebsd"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	_, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
	if !strings.Contains(err.Error(), "no release asset") {
		t.Fatalf("error = %q, want mention of missing asset", err.Error())
	}
}

func TestCheckFromURL_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close() // close immediately to force connection error

	_, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for closed server")
	}
}

func TestExtractFromTarGz(t *testing.T) {
	want := []byte("fake-binary-content")

	archive := createTarGz(t, map[string][]byte{
		"wheresmybus_v0.2.0_linux_amd64/wheresmybus":   want,
		"wheresmybus_v0.2.0_linux_amd64/README.md":     []byte("readme"),
		"wheresmybus_v0.2.0_linux_amd64/.env.example":  []byte("example"),
	})

	got, err := extractFromTarGz(archive, "wheresmybus")
	if err != nil {
		t.Fatalf("extractFromTarGz error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted content = %q, want %q", got, want)
	}
}

func TestExtractFromTarGz_MissingBinary(t *testing.T) {
	archive := createTarGz(t, map[string][]byte{
		"wheresmybus_v0.2.0_linux_amd64/README.md": []byte("readme"),
	})

	_, err := extractFromTarGz(archive, "wheresmybus")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Fatalf("error = %q, want 'not found in archive'", err.Error())
	}
}

func TestExtractFromZip(t *testing.T) {
	want := []byte("fake-binary-content")

	archive := createZip(t, map[string][]byte{
		"wheresmybus_v0.2.0_windows_amd64/wheresmybus.exe": want,
		"wheresmybus_v0.2.0_windows_amd64/README.md":       []byte("readme"),
		"wheresmybus_v0.2.0_windows_amd64/.env.example":    []byte("example"),
	})

	got, err := extractFromZip(archive, "wheresmybus.exe")
	if err != nil {
		t.Fatalf("extractFromZip error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted content = %q, want %q", got, want)
	}
}

func TestExtractFromZip_MissingBinary(t *testing.T) {
	archive := createZip(t, map[string][]byte{
		"wheresmybus_v0.2.0_windows_amd64/README.md": []byte("readme"),
	})

	_, err := extractFromZip(archive, "wheresmybus.exe")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Fatalf("error = %q, want 'not found in archive'", err.Error())
	}
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "wheresmybus")

	original := []byte("original-binary")
	if err := os.WriteFile(execPath, original, 0o755); err != nil {
		t.Fatal(err)
	}

	updated := []byte("updated-binary")
	if err := replaceBinary(execPath, updated); err != nil {
		t.Fatalf("replaceBinary error: %v", err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if !bytes.Equal(got, updated) {
		t.Fatalf("binary content = %q, want %q", got, updated)
	}

	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	// Windows doesn't support Unix permission bits; skip this check there.
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("permissions = %o, want 755", info.Mode().Perm())
		}
	}

	// .old file should be cleaned up
	if _, err := os.Stat(execPath + ".old"); !os.IsNotExist(err) {
		t.Fatal("expected .old file to be removed")
	}
}

func TestReplaceBinary_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "wheresmybus")

	if err := os.WriteFile(execPath, []byte("original"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(execPath, []byte("updated")); err != nil {
		t.Fatalf("replaceBinary error: %v", err)
	}

	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	// Windows doesn't support Unix permission bits; skip this check there.
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("permissions = %o, want 700", info.Mode().Perm())
		}
	}
}

// testAssetsForCurrentPlatform returns a set of GitHub release assets
// that includes an asset matching the current runtime platform.
func testAssetsForCurrentPlatform(version string) []githubAsset {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	name := fmt.Sprintf("wheresmybus_%s_%s_%s.%s", version, runtime.GOOS, runtime.GOARCH, ext)
	return []githubAsset{
		{Name: name, BrowserDownloadURL: "https://example.com/" + name},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
	}
}

func createTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0o755,
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// currentBinaryName returns the expected binary name for the current platform.
func currentBinaryName() string {
	if runtime.GOOS == "windows" {
		return binaryName + ".exe"
	}
	return binaryName
}

// --- Phase 1: applyToPath tests ---

// Mutation: delete the replaceBinary call in applyToPath → file contents unchanged.
func TestApplyToPath_DownloadsAndReplacesFromTarGz(t *testing.T) {
	binContent := []byte("ELF-fake-binary-v0.3.0-linux-amd64")
	binName := currentBinaryName()

	archive := createTarGz(t, map[string][]byte{
		"wheresmybus_v0.3.0/" + binName: binContent,
		"wheresmybus_v0.3.0/README.md":  []byte("# release notes"),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, binName)
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	assetName := "wheresmybus_v0.3.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := applyToPath(server.Client(), server.URL, assetName, execPath); err != nil {
		t.Fatalf("applyToPath error: %v", err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if !bytes.Equal(got, binContent) {
		t.Fatalf("binary content = %q, want %q", got, binContent)
	}
}

// Mutation: change HasSuffix(assetName, ".zip") to always-false → zip archive sent to tar.gz extractor.
func TestApplyToPath_DownloadsAndReplacesFromZip(t *testing.T) {
	binContent := []byte("MZ-fake-binary-v0.3.0-windows-amd64")
	binName := currentBinaryName()

	archive := createZip(t, map[string][]byte{
		"wheresmybus_v0.3.0/" + binName: binContent,
		"wheresmybus_v0.3.0/README.md":  []byte("# release notes"),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, binName)
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	assetName := "wheresmybus_v0.3.0_windows_amd64.zip"
	if err := applyToPath(server.Client(), server.URL, assetName, execPath); err != nil {
		t.Fatalf("applyToPath error: %v", err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if !bytes.Equal(got, binContent) {
		t.Fatalf("binary content = %q, want %q", got, binContent)
	}
}

// Mutation: remove the error check after client.Get → nil pointer dereference.
func TestApplyToPath_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close() // close immediately to force connection refused

	dir := t.TempDir()
	execPath := filepath.Join(dir, currentBinaryName())
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := applyToPath(server.Client(), server.URL, "asset.tar.gz", execPath)
	if err == nil {
		t.Fatal("expected error for closed server")
	}
	if !strings.Contains(err.Error(), "download update") {
		t.Fatalf("error = %q, want wrapping of 'download update'", err.Error())
	}
}

// Mutation: remove the StatusCode != 200 check → garbage HTML passed to extractor.
func TestApplyToPath_HTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, currentBinaryName())
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := applyToPath(server.Client(), server.URL, "asset.tar.gz", execPath)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("error = %q, want mention of HTTP 403", err.Error())
	}
}

// Mutation: remove the extract error check → nil binaryData passed to replaceBinary.
func TestApplyToPath_CorruptArchiveData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("this-is-not-a-valid-gzip-or-tar-archive"))
	}))
	defer server.Close()

	dir := t.TempDir()
	execPath := filepath.Join(dir, currentBinaryName())
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := applyToPath(server.Client(), server.URL, "asset.tar.gz", execPath)
	if err == nil {
		t.Fatal("expected error for corrupt archive")
	}
	if !strings.Contains(err.Error(), "extract binary") {
		t.Fatalf("error = %q, want wrapping of 'extract binary'", err.Error())
	}

	// Original binary should be untouched since failure happens before replaceBinary.
	got, _ := os.ReadFile(execPath)
	if string(got) != "old" {
		t.Fatalf("original binary was modified despite extraction failure")
	}
}

// --- Phase 2: replaceBinary error path tests ---

// Mutation: remove os.Stat check → panic or incorrect behavior with bogus path.
func TestReplaceBinary_NonExistentPath(t *testing.T) {
	dir := t.TempDir()
	bogusPath := filepath.Join(dir, "nonexistent-binary")

	err := replaceBinary(bogusPath, []byte("new-content"))
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "stat executable") {
		t.Fatalf("error = %q, want 'stat executable' wrapping", err.Error())
	}
}

// Mutation: remove os.CreateTemp error check → nil pointer dereference on tmpFile.
func TestReplaceBinary_ReadOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission model not available on Windows")
	}

	dir := t.TempDir()
	execPath := filepath.Join(dir, "wheresmybus")
	if err := os.WriteFile(execPath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Make directory read-only so CreateTemp fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := replaceBinary(execPath, []byte("new-content"))
	if err == nil {
		t.Fatal("expected error for read-only directory")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Fatalf("error = %q, want 'create temp file' wrapping", err.Error())
	}

	// Original binary should be untouched.
	_ = os.Chmod(dir, 0o755)
	got, _ := os.ReadFile(execPath)
	if string(got) != "original" {
		t.Fatal("original binary was modified despite temp file creation failure")
	}
}

// Mutation: remove os.Remove(tmpPath) in rename error handler → temp file leaked.
func TestReplaceBinary_BackupRenameFails_CleansUpTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory-as-rename-blocker behavior varies on Windows")
	}

	dir := t.TempDir()
	execPath := filepath.Join(dir, "wheresmybus")
	if err := os.WriteFile(execPath, []byte("original-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-create execPath+".old" as a non-empty directory.
	// os.Remove(oldPath) silently fails (non-empty dir), then
	// os.Rename(execPath, oldPath) fails because oldPath is a directory.
	oldDir := execPath + ".old"
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(oldDir, "blocker.txt")
	if err := os.WriteFile(blocker, []byte("prevents removal"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := replaceBinary(execPath, []byte("updated-binary"))
	if err == nil {
		t.Fatal("expected error when backup rename fails")
	}
	if !strings.Contains(err.Error(), "backup current executable") {
		t.Fatalf("error = %q, want 'backup current executable' wrapping", err.Error())
	}

	// Original binary should still be intact.
	got, readErr := os.ReadFile(execPath)
	if readErr != nil {
		t.Fatalf("original binary unreadable: %v", readErr)
	}
	if string(got) != "original-binary" {
		t.Fatalf("original binary content = %q, want %q", got, "original-binary")
	}

	// Temp file should have been cleaned up.
	matches, _ := filepath.Glob(filepath.Join(dir, "wheresmybus-update-*"))
	if len(matches) > 0 {
		t.Fatalf("temp file not cleaned up: %v", matches)
	}
}

// --- Phase 3: Extract error path tests ---

// Mutation: remove gzip.NewReader error check → panic on nil reader.
func TestExtractFromTarGz_InvalidGzipData(t *testing.T) {
	_, err := extractFromTarGz([]byte("not-gzip-data-at-all"), "wheresmybus")
	if err == nil {
		t.Fatal("expected error for invalid gzip data")
	}
	if !strings.Contains(err.Error(), "open gzip") {
		t.Fatalf("error = %q, want 'open gzip' wrapping", err.Error())
	}
}

// Mutation: remove zip.NewReader error check → panic on nil reader.
func TestExtractFromZip_InvalidZipData(t *testing.T) {
	_, err := extractFromZip([]byte("not-a-zip-file-at-all"), "wheresmybus.exe")
	if err == nil {
		t.Fatal("expected error for invalid zip data")
	}
	if !strings.Contains(err.Error(), "open zip") {
		t.Fatalf("error = %q, want 'open zip' wrapping", err.Error())
	}
}

// --- Phase 4: CheckFromURL edge case ---

// Mutation: remove json.Decode error check → proceeds with zero-value release, wrongly
// reports "no update available" for any version string.
func TestCheckFromURL_MalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>unexpected error page</html>"))
	}))
	defer server.Close()

	_, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "decode release") {
		t.Fatalf("error = %q, want 'decode release' wrapping", err.Error())
	}
}

// Mutation: remove Accept header → GitHub might return a different format in the future.
func TestCheckFromURL_SetsAcceptHeader(t *testing.T) {
	var gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.1.0",
			Assets:  testAssetsForCurrentPlatform("v0.1.0"),
		})
	}))
	defer server.Close()

	_, err := CheckFromURL(server.Client(), server.URL, "v0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Fatalf("Accept header = %q, want %q", gotAccept, "application/vnd.github+json")
	}
}
