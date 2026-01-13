package calibre

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anilpdv/go-calibre/models"
	"github.com/anilpdv/go-calibre/opf"
)

// GetMetadata extracts metadata from an ebook file
func (c *Calibre) GetMetadata(ebookPath string) (*models.Metadata, error) {
	return c.GetMetadataContext(context.Background(), ebookPath)
}

// GetMetadataContext extracts metadata with context for cancellation
func (c *Calibre) GetMetadataContext(ctx context.Context, ebookPath string) (*models.Metadata, error) {
	// Create temp file for OPF output
	tmpFile, err := os.CreateTemp("", "calibre-meta-*.opf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Run ebook-meta to extract metadata to OPF
	_, err = c.runCommand(ctx, c.ebookMeta, ebookPath, "--to-opf", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("ebook-meta failed: %w", err)
	}

	// Parse the OPF file
	parsed, err := opf.ParseFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OPF: %w", err)
	}

	// Convert to our Metadata struct
	return &models.Metadata{
		Title:       parsed.Title,
		Authors:     parsed.Authors,
		AuthorSort:  parsed.AuthorSort,
		Publisher:   parsed.Publisher,
		PublishDate: parsed.PublishDate.Format("2006-01-02"),
		Language:    parsed.Language,
		ISBN:        parsed.ISBN,
		Identifiers: parsed.Identifiers,
		Tags:        parsed.Tags,
		Series:      parsed.Series,
		SeriesIndex: parsed.SeriesIndex,
		Description: parsed.Description,
	}, nil
}

// ExtractCover extracts the cover image from an ebook
func (c *Calibre) ExtractCover(ebookPath, outputPath string) error {
	return c.ExtractCoverContext(context.Background(), ebookPath, outputPath)
}

// ExtractCoverContext extracts cover with context for cancellation
func (c *Calibre) ExtractCoverContext(ctx context.Context, ebookPath, outputPath string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Run ebook-meta with --get-cover
	_, err := c.runCommand(ctx, c.ebookMeta, ebookPath, "--get-cover", outputPath)
	if err != nil {
		return fmt.Errorf("failed to extract cover: %w", err)
	}

	// Verify cover was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("cover extraction produced no output (book may have no cover)")
	}

	return nil
}

// GetBook extracts full book info including metadata
func (c *Calibre) GetBook(ebookPath string) (*models.Book, error) {
	return c.GetBookContext(context.Background(), ebookPath)
}

// GetBookContext extracts book with context
func (c *Calibre) GetBookContext(ctx context.Context, ebookPath string) (*models.Book, error) {
	// Get metadata first
	meta, err := c.GetMetadataContext(ctx, ebookPath)
	if err != nil {
		return nil, err
	}

	book := &models.Book{
		Title:       meta.Title,
		Authors:     meta.Authors,
		Publisher:   meta.Publisher,
		Language:    meta.Language,
		ISBN:        meta.ISBN,
		Identifiers: meta.Identifiers,
		Tags:        meta.Tags,
		Series:      meta.Series,
		SeriesIndex: meta.SeriesIndex,
		Description: meta.Description,
		FilePath:    ebookPath,
		Format:      filepath.Ext(ebookPath),
	}

	return book, nil
}
