package calibre

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Skipf("Calibre not installed: %v", err)
	}

	if c.ebookMeta == "" {
		t.Error("ebook-meta path should be set")
	}
}

func TestVersion(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Skipf("Calibre not installed: %v", err)
	}

	version, err := c.Version()
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}

	if version == "" {
		t.Error("Version should not be empty")
	}

	t.Logf("Calibre version: %s", version)
}

func TestIsInstalled(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Skipf("Calibre not installed: %v", err)
	}

	if !c.IsInstalled() {
		t.Error("IsInstalled should return true")
	}
}

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()
	if len(formats) == 0 {
		t.Error("SupportedFormats should return formats")
	}

	// Check for common formats
	hasEPUB := false
	hasPDF := false
	for _, f := range formats {
		if f == "epub" {
			hasEPUB = true
		}
		if f == "pdf" {
			hasPDF = true
		}
	}

	if !hasEPUB {
		t.Error("Should support epub")
	}
	if !hasPDF {
		t.Error("Should support pdf")
	}
}

// TestGetMetadata tests metadata extraction with a real file
func TestGetMetadata(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Skipf("Calibre not installed: %v", err)
	}

	// Look for test files
	testFiles := []string{
		"/Users/anilpdv/Desktop/pg19942-images-3.epub",
		filepath.Join(os.TempDir(), "test.epub"),
	}

	var testFile string
	for _, f := range testFiles {
		if _, err := os.Stat(f); err == nil {
			testFile = f
			break
		}
	}

	if testFile == "" {
		t.Skip("No test EPUB file found")
	}

	meta, err := c.GetMetadata(testFile)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if meta.Title == "" {
		t.Error("Title should not be empty")
	}

	t.Logf("Title: %s", meta.Title)
	t.Logf("Authors: %v", meta.Authors)
	t.Logf("Language: %s", meta.Language)
}

// TestExtractChapters tests chapter extraction
func TestExtractChapters(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Skipf("Calibre not installed: %v", err)
	}

	if c.ebookConvert == "" {
		t.Skip("ebook-convert not found")
	}

	// Look for test file
	testFile := "/Users/anilpdv/Desktop/pg19942-images-3.epub"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found")
	}

	chapters, err := c.ExtractChapters(testFile)
	if err != nil {
		t.Fatalf("ExtractChapters failed: %v", err)
	}

	if len(chapters) == 0 {
		t.Error("Should extract at least one chapter")
	}

	for i, ch := range chapters {
		t.Logf("Chapter %d: %s (%d words)", i+1, ch.Title, ch.WordCount)
	}
}
