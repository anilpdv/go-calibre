// Package ncx provides parsing for NCX (Navigation Control file for XML)
// which is the table of contents format used in EPUB 2 and many EPUB 3 books.
package ncx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// NCX represents the root NCX document
type NCX struct {
	XMLName  xml.Name `xml:"ncx"`
	DocTitle DocTitle `xml:"docTitle"`
	NavMap   NavMap   `xml:"navMap"`
}

// DocTitle contains the document title
type DocTitle struct {
	Text string `xml:"text"`
}

// NavMap contains the navigation points (chapters)
type NavMap struct {
	NavPoints []NavPoint `xml:"navPoint"`
}

// NavPoint represents a navigation point (chapter)
type NavPoint struct {
	ID        string     `xml:"id,attr"`
	PlayOrder int        `xml:"playOrder,attr"`
	Class     string     `xml:"class,attr"`
	Label     NavLabel   `xml:"navLabel"`
	Content   Content    `xml:"content"`
	Children  []NavPoint `xml:"navPoint"` // Nested chapters
}

// NavLabel contains the chapter title
type NavLabel struct {
	Text string `xml:"text"`
}

// Content contains the link to chapter content
type Content struct {
	Src string `xml:"src,attr"`
}

// TOCEntry represents a parsed table of contents entry
type TOCEntry struct {
	Title    string
	Level    int
	Href     string // Reference to content file
	Order    int
	Children []TOCEntry
}

// ParseNCX parses NCX XML content
func ParseNCX(r io.Reader) (*NCX, error) {
	var ncx NCX
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&ncx); err != nil {
		return nil, fmt.Errorf("failed to parse NCX: %w", err)
	}
	return &ncx, nil
}

// ParseNCXBytes parses NCX from bytes
func ParseNCXBytes(data []byte) (*NCX, error) {
	return ParseNCX(strings.NewReader(string(data)))
}

// GetTOC extracts a flat list of TOC entries from the NCX
func (ncx *NCX) GetTOC() []TOCEntry {
	var entries []TOCEntry
	for _, np := range ncx.NavMap.NavPoints {
		entries = append(entries, flattenNavPoint(np, 1)...)
	}
	return entries
}

// flattenNavPoint recursively flattens a NavPoint and its children
func flattenNavPoint(np NavPoint, level int) []TOCEntry {
	entry := TOCEntry{
		Title: strings.TrimSpace(np.Label.Text),
		Level: level,
		Href:  np.Content.Src,
		Order: np.PlayOrder,
	}

	var entries []TOCEntry
	entries = append(entries, entry)

	for _, child := range np.Children {
		entries = append(entries, flattenNavPoint(child, level+1)...)
	}

	return entries
}

// ExtractNCXFromEPUB extracts and parses the NCX file from an EPUB
func ExtractNCXFromEPUB(epubPath string) (*NCX, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer r.Close()

	// Look for NCX file
	var ncxFile *zip.File
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".ncx") {
			ncxFile = f
			break
		}
	}

	if ncxFile == nil {
		return nil, fmt.Errorf("NCX file not found in EPUB")
	}

	rc, err := ncxFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open NCX file: %w", err)
	}
	defer rc.Close()

	return ParseNCX(rc)
}

// GetChapterContent extracts the content of a specific chapter from an EPUB
// If nextHref is provided, content will be extracted from the current href's fragment
// up to the next href's fragment
func GetChapterContent(epubPath, href string) (string, error) {
	return GetChapterContentRange(epubPath, href, "")
}

// GetChapterContentRange extracts content between two fragment identifiers
func GetChapterContentRange(epubPath, href, nextHref string) (string, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return "", fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer r.Close()

	// Parse the href and fragment
	hrefParts := strings.SplitN(href, "#", 2)
	filePath := hrefParts[0]
	startFragment := ""
	if len(hrefParts) > 1 {
		startFragment = hrefParts[1]
	}

	// Parse the next href fragment if provided
	endFragment := ""
	if nextHref != "" {
		nextParts := strings.SplitN(nextHref, "#", 2)
		// Only use end fragment if it's the same file
		if len(nextParts) > 1 && (nextParts[0] == filePath || nextParts[0] == "" || strings.HasSuffix(filePath, nextParts[0])) {
			endFragment = nextParts[1]
		}
	}

	// Normalize path
	filePath = filepath.Clean(filePath)

	// Try to find the file
	for _, f := range r.File {
		// Try exact match and with OEBPS prefix
		if f.Name == filePath || strings.HasSuffix(f.Name, filePath) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			html := string(data)

			// If we have fragment identifiers, extract just that portion
			if startFragment != "" {
				html = extractFragmentContent(html, startFragment, endFragment)
			}

			return htmlToText(html), nil
		}
	}

	return "", fmt.Errorf("chapter file not found: %s", filePath)
}

// extractFragmentContent extracts HTML content between two fragment identifiers
func extractFragmentContent(html, startFragment, endFragment string) string {
	// Find the start element with the given id
	startPatterns := []string{
		fmt.Sprintf(`id="%s"`, startFragment),
		fmt.Sprintf(`id='%s'`, startFragment),
		fmt.Sprintf(`name="%s"`, startFragment),
		fmt.Sprintf(`name='%s'`, startFragment),
	}

	startIdx := -1
	for _, pattern := range startPatterns {
		idx := strings.Index(html, pattern)
		if idx != -1 {
			startIdx = idx
			break
		}
	}

	if startIdx == -1 {
		// Fragment not found, return all content
		return html
	}

	// Find the end fragment if specified
	endIdx := len(html)
	if endFragment != "" {
		endPatterns := []string{
			fmt.Sprintf(`id="%s"`, endFragment),
			fmt.Sprintf(`id='%s'`, endFragment),
			fmt.Sprintf(`name="%s"`, endFragment),
			fmt.Sprintf(`name='%s'`, endFragment),
		}

		for _, pattern := range endPatterns {
			idx := strings.Index(html[startIdx+1:], pattern)
			if idx != -1 {
				endIdx = startIdx + 1 + idx
				break
			}
		}
	}

	// Extract the content between start and end
	content := html[startIdx:endIdx]

	// Try to find the closing tag of the element containing the start fragment
	// and include content until we hit another major section
	return content
}

// htmlToText converts HTML to plain text (simple version)
func htmlToText(html string) string {
	// Remove script and style tags
	html = removeTag(html, "script")
	html = removeTag(html, "style")

	// Convert block elements to newlines
	for _, tag := range []string{"p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr"} {
		html = strings.ReplaceAll(html, "<"+tag, "\n<"+tag)
		html = strings.ReplaceAll(html, "</"+tag+">", "\n")
	}

	// Remove all remaining HTML tags
	result := strings.Builder{}
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}

	// Clean up whitespace
	text := result.String()
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n\n")
}

func removeTag(html, tag string) string {
	// Simple tag removal - not perfect but works for most cases
	for {
		start := strings.Index(strings.ToLower(html), "<"+tag)
		if start == -1 {
			break
		}
		end := strings.Index(html[start:], "</"+tag+">")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+len("</"+tag+">"):]
	}
	return html
}
