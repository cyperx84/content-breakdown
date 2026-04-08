package source

import (
	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/youtube"
)

// YouTubeAdapter wraps the existing youtube package as a source.Adapter.
type YouTubeAdapter struct{}

func init() {
	RegisterWithPriority(&YouTubeAdapter{}, PriorityHigh)
}

func (y *YouTubeAdapter) Detect(input string) bool {
	return IsYouTubeURL(input)
}

func (y *YouTubeAdapter) Ingest(input string) (*schema.SourceRecord, error) {
	return youtube.Ingest(input)
}
