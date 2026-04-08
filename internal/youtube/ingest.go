// Package youtube handles YouTube video ingestion via yt-dlp.
//
// It extracts transcripts (from subtitles/captions) and metadata,
// producing a normalized SourceRecord.
//
// Requires yt-dlp on PATH. Install with: brew install yt-dlp
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

const (
	metadataTimeout   = 60 * time.Second
	transcriptTimeout = 180 * time.Second
)

// Ingest fetches transcript and metadata from a YouTube URL.
// Returns a SourceRecord ready for the extraction pipeline.
func Ingest(videoURL string) (*schema.SourceRecord, error) {
	if err := checkYTDLP(); err != nil {
		return nil, err
	}

	meta, err := fetchMetadata(videoURL)
	if err != nil {
		return nil, fmt.Errorf("metadata fetch: %w", err)
	}

	transcript, err := fetchTranscript(videoURL, meta.ID)
	if err != nil {
		return nil, fmt.Errorf("transcript fetch: %w", err)
	}

	return buildSourceRecord(meta, transcript), nil
}

func checkYTDLP() error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found. Install with: brew install yt-dlp")
	}
	return nil
}

// ytDlpMeta is the subset of yt-dlp --dump-json we care about.
type ytDlpMeta struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Channel    string `json:"channel"`
	UploadDate string `json:"upload_date"`
	Duration   int    `json:"duration"`
	URL        string `json:"webpage_url"`
}

func fetchMetadata(videoURL string) (*ytDlpMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", "--dump-json", "--no-download", videoURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("yt-dlp metadata timed out after %s", metadataTimeout)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w\nstderr: %s", err, stderr.String())
	}

	var meta ytDlpMeta
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	return &meta, nil
}

func fetchTranscript(videoURL, videoID string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "breakdown-yt-"+videoID)
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), transcriptTimeout)
	defer cancel()

	basePath := filepath.Join(tmpDir, videoID)
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--write-subs",
		"--write-auto-subs",
		"--sub-lang", "en",
		"--skip-download",
		"--no-playlist",
		"-o", basePath,
		videoURL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("yt-dlp transcript timed out after %s", transcriptTimeout)
		}
		return "", fmt.Errorf("download subtitles: %w\noutput: %s", err, string(output))
	}

	// Find the subtitle file (could be .vtt, .srv3, .json3, etc.)
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*.vtt"))
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(tmpDir, "*.srv3"))
	}
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(tmpDir, "*.json3"))
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no subtitle files found (video may not have captions)")
	}

	subFile := files[0]
	content, err := os.ReadFile(subFile)
	if err != nil {
		return "", fmt.Errorf("read subtitle file: %w", err)
	}

	ext := filepath.Ext(subFile)
	var transcript string
	switch ext {
	case ".vtt":
		transcript = parseVTT(string(content))
	case ".json3":
		transcript = parseJSON3(content)
	case ".srv3":
		transcript = parseSRV3(string(content))
	default:
		transcript = stripTimestamps(string(content))
	}

	return transcript, nil
}

func parseVTT(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Skip WEBVTT header, timestamps, and empty lines
		if line == "" || strings.HasPrefix(line, "WEBVTT") ||
			strings.HasPrefix(line, "Kind:") || strings.HasPrefix(line, "Language:") ||
			strings.Contains(line, "-->") || strings.HasPrefix(line, "NOTE") {
			continue
		}
		// Skip timestamp position cues (e.g., "position:50%,start")
		if strings.Contains(line, "position:") {
			continue
		}
		// Skip numeric-only lines (cue identifiers)
		if isNumeric(line) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, " ")
}

func parseJSON3(content []byte) string {
	var data struct {
		Events []struct {
			Segs []struct {
				Utf8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}

	if err := json.Unmarshal(content, &data); err != nil {
		return ""
	}

	var lines []string
	for _, event := range data.Events {
		for _, seg := range event.Segs {
			if seg.Utf8 != "" && seg.Utf8 != "\n" {
				lines = append(lines, seg.Utf8)
			}
		}
	}
	return strings.Join(lines, " ")
}

func parseSRV3(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "<") || strings.HasSuffix(line, ">") {
			continue
		}
		line = strings.ReplaceAll(line, "&amp;", "&")
		line = strings.ReplaceAll(line, "&lt;", "<")
		line = strings.ReplaceAll(line, "&gt;", ">")
		line = strings.ReplaceAll(line, "&apos;", "'")
		line = strings.ReplaceAll(line, "&quot;", "\"")
		lines = append(lines, line)
	}
	return strings.Join(lines, " ")
}

func stripTimestamps(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "-->") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func buildSourceRecord(meta *ytDlpMeta, transcript string) *schema.SourceRecord {
	now := time.Now()
	var publishedAt string
	if meta.UploadDate != "" {
		if t, err := time.Parse("20060102", meta.UploadDate); err == nil {
			publishedAt = t.Format("2006-01-02")
		}
	}

	var duration string
	if meta.Duration > 0 {
		duration = fmt.Sprintf("%dm%ds", meta.Duration/60, meta.Duration%60)
	}

	return &schema.SourceRecord{
		ID:           fmt.Sprintf("yt_%s", meta.ID),
		Type:         "youtube",
		CanonicalURL: meta.URL,
		Title:        meta.Title,
		Author:       meta.Channel,
		PublishedAt:  publishedAt,
		Duration:     duration,
		Transcript:   transcript,
		Metadata: schema.SourceMetadata{
			ExtractedAt: now,
			Extractor:   "yt-dlp",
			VideoID:     meta.ID,
		},
	}
}
