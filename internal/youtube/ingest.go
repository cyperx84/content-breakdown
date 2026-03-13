// Package youtube handles YouTube video ingestion via yt-dlp.
//
// It extracts transcripts (from subtitles/captions) and metadata,
// producing a normalized SourceRecord.
//
// Requires yt-dlp on PATH. Install with: brew install yt-dlp
package youtube

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cyperx84/content-breakdown/internal/schema"
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

	transcript, err := fetchTranscript(meta.ID)
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
	ID           string `json:"id"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	UploadDate   string `json:"upload_date"`
	Duration     int    `json:"duration"`
	URL          string `json:"webpage_url"`
}

func fetchMetadata(videoURL string) (*ytDlpMeta, error) {
	cmd := exec.Command("yt-dlp", "--dump-json", "--no-download", videoURL)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var meta ytDlpMeta
	if err := json.Unmarshal(out, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	return &meta, nil
}

func fetchTranscript(videoID string) (string, error) {
	// Try to get subtitles in JSON3 format for clean parsing
	cmd := exec.Command("yt-dlp",
		"--write-auto-sub",
		"--sub-lang", "en",
		"--sub-format", "json3",
		"--skip-download",
		"--no-playlist",
		"-o", "/dev/stdout",
		fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
	)
	// Note: for MVP, this is a simplified approach.
	// The actual implementation may need to write to a temp file
	// since yt-dlp writes subtitle files alongside the video.
	// This placeholder returns a structured error if subtitles
	// can't be fetched.
	_ = cmd

	return "", fmt.Errorf("transcript extraction: not yet implemented (needs temp-file-based yt-dlp subtitle workflow)")
}

func buildSourceRecord(meta *ytDlpMeta, transcript string) *schema.SourceRecord {
	return &schema.SourceRecord{
		ID:           fmt.Sprintf("yt_%s", meta.ID),
		Type:         "youtube",
		CanonicalURL: meta.URL,
		Title:        meta.Title,
		Author:       meta.Channel,
		Transcript:   transcript,
		Metadata: schema.SourceMetadata{
			Extractor: "yt-dlp",
			VideoID:   meta.ID,
		},
	}
}

// Slug returns a filesystem-safe slug for the video.
func Slug(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' {
			return r
		}
		return -1
	}, s)
	s = strings.ReplaceAll(s, " ", "-")
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}
	return s
}
