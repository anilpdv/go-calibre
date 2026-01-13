package calibre

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anilpdv/go-calibre/models"
	"github.com/anilpdv/go-calibre/ncx"
)

// ChapterOptions configures chapter extraction
type ChapterOptions struct {
	// ChapterXPath is an XPath expression to detect chapter boundaries
	// If empty, Calibre's auto-detection is used
	ChapterXPath string

	// ChapterMark controls how chapters are marked: "pagebreak", "rule", "none", "both"
	ChapterMark string

	// KeepHTML preserves HTML content in addition to plain text
	KeepHTML bool
}

// ExtractChapters extracts chapters from an ebook using Calibre's chapter detection
func (c *Calibre) ExtractChapters(ebookPath string) ([]models.Chapter, error) {
	return c.ExtractChaptersWithOptions(context.Background(), ebookPath, ChapterOptions{})
}

// ExtractChaptersContext extracts chapters with context
func (c *Calibre) ExtractChaptersContext(ctx context.Context, ebookPath string) ([]models.Chapter, error) {
	return c.ExtractChaptersWithOptions(ctx, ebookPath, ChapterOptions{})
}

// ExtractChaptersWithOptions extracts chapters with custom options
func (c *Calibre) ExtractChaptersWithOptions(ctx context.Context, ebookPath string, opts ChapterOptions) ([]models.Chapter, error) {
	if c.ebookConvert == "" {
		return nil, fmt.Errorf("ebook-convert not found")
	}

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "calibre-chapters-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// First, try NCX-based extraction (Calibre's proper chapter API)
	chapters, err := c.extractChaptersWithNCX(ctx, ebookPath, tmpDir, opts)
	if err == nil && len(chapters) > 0 {
		return chapters, nil
	}

	// Fallback to text-based extraction with regex
	return c.extractChaptersWithText(ctx, ebookPath, tmpDir, opts)
}

// extractChaptersWithNCX uses the NCX table of contents for proper chapter detection
func (c *Calibre) extractChaptersWithNCX(ctx context.Context, ebookPath, tmpDir string, opts ChapterOptions) ([]models.Chapter, error) {
	// First, try to use the original EPUB's NCX (often has better chapter titles)
	if strings.HasSuffix(strings.ToLower(ebookPath), ".epub") {
		chapters, err := c.extractChaptersFromOriginalNCX(ebookPath)
		if err == nil && len(chapters) >= 3 {
			return chapters, nil
		}
	}

	// Fallback: Convert to EPUB with Calibre's chapter detection
	return c.extractChaptersWithCalibreNCX(ctx, ebookPath, tmpDir, opts)
}

// extractChaptersFromOriginalNCX extracts chapters using the original EPUB's NCX
func (c *Calibre) extractChaptersFromOriginalNCX(epubPath string) ([]models.Chapter, error) {
	// Parse the NCX from the original EPUB
	ncxDoc, err := ncx.ExtractNCXFromEPUB(epubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract NCX: %w", err)
	}

	// Get TOC entries from NCX
	tocEntries := ncxDoc.GetTOC()
	if len(tocEntries) == 0 {
		return nil, fmt.Errorf("no chapters found in NCX")
	}

	// Filter to get only chapter-like entries (skip front matter, etc.)
	chapterEntries := filterChapterEntries(tocEntries)
	if len(chapterEntries) == 0 {
		return nil, fmt.Errorf("no chapter entries found")
	}

	// Extract chapter content for each entry
	var chapters []models.Chapter
	for i, entry := range chapterEntries {
		// Get the next href for range extraction
		nextHref := ""
		if i+1 < len(chapterEntries) {
			nextHref = chapterEntries[i+1].Href
		}

		// Get chapter content from the EPUB using the href range
		content, err := ncx.GetChapterContentRange(epubPath, entry.Href, nextHref)
		if err != nil {
			// Skip chapters we can't extract content for
			continue
		}

		// Skip very short content (likely front matter or navigation)
		if len(strings.Fields(content)) < 50 {
			continue
		}

		title := entry.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}

		chapters = append(chapters, models.NewChapter(len(chapters), title, content))
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("failed to extract any chapter content")
	}

	return chapters, nil
}

