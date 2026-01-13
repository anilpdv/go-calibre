package models

import "time"

// Book represents a complete ebook with metadata and chapters
type Book struct {
	// Core metadata
	Title       string
	Authors     []string
	Language    string
	Publisher   string
	PublishDate time.Time
	Description string

	// Identifiers
	ISBN       string
	Identifiers map[string]string // asin, goodreads, etc.

	// Classification
	Tags   []string
	Series string
	SeriesIndex float64

	// Content
	Chapters []Chapter
	TOC      []TOCEntry

	// Files
	FilePath   string
	Format     string
	CoverPath  string
	CoverData  []byte
}

// Metadata represents just the metadata portion of a book
type Metadata struct {
	Title         string            `json:"title"`
	Authors       []string          `json:"authors"`
	AuthorSort    string            `json:"author_sort"`
	Publisher     string            `json:"publisher"`
	PublishDate   string            `json:"publish_date"`
	Language      string            `json:"language"`
	ISBN          string            `json:"isbn"`
	Identifiers   map[string]string `json:"identifiers"`
	Tags          []string          `json:"tags"`
	Series        string            `json:"series"`
	SeriesIndex   float64           `json:"series_index"`
	Rating        int               `json:"rating"` // 1-5
	Description   string            `json:"description"`
	Comments      string            `json:"comments"`
	BookProducer  string            `json:"book_producer"`
}

// TOCEntry represents an entry in the table of contents
type TOCEntry struct {
	Title    string
	Level    int    // Nesting level (1 = top level)
	Href     string // Link to content
	Children []TOCEntry
}

// PrimaryAuthor returns the first author or empty string
func (b *Book) PrimaryAuthor() string {
	if len(b.Authors) > 0 {
		return b.Authors[0]
	}
	return ""
}

// HasChapters returns true if chapters have been extracted
func (b *Book) HasChapters() bool {
	return len(b.Chapters) > 0
}

// ChapterCount returns the number of chapters
func (b *Book) ChapterCount() int {
	return len(b.Chapters)
}
