package models

// Chapter represents a single chapter extracted from an ebook
type Chapter struct {
	// Index is the chapter number (0-based)
	Index int

	// Title is the chapter title from TOC or detected heading
	Title string

	// Content is the plain text content of the chapter
	Content string

	// HTMLContent is the original HTML content (if available)
	HTMLContent string

	// WordCount is the approximate word count
	WordCount int

	// CharCount is the character count
	CharCount int
}

// NewChapter creates a new chapter with the given index and title
func NewChapter(index int, title, content string) Chapter {
	return Chapter{
		Index:     index,
		Title:     title,
		Content:   content,
		WordCount: countWords(content),
		CharCount: len(content),
	}
}

// countWords provides a simple word count
func countWords(text string) int {
	if text == "" {
		return 0
	}

	count := 0
	inWord := false

	for _, r := range text {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if isSpace {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}

	return count
}

// IsEmpty returns true if the chapter has no content
func (c *Chapter) IsEmpty() bool {
	return len(c.Content) == 0
}

// Summary returns the first N characters of content as a preview
func (c *Chapter) Summary(maxLen int) string {
	if len(c.Content) <= maxLen {
		return c.Content
	}

	// Try to break at a word boundary
	text := c.Content[:maxLen]
	for i := len(text) - 1; i > maxLen-20; i-- {
		if text[i] == ' ' {
			return text[:i] + "..."
		}
	}

	return text + "..."
}
