// Package calibre provides a Go wrapper around Calibre's command-line tools
// for professional-grade ebook parsing, metadata extraction, chapter detection,
// and format conversion.
//
// This library requires Calibre to be installed on the system.
// Install via: brew install calibre (macOS) or apt install calibre (Linux)
package calibre

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// DefaultTimeout is the default timeout for Calibre commands
const DefaultTimeout = 5 * time.Minute

// Calibre holds configuration for the Calibre wrapper
type Calibre struct {
	// Path to Calibre binaries (auto-detected if empty)
	BinPath string

	// Timeout for commands (defaults to 5 minutes)
	Timeout time.Duration

	// Paths to individual tools (auto-detected)
	ebookMeta    string
	ebookConvert string
	fetchMeta    string
	ebookPolish  string
	calibredb    string
}

// New creates a new Calibre instance with auto-detected paths
func New() (*Calibre, error) {
	c := &Calibre{
		Timeout: DefaultTimeout,
	}

	if err := c.detectTools(); err != nil {
		return nil, err
	}

	return c, nil
}

// detectTools finds the paths to Calibre command-line tools
func (c *Calibre) detectTools() error {
	tools := map[string]*string{
		"ebook-meta":           &c.ebookMeta,
		"ebook-convert":        &c.ebookConvert,
		"fetch-ebook-metadata": &c.fetchMeta,
		"ebook-polish":         &c.ebookPolish,
		"calibredb":            &c.calibredb,
	}

	for name, path := range tools {
		p, err := exec.LookPath(name)
		if err != nil {
			// ebook-meta is required, others are optional
			if name == "ebook-meta" {
				return fmt.Errorf("calibre not found: %s not in PATH. Install with: brew install calibre", name)
			}
			continue
		}
		*path = p
	}

	return nil
}

// Version returns the installed Calibre version
func (c *Calibre) Version() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.ebookMeta, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	// Parse version from output like "ebook-meta (calibre 8.16.2)"
	re := regexp.MustCompile(`calibre\s+(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse version from: %s", output)
	}

	return matches[1], nil
}

// IsInstalled checks if Calibre is properly installed
func (c *Calibre) IsInstalled() bool {
	_, err := c.Version()
	return err == nil
}

// SupportedFormats returns the list of formats Calibre can read
func SupportedFormats() []string {
	return []string{
		"azw", "azw1", "azw3", "azw4", "cb7", "cbc", "cbr", "cbz",
		"chm", "docx", "epub", "fb2", "fbz", "html", "htmlz", "imp",
		"kepub", "lit", "lrf", "lrx", "mobi", "odt", "oebzip", "opf",
		"pdb", "pdf", "pml", "pmlz", "pobi", "prc", "rar", "rb",
		"rtf", "snb", "tpz", "txt", "txtz", "updb", "zip",
	}
}

// runCommand executes a Calibre command with timeout
func (c *Calibre) runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %v", c.Timeout)
		}
		return nil, fmt.Errorf("command failed: %w\nOutput: %s", err, strings.TrimSpace(string(output)))
	}

	return output, nil
}
