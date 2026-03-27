package geo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oschwald/geoip2-golang"
)

// ── Constants and Defaults ──────────────────────────────────────────────────

func TestDefaultConstants(t *testing.T) {
	// Verify default configuration constants
	if DefaultRefreshInterval <= 0 {
		t.Errorf("DefaultRefreshInterval should be positive, got %v", DefaultRefreshInterval)
	}
	if DefaultDBPath == "" {
		t.Error("DefaultDBPath should not be empty")
	}
	if downloadTimeout <= 0 {
		t.Errorf("downloadTimeout should be positive, got %v", downloadTimeout)
	}
	if maxDownloadRetries <= 0 {
		t.Errorf("maxDownloadRetries should be positive, got %d", maxDownloadRetries)
	}
	if retryBaseDelay <= 0 {
		t.Errorf("retryBaseDelay should be positive, got %v", retryBaseDelay)
	}
}

// ── Interface Compliance ─────────────────────────────────────────────────────

// Compile-time check: *DB satisfies the Database interface.
var _ Database = (*DB)(nil)

// Compile-time check: GeoReader interface includes Country and Close.
var _ GeoReader = (*geoip2.Reader)(nil)

// ── DB ───────────────────────────────────────────────────────────────────────

func TestDB_IsLoaded_FalseOnNew(t *testing.T) {
	db := &DB{}
	if db.IsLoaded() {
		t.Error("IsLoaded: got true for uninitialised DB, want false")
	}
}

func TestDB_Reader_NilWhenNotLoaded(t *testing.T) {
	db := &DB{}
	reader, unlock := db.Reader()
	defer unlock()
	if reader != nil {
		t.Error("Reader: got non-nil reader for unloaded DB, want nil")
	}
}

func TestDB_Reader_ReturnsUnlockFunction(t *testing.T) {
	db := &DB{}
	_, unlock := db.Reader()
	defer unlock()
	// Test that unlock is callable without panic (already called via defer)
}

func TestDB_IsLoaded_MultipleReads(t *testing.T) {
	// Ensure IsLoaded is consistent on multiple calls
	db := &DB{}
	for i := 0; i < 3; i++ {
		if db.IsLoaded() {
			t.Errorf("IsLoaded call %d: unexpected true for unloaded DB", i+1)
		}
	}
}

func TestDB_Reader_MultipleUnlocks(t *testing.T) {
	// Ensure multiple Reader() calls work correctly
	db := &DB{}

	reader1, unlock1 := db.Reader()
	reader2, unlock2 := db.Reader()

	// Both should return nil
	if reader1 != nil || reader2 != nil {
		t.Error("Expected nil readers for unloaded DB")
	}

	// Both unlocks should be callable without panic
	defer func() {
		unlock1()
		unlock2()
	}()
}

// ── extractMMDBFromTarGz ─────────────────────────────────────────────────────

