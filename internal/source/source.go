// Package source provides a unified adapter interface for ingesting
// source material from different input types.
//
// Each adapter takes a raw input (URL, file path, stdin) and produces
// a normalized SourceRecord ready for the extraction pipeline.
package source

import (
	"fmt"
	"strings"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

// Adapter is implemented by each source type.
type Adapter interface {
	// Detect returns true if this adapter can handle the given input.
	Detect(input string) bool
	// Ingest fetches/reads the input and returns a SourceRecord.
	Ingest(input string) (*schema.SourceRecord, error)
}

// registry is the ordered list of registered adapters.
var registry []Adapter

// Register adds an adapter to the registry.
func Register(a Adapter) {
	registry = append(registry, a)
}

// Ingest routes the input to the correct adapter and returns a SourceRecord.
func Ingest(input string) (*schema.SourceRecord, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}
	for _, a := range registry {
		if a.Detect(input) {
			return a.Ingest(input)
		}
	}
	return nil, fmt.Errorf("no adapter found for input: %s", input)
}

// DetectedType returns the adapter name for the input, or "" if none match.
func DetectedType(input string) string {
	for _, a := range registry {
		if a.Detect(input) {
			return fmt.Sprintf("%T", a)
		}
	}
	return ""
}