// filterChapterEntries filters TOC entries to get actual chapter content
func filterChapterEntries(entries []ncx.TOCEntry) []ncx.TOCEntry {
	var chapters []ncx.TOCEntry

	// Skip common front/back matter patterns
	skipPatterns := []string{
		"transcriber", "note", "copyright", "dedication", "epigraph",
		"acknowledgment", "about the author", "about the book",
		"the full project gutenberg", "project gutenberg", "license",
		"the modern library", "footnotes", "endnotes", "index",
		"bibliography", "contents", "table of contents",
	}

	for _, entry := range entries {
		titleLower := strings.ToLower(entry.Title)

		// Skip entries that match skip patterns
		skip := false
		for _, pattern := range skipPatterns {
			if strings.Contains(titleLower, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Skip very short titles (likely navigation elements)
		if len(entry.Title) < 2 {
			continue
		}

		// Look for chapter-like entries
		// - Has "chapter" in title
		// - Has Roman numerals (I, II, III, etc.) potentially with more text
		// - Level 2 entries are often actual chapters
		isChapter := false
		if strings.Contains(titleLower, "chapter") ||
			strings.Contains(titleLower, "part") ||
			regexp.MustCompile(`^\s*(I{1,3}|IV|V|VI{0,3}|IX|X{0,3}|[0-9]+)\s*\.?\s+\w`).MatchString(entry.Title) ||
			entry.Level >= 2 {
			isChapter = true
		}

		// Also include titles that look like chapter headings (start with Roman/Arabic number + text)
		if regexp.MustCompile(`(?i)^(chapter|part)\s+`).MatchString(entry.Title) {
			isChapter = true
		}

		// Include entries with descriptive titles (HOW CANDIDE WAS BROUGHT UP...)
		if len(entry.Title) > 20 && strings.Contains(entry.Title, " ") {
			isChapter = true
		}

		if isChapter {
			chapters = append(chapters, entry)
		}
	}

	return chapters
}

// extractChaptersWithCalibreNCX uses Calibre's conversion to generate NCX
func (c *Calibre) extractChaptersWithCalibreNCX(ctx context.Context, ebookPath, tmpDir string, opts ChapterOptions) ([]models.Chapter, error) {
	// Convert to EPUB with proper chapter detection
	epubPath := filepath.Join(tmpDir, "book.epub")

	args := []string{ebookPath, epubPath}

	// Add chapter detection XPath - Calibre will generate NCX with chapter info
	if opts.ChapterXPath != "" {
		args = append(args, "--chapter", opts.ChapterXPath)
	} else {
		// Default to common heading tags for chapter detection
		args = append(args, "--chapter", "//h:h1|//h:h2|//h:h3")
	}

	// Force TOC generation
	args = append(args, "--use-auto-toc")
	args = append(args, "--level1-toc", "//h:h1")
	args = append(args, "--level2-toc", "//h:h2")

	_, err := c.runCommand(ctx, c.ebookConvert, args...)
	if err != nil {
		return nil, fmt.Errorf("ebook-convert to EPUB failed: %w", err)
	}

	// Parse the NCX from the converted EPUB
	ncxDoc, err := ncx.ExtractNCXFromEPUB(epubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract NCX: %w", err)
	}

	// Get TOC entries from NCX
	tocEntries := ncxDoc.GetTOC()
	if len(tocEntries) == 0 {
		return nil, fmt.Errorf("no chapters found in NCX")
	}

	// Extract chapter content for each TOC entry
	var chapters []models.Chapter
	for i, entry := range tocEntries {
		// Get chapter content from the EPUB using the href
		content, err := ncx.GetChapterContent(epubPath, entry.Href)
		if err != nil {
			// Skip chapters we can't extract content for
			continue
		}

		// Use the TOC title (from Calibre's detection) as the chapter title
		title := entry.Title
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}

		chapters = append(chapters, models.NewChapter(i, title, content))
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("failed to extract any chapter content")
	}

	return chapters, nil
}

// extractChaptersWithText is the fallback regex-based chapter extraction
func (c *Calibre) extractChaptersWithText(ctx context.Context, ebookPath, tmpDir string, opts ChapterOptions) ([]models.Chapter, error) {
	// Convert to plain text for content extraction
	txtPath := filepath.Join(tmpDir, "book.txt")
	txtArgs := []string{ebookPath, txtPath}
	if opts.ChapterMark == "" {
		txtArgs = append(txtArgs, "--chapter-mark", "pagebreak")
	} else {
		txtArgs = append(txtArgs, "--chapter-mark", opts.ChapterMark)
	}

	_, err := c.runCommand(ctx, c.ebookConvert, txtArgs...)
	if err != nil {
		return nil, fmt.Errorf("ebook-convert to txt failed: %w", err)
	}

	// Read the text content
	txtContent, err := os.ReadFile(txtPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read text output: %w", err)
	}

	// Split by page breaks (form feed character or multiple newlines)
	chapters := splitIntoChapters(string(txtContent))

	return chapters, nil
}

// splitIntoChapters splits text content into chapters
func splitIntoChapters(content string) []models.Chapter {
	var chapters []models.Chapter

	// Calibre uses form feed (\f) or page break markers
	// Also try splitting on common chapter patterns

	// First try form feed (page break)
	parts := strings.Split(content, "\f")
	if len(parts) <= 1 {
		// Try splitting by "* * *" separator (common in Gutenberg books)
		parts = splitByStarSeparator(content)
	}
	if len(parts) <= 1 {
		// Try chapter heading patterns
		parts = splitByChapterPatterns(content)
	}

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		title := detectChapterTitle(part, i+1)
		chapters = append(chapters, models.NewChapter(i, title, part))
	}

	return chapters
}

