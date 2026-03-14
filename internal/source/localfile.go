package source

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

// LocalFileAdapter ingests plain text, markdown, and PDF files.
type LocalFileAdapter struct{}

func init() {
	Register(&LocalFileAdapter{})
}

func (l *LocalFileAdapter) Detect(input string) bool {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(input))
	switch ext {
	case ".md", ".txt", ".text", ".markdown", ".pdf":
		return true
	}
	// Accept extensionless paths that exist on disk
	if _, err := os.Stat(input); err == nil {
		return true
	}
	return false
}

func (l *LocalFileAdapter) Ingest(path string) (*schema.SourceRecord, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input is a directory, not a file: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var text string
	if ext == ".pdf" {
		text, err = extractPDF(path)
	} else {
		var raw []byte
		raw, err = os.ReadFile(path)
		text = string(raw)
	}
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	text = strings.TrimSpace(text)
	if len(text) < 50 {
		return nil, fmt.Errorf("file content too short: %s", path)
	}

	title := inferTitle(path, text)
	now := time.Now()
	abs, _ := filepath.Abs(path)

	return &schema.SourceRecord{
		ID:           fmt.Sprintf("file_%s", slugify(filepath.Base(path), 32)),
		Type:         localFileType(ext),
		CanonicalURL: abs,
		Title:        title,
		Author:       "",
		Transcript:   text,
		Metadata: schema.SourceMetadata{
			ExtractedAt: now,
			Extractor:   "breakdown-localfile",
		},
	}, nil
}

func extractPDF(path string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not found — install poppler: brew install poppler")
	}
	out, err := exec.Command("pdftotext", "-layout", path, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}
	return string(out), nil
}

func inferTitle(path, text string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Capitalise first letter
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	// If markdown, try to extract H1
	lines := strings.SplitN(text, "\n", 20)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return name
}

func localFileType(ext string) string {
	switch ext {
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	default:
		return "text"
	}
}
