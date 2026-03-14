package source

import (
	"strings"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/youtube"
)

// YouTubeAdapter wraps the existing youtube package as a source.Adapter.
type YouTubeAdapter struct{}

func init() {
	// Register first so YouTube is checked before the generic webpage adapter.
	registry = append([]Adapter{&YouTubeAdapter{}}, registry...)
}

func (y *YouTubeAdapter) Detect(input string) bool {
	return strings.Contains(input, "youtube.com/watch") ||
		strings.Contains(input, "youtu.be/") ||
		strings.Contains(input, "youtube.com/shorts")
}

func (y *YouTubeAdapter) Ingest(input string) (*schema.SourceRecord, error) {
	return youtube.Ingest(input)
}
