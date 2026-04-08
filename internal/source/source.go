// Package source provides a unified adapter interface for ingesting
// source material from different input types.
//
// Each adapter takes a raw input (URL, file path, stdin) and produces
// a normalized SourceRecord ready for the extraction pipeline.
package source

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

// Adapter is implemented by each source type.
type Adapter interface {
	// Detect returns true if this adapter can handle the given input.
	Detect(input string) bool
	// Ingest fetches/reads the input and returns a SourceRecord.
	Ingest(input string) (*schema.SourceRecord, error)
}

// Priority values control the order in which adapters are consulted by
// Ingest. Higher values run first. The built-in adapters use:
//
//	PriorityHigh   = YouTube (must check before the generic webpage adapter)
//	PriorityMedium = Webpage
//	PriorityLow    = LocalFile (catch-all)
const (
	PriorityHigh   = 100
	PriorityMedium = 50
	PriorityLow    = 10
)

type registered struct {
	priority int
	seq      int // tie-breaker preserving registration order
	adapter  Adapter
}

var (
	registryMu sync.RWMutex
	registry   []registered
	nextSeq    int
)

// Register adds an adapter at PriorityMedium. Kept for backwards compatibility.
func Register(a Adapter) {
	RegisterWithPriority(a, PriorityMedium)
}

// RegisterWithPriority adds an adapter to the registry at the given priority.
// Higher priorities are consulted first. Within the same priority, registration
// order is preserved.
func RegisterWithPriority(a Adapter, priority int) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, registered{priority: priority, seq: nextSeq, adapter: a})
	nextSeq++
	sort.SliceStable(registry, func(i, j int) bool {
		if registry[i].priority != registry[j].priority {
			return registry[i].priority > registry[j].priority
		}
		return registry[i].seq < registry[j].seq
	})
}

// Adapters returns a copy of the current adapter list in resolution order.
// Exposed for tests.
func Adapters() []Adapter {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Adapter, len(registry))
	for i, r := range registry {
		out[i] = r.adapter
	}
	return out
}

// Ingest routes the input to the correct adapter and returns a SourceRecord.
func Ingest(input string) (*schema.SourceRecord, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}
	for _, a := range Adapters() {
		if a.Detect(input) {
			return a.Ingest(input)
		}
	}
	return nil, fmt.Errorf("no adapter found for input: %s", input)
}

// IsYouTubeURL reports whether the input looks like a YouTube URL.
// Exported so multiple adapters can share the same detection.
func IsYouTubeURL(u string) bool {
	return strings.Contains(u, "youtube.com/watch") ||
		strings.Contains(u, "youtu.be/") ||
		strings.Contains(u, "youtube.com/shorts")
}