// makeTarGz builds an in-memory tar.gz containing a single file at tarPath
// with the given content.
func makeTarGz(t *testing.T, tarPath string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name: tarPath,
		Mode: 0600,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar Write: %v", err)
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func TestExtractMMDBFromTarGz_Success(t *testing.T) {
	payload := []byte("fake mmdb content")
	archive := makeTarGz(t, "GeoLite2-Country_20240101/GeoLite2-Country.mmdb", payload)
	dest := filepath.Join(t.TempDir(), "out.mmdb")
	if err := extractMMDBFromTarGz(bytes.NewReader(archive), dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("reading dest file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("extracted content mismatch: got %q, want %q", got, payload)
	}
}

func TestExtractMMDBFromTarGz_NoMMDB(t *testing.T) {
	archive := makeTarGz(t, "README.txt", []byte("hello"))
	dest := filepath.Join(t.TempDir(), "out.mmdb")
	if err := extractMMDBFromTarGz(bytes.NewReader(archive), dest); err == nil {
		t.Error("expected error when archive contains no .mmdb, got nil")
	}
}

func TestExtractMMDBFromTarGz_InvalidGzip(t *testing.T) {
	if err := extractMMDBFromTarGz(bytes.NewReader([]byte("not gzip data")), "/tmp/irrelevant"); err == nil {
		t.Error("expected error for invalid gzip data, got nil")
	}
}

func TestExtractMMDBFromTarGz_SkipsNonMMDB(t *testing.T) {
	// Archive with two entries: a .txt that must be skipped, then the .mmdb.
	payload := []byte("real mmdb")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, entry := range []struct {
		name    string
		content []byte
	}{
		{"GeoLite2/README.txt", []byte("ignore me")},
		{"GeoLite2/GeoLite2-Country.mmdb", payload},
	} {
		hdr := &tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := tw.Write(entry.content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	_ = tw.Close()
	_ = gw.Close()
	dest := filepath.Join(t.TempDir(), "out.mmdb")
	if err := extractMMDBFromTarGz(&buf, dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("wrong file extracted: got %q, want %q", got, payload)
	}
}

func TestExtractMMDBFromTarGz_TarReadError(t *testing.T) {
	// Create a reader that returns an error
	errReader := &errorReader{}
	// After gzip header, should fail on tar read
	if err := extractMMDBFromTarGz(errReader, "/tmp/irrelevant"); err == nil {
		t.Error("expected error when tar read fails, got nil")
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

// ── Additional Archive Extraction Tests ──────────────────────────────────────

func TestExtractMMDBFromTarGz_MultipleFiles_SelectsFirst(t *testing.T) {
	// Test that when multiple .mmdb files exist, the first one is extracted
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	files := []struct {
		name    string
		content []byte
	}{
		{"GeoLite2/first.mmdb", []byte("first mmdb")},
		{"GeoLite2/second.mmdb", []byte("second mmdb")},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: 0600,
			Size: int64(len(file.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := tw.Write(file.content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	_ = tw.Close()
	_ = gw.Close()

	dest := filepath.Join(t.TempDir(), "out.mmdb")
	if err := extractMMDBFromTarGz(&buf, dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}

	if !bytes.Equal(got, []byte("first mmdb")) {
		t.Errorf("expected first mmdb to be extracted, got %q", got)
	}
}

func TestExtractMMDBFromTarGz_DeepPath(t *testing.T) {
	// Test extraction from deeply nested path in archive
	payload := []byte("deep mmdb")
	archive := makeTarGz(t, "path/to/deeply/nested/GeoLite2-Country.mmdb", payload)
	dest := filepath.Join(t.TempDir(), "out.mmdb")

	if err := extractMMDBFromTarGz(bytes.NewReader(archive), dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Errorf("deep path extraction: got %q, want %q", got, payload)
	}
}

func TestExtractMMDBFromTarGz_EmptyArchive(t *testing.T) {
	// Test with completely empty tar.gz
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.Close()
	_ = gw.Close()

	dest := filepath.Join(t.TempDir(), "out.mmdb")
	if err := extractMMDBFromTarGz(&buf, dest); err == nil {
		t.Error("expected error for empty archive, got nil")
	}
}

func TestExtractMMDBFromTarGz_LargeFile(t *testing.T) {
	// Test extraction of larger .mmdb file
	largeMmdb := bytes.Repeat([]byte("x"), 1024*1024) // 1MB file
	archive := makeTarGz(t, "GeoLite2-Country.mmdb", largeMmdb)
	dest := filepath.Join(t.TempDir(), "out.mmdb")

	if err := extractMMDBFromTarGz(bytes.NewReader(archive), dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}

	if len(got) != len(largeMmdb) {
		t.Errorf("large file extraction: got size %d, want %d", len(got), len(largeMmdb))
	}
}

// ── WriteFile Functionality ──────────────────────────────────────────────────

func TestWriteFile_Success(t *testing.T) {
	content := []byte("test file content")
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "test.mmdb")

	if err := writeFile(bytes.NewReader(content), dest); err != nil {
		t.Fatalf("writeFile: unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("written content mismatch: got %q, want %q", got, content)
	}
}

func TestWriteFile_ToNonexistentDirectory(t *testing.T) {
	content := []byte("test content")
	nonexistentPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "file.mmdb")

	if err := writeFile(bytes.NewReader(content), nonexistentPath); err == nil {
		t.Error("writeFile to nonexistent dir: expected error, got nil")
	}
}

func TestWriteFile_LargeContent(t *testing.T) {
	// Test writing large file
	largeContent := bytes.Repeat([]byte("x"), 10*1024*1024) // 10MB
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "large.mmdb")

	if err := writeFile(bytes.NewReader(largeContent), dest); err != nil {
		t.Fatalf("writeFile large: %v", err)
	}

	info, err := os.Stat(dest) //nolint:gosec
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}

	if info.Size() != int64(len(largeContent)) {
		t.Errorf("file size: got %d, want %d", info.Size(), len(largeContent))
	}
}
