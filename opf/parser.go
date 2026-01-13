// Package opf provides parsing for OPF (Open Package Format) XML files
// which is the standard metadata format used by Calibre and EPUB files.
package opf

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Package represents the root OPF package element
type Package struct {
	XMLName  xml.Name `xml:"package"`
	Metadata Metadata `xml:"metadata"`
}

// Metadata contains Dublin Core metadata elements
type Metadata struct {
	Title       string      `xml:"title"`
	Creators    []Creator   `xml:"creator"`
	Publisher   string      `xml:"publisher"`
	Date        string      `xml:"date"`
	Language    string      `xml:"language"`
	Subjects    []string    `xml:"subject"`
	Description string      `xml:"description"`
	Identifiers []Identifier `xml:"identifier"`
	Meta        []Meta      `xml:"meta"`
}

// Creator represents a dc:creator element (author)
type Creator struct {
	Name   string `xml:",chardata"`
	Role   string `xml:"role,attr"`
	FileAs string `xml:"file-as,attr"`
}

// Identifier represents a dc:identifier element (ISBN, UUID, etc.)
type Identifier struct {
	ID     string `xml:"id,attr"`
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}

// Meta represents a calibre or opf meta element
type Meta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// ParsedMetadata is the clean Go struct with parsed metadata
type ParsedMetadata struct {
	Title         string
	Authors       []string
	AuthorSort    string
	Publisher     string
	PublishDate   time.Time
	Language      string
	Tags          []string
	Description   string
	ISBN          string
	Identifiers   map[string]string
	Series        string
	SeriesIndex   float64
}

// ParseFile parses an OPF file from disk
func ParseFile(path string) (*ParsedMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open OPF file: %w", err)
	}
	defer f.Close()

	return Parse(f)
}

// Parse parses OPF XML from a reader
func Parse(r io.Reader) (*ParsedMetadata, error) {
	var pkg Package
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&pkg); err != nil {
		return nil, fmt.Errorf("failed to parse OPF XML: %w", err)
	}

	return parseMetadata(&pkg.Metadata), nil
}

// ParseBytes parses OPF XML from bytes
func ParseBytes(data []byte) (*ParsedMetadata, error) {
	return Parse(strings.NewReader(string(data)))
}

// parseMetadata converts raw OPF metadata to our clean struct
func parseMetadata(m *Metadata) *ParsedMetadata {
	result := &ParsedMetadata{
		Title:       m.Title,
		Publisher:   m.Publisher,
		Language:    m.Language,
		Tags:        m.Subjects,
		Description: m.Description,
		Identifiers: make(map[string]string),
	}

	// Parse authors
	for _, creator := range m.Creators {
		if creator.Role == "" || creator.Role == "aut" {
			result.Authors = append(result.Authors, creator.Name)
			if result.AuthorSort == "" && creator.FileAs != "" {
				result.AuthorSort = creator.FileAs
			}
		}
	}

	// Parse identifiers
	for _, id := range m.Identifiers {
		scheme := strings.ToLower(id.Scheme)
		if scheme == "" {
			scheme = strings.ToLower(id.ID)
		}
		result.Identifiers[scheme] = id.Value

		// Extract ISBN specifically
		if scheme == "isbn" {
			result.ISBN = id.Value
		}
	}

	// Parse date
	if m.Date != "" {
		// Try various date formats
		formats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05-07:00",
			"2006-01-02",
			"2006",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, m.Date); err == nil {
				result.PublishDate = t
				break
			}
		}
	}

	// Parse Calibre-specific meta tags
	for _, meta := range m.Meta {
		switch meta.Name {
		case "calibre:series":
			result.Series = meta.Content
		case "calibre:series_index":
			if idx, err := strconv.ParseFloat(meta.Content, 64); err == nil {
				result.SeriesIndex = idx
			}
		case "calibre:author_link_map":
			// Could parse author links if needed
		}
	}

	return result
}