// splitByStarSeparator splits content by "* * *" separators (common in Project Gutenberg)
func splitByStarSeparator(content string) []string {
	// Match various star/asterisk separators
	re := regexp.MustCompile(`\n\s*\*\s*\*\s*\*\s*\n`)
	parts := re.Split(content, -1)

	// Filter out short parts (likely front/back matter)
	var chapters []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		// Only keep substantial content (skip front matter, TOC, etc.)
		if len(trimmed) > 500 {
			chapters = append(chapters, trimmed)
		}
	}

	// If we found at least 3 chapters, use this split
	if len(chapters) >= 3 {
		return chapters
	}

	return []string{content}
}

// splitByChapterPatterns splits content by chapter heading patterns
func splitByChapterPatterns(content string) []string {
	// Pattern to match chapter headings (order matters - try most specific first)
	patterns := []string{
		// "Chapter 1" or "CHAPTER I" style
		`(?m)^(Chapter|CHAPTER)\s+(\d+|[IVXLC]+)`,
		// "I. Title text" - Roman numeral with title (like Candide)
		`(?m)^([IVXLC]+)\.\s+[A-Z]`,
		// "1. Title text" - Arabic numeral with title
		`(?m)^(\d+)\.\s+[A-Z]`,
		// Standalone Roman numeral on its own line
		`(?m)^([IVXLC]+)\.\s*$`,
		// Standalone number on its own line
		`(?m)^(\d+)\.\s*$`,
		// "Part 1" style
		`(?m)^Part\s+(\d+|[IVXLC]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(content, -1)

		if len(matches) >= 3 { // Need at least 3 chapters to be confident
			var parts []string
			lastEnd := 0

			for _, match := range matches {
				if match[0] > lastEnd {
					// Add content before this match if it's substantial
					before := strings.TrimSpace(content[lastEnd:match[0]])
					if len(before) > 100 && lastEnd > 0 {
						parts = append(parts, before)
					}
				}
				lastEnd = match[0]
			}

			// Add remaining content
			if lastEnd < len(content) {
				remaining := strings.TrimSpace(content[lastEnd:])
				if len(remaining) > 100 {
					parts = append(parts, remaining)
				}
			}

			if len(parts) >= 3 {
				return parts
			}
		}
	}

	// Fallback: return as single chapter
	return []string{content}
}

// detectChapterTitle extracts the chapter title from the beginning of content
func detectChapterTitle(content string, defaultNum int) string {
	lines := strings.Split(content, "\n")

	// Collect first few non-empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmptyLines = append(nonEmptyLines, line)
		if len(nonEmptyLines) >= 5 {
			break
		}
	}

	if len(nonEmptyLines) == 0 {
		return fmt.Sprintf("Chapter %d", defaultNum)
	}

	// Check for Roman numeral followed by title on next line (Project Gutenberg style)
	// e.g., "I" on line 1, "HOW CANDIDE WAS BROUGHT UP..." on line 2
	if len(nonEmptyLines) >= 2 {
		if regexp.MustCompile(`^[IVXLC]+$`).MatchString(nonEmptyLines[0]) {
			romanNum := nonEmptyLines[0]
			titleLine := nonEmptyLines[1]
			// If second line is a title (caps or title case, reasonable length)
			if len(titleLine) > 5 && len(titleLine) < 100 {
				return fmt.Sprintf("Chapter %s: %s", romanNum, titleCase(titleLine))
			}
			return fmt.Sprintf("Chapter %s", romanNum)
		}
	}

	// Check first line for chapter patterns
	firstLine := nonEmptyLines[0]

	// "Chapter X" or "CHAPTER X"
	if re := regexp.MustCompile(`^(Chapter|CHAPTER)\s+(\d+|[IVXLC]+)`); re.MatchString(firstLine) {
		if len(firstLine) < 80 {
			return firstLine
		}
	}

	// "I. Title text" format
	if re := regexp.MustCompile(`^([IVXLC]+)\.\s+(.+)$`); re.MatchString(firstLine) {
		matches := re.FindStringSubmatch(firstLine)
		if len(matches) >= 3 {
			return fmt.Sprintf("Chapter %s: %s", matches[1], titleCase(matches[2]))
		}
	}

	// Standalone Roman numeral
	if regexp.MustCompile(`^[IVXLC]+\.?$`).MatchString(firstLine) {
		return formatChapterTitle(firstLine)
	}

	// Standalone number
	if regexp.MustCompile(`^\d+\.?$`).MatchString(firstLine) {
		return formatChapterTitle(firstLine)
	}

	// Short line that looks like a title
	if len(firstLine) < 60 && len(firstLine) > 3 {
		return titleCase(firstLine)
	}

	return fmt.Sprintf("Chapter %d", defaultNum)
}

// titleCase converts a string to title case
func titleCase(s string) string {
	// If it's all caps, convert to title case
	if s == strings.ToUpper(s) && len(s) > 10 {
		words := strings.Fields(strings.ToLower(s))
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(string(word[0])) + word[1:]
			}
		}
		return strings.Join(words, " ")
	}
	return s
}

// formatChapterTitle normalizes a chapter title
func formatChapterTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimSuffix(title, ".")

	// Convert standalone Roman numerals to "Chapter X"
	if regexp.MustCompile(`^[IVXLC]+$`).MatchString(title) {
		return fmt.Sprintf("Chapter %s", title)
	}

	// Convert standalone numbers to "Chapter N"
	if regexp.MustCompile(`^\d+$`).MatchString(title) {
		return fmt.Sprintf("Chapter %s", title)
	}

	return title
}

// GetTOC extracts the table of contents from an ebook
func (c *Calibre) GetTOC(ebookPath string) ([]models.TOCEntry, error) {
	return c.GetTOCContext(context.Background(), ebookPath)
}

// GetTOCContext extracts TOC with context
func (c *Calibre) GetTOCContext(ctx context.Context, ebookPath string) ([]models.TOCEntry, error) {
	// For now, extract chapters and use their titles as TOC
	// A more complete implementation would parse the NCX/NAV file from EPUB
	chapters, err := c.ExtractChaptersContext(ctx, ebookPath)
	if err != nil {
		return nil, err
	}

	var toc []models.TOCEntry
	for _, ch := range chapters {
		toc = append(toc, models.TOCEntry{
			Title: ch.Title,
			Level: 1,
		})
	}

	return toc, nil
}
