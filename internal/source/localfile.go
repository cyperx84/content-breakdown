package source

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/slug"
)

// LocalFileAdapter ingests plain text, markdown, and PDF files.
type LocalFileAdapter struct{}

// MinLocalFileChars is the minimum content length accepted from a local file.
// Files shorter than this are rejected as likely empty / placeholders.
var MinLocalFileChars = 20

const pdfExtractTimeout = 60 * time.Second

func init() {
	RegisterWithPriority(&LocalFileAdapter{}, PriorityLow)
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
	if len(text) < MinLocalFileChars {
		return nil, fmt.Errorf("file content too short (<%d chars): %s", MinLocalFileChars, path)
	}

	title := inferTitle(path, text)
	now := time.Now()
	abs, _ := filepath.Abs(path)

	return &schema.SourceRecord{
		ID:           fmt.Sprintf("file_%s", slug.Make(filepath.Base(path), 32)),
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
	ctx, cancel := context.WithTimeout(context.Background(), pdfExtractTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pdftotext", "-layout", path, "-").Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("pdftotext timed out after %s on %s", pdfExtractTimeout, path)
		}
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
