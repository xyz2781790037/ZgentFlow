package docparser

import (
	"context"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// simpleFormats lists file extensions that Go can handle without the Python service.
var simpleFormats = map[string]bool{
	"md": true, "markdown": true,
	"txt": true, "text": true,
	"csv":  true,
	"json": true,
}

// IsSimpleFormat returns true if the file type can be handled by the Go SimpleFormatReader.
func IsSimpleFormat(fileType string) bool {
	return simpleFormats[strings.ToLower(strings.TrimPrefix(fileType, "."))]
}

// SimpleFormatReader handles simple file formats and images directly in Go,
// bypassing the Python docreader service.
type SimpleFormatReader struct{}

// Read reads simple format files and returns markdown.
func (b *SimpleFormatReader) Read(_ context.Context, req *types.ReadRequest) (*types.ReadResult, error) {
	ft := strings.ToLower(strings.TrimPrefix(req.FileType, "."))
	if ft == "" {
		ft = strings.TrimPrefix(strings.ToLower(filepath.Ext(req.FileName)), ".")
	}

	switch {
	case ft == "md" || ft == "markdown":
		return &types.ReadResult{MarkdownContent: string(req.FileContent)}, nil
	case ft == "txt" || ft == "text":
		return &types.ReadResult{MarkdownContent: string(req.FileContent)}, nil
	case ft == "csv":
		md, err := csvToMarkdown(req.FileContent)
		if err != nil {
			return nil, fmt.Errorf("csv conversion failed: %w", err)
		}
		return &types.ReadResult{MarkdownContent: md}, nil
	case ft == "json":
		md, err := jsonToMarkdown(req.FileContent)
		if err != nil {
			return nil, fmt.Errorf("json conversion failed: %w", err)
		}
		return &types.ReadResult{MarkdownContent: md}, nil
	default:
		return nil, fmt.Errorf("unsupported simple format: %s", ft)
	}
}

func csvToMarkdown(data []byte) (string, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// Header row
	header := records[0]
	sb.WriteString("| ")
	sb.WriteString(strings.Join(header, " | "))
	sb.WriteString(" |\n")

	// Separator
	sb.WriteString("|")
	for range header {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range records[1:] {
		sb.WriteString("| ")
		// Pad row if shorter than header
		cells := make([]string, len(header))
		for i := range cells {
			if i < len(row) {
				cells[i] = row[i]
			}
		}
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |\n")
	}

	return sb.String(), nil
}
