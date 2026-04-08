// Package extract performs the LLM extraction pass on source records.
//
// It takes a SourceRecord (with transcript) and uses an LLM to extract
// structured findings: summary, tools, workflows, opportunities, claims, quotes.
//
// The LLM call is stdin/stdout based for keyless operation:
// - Prompt is written to stdout
// - LLM response is read from stdin
// - Or use LLMCmd to pipe through an external command
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/cyperx84/content-breakdown/internal/llm"
	"github.com/cyperx84/content-breakdown/internal/schema"
)

// DefaultMaxTranscriptChars is used when Options.MaxTranscriptChars is zero.
const DefaultMaxTranscriptChars = 80000

// Options configures the extraction pass.
type Options struct {
	// LLMCmd is an external command to pipe prompts through.
	// If empty, uses stdin/stdout directly.
	LLMCmd string

	// Verbose prints progress to stderr.
	Verbose bool

	// MaxTranscriptChars caps per-chunk transcript length before calling the LLM.
	// Zero uses DefaultMaxTranscriptChars.
	MaxTranscriptChars int
}

// ExtractionOutput is the LLM response structure.
type ExtractionOutput struct {
	Summary       string   `json:"summary"`
	Tools         []string `json:"tools"`
	Workflows     []string `json:"workflows"`
	Opportunities []string `json:"opportunities"`
	Claims        []string `json:"claims"`
	Quotes        []string `json:"quotes"`
}

// Run performs the extraction pass on a source record.
func Run(src *schema.SourceRecord, opts Options) (*schema.ExtractionRecord, error) {
	transcript := strings.TrimSpace(src.Transcript)
	if transcript == "" {
		return nil, fmt.Errorf("empty transcript")
	}

	maxChars := opts.MaxTranscriptChars
	if maxChars <= 0 {
		maxChars = DefaultMaxTranscriptChars
	}

	chunks := chunkTranscript(transcript, maxChars)

	if opts.Verbose {
		if len(chunks) > 1 {
			fmt.Fprintf(os.Stderr, "[extract] Running extraction pass (%d chunks)...\n", len(chunks))
		} else {
			fmt.Fprintln(os.Stderr, "[extract] Running extraction pass...")
		}
	}

	outputs := make([]*ExtractionOutput, 0, len(chunks))
	llmOpts := llm.Options{
		LLMCmd:    opts.LLMCmd,
		Verbose:   opts.Verbose,
		LogPrefix: "[extract]",
	}

	for i, chunk := range chunks {
		if opts.Verbose && len(chunks) > 1 {
			fmt.Fprintf(os.Stderr, "[extract] Chunk %d/%d...\n", i+1, len(chunks))
		}

		prompt, err := buildPrompt(src, chunk)
		if err != nil {
			return nil, fmt.Errorf("build prompt: %w", err)
		}

		response, err := llm.Call(prompt, llmOpts)
		if err != nil {
			return nil, fmt.Errorf("LLM call: %w", err)
		}

		output, err := parseResponse(response)
		if err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		outputs = append(outputs, output)
	}

	merged := mergeOutputs(outputs)

	return &schema.ExtractionRecord{
		SourceID:      src.ID,
		Summary:       merged.Summary,
		Tools:         merged.Tools,
		Workflows:     merged.Workflows,
		Opportunities: merged.Opportunities,
		Claims:        merged.Claims,
		Quotes:        merged.Quotes,
		Metadata: schema.ExtractionMetadata{
			GeneratedAt: time.Now(),
		},
	}, nil
}

func buildPrompt(src *schema.SourceRecord, transcript string) (string, error) {
	tmpl, err := template.New("extract").Parse(ExtractionPromptTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Transcript string
		Title      string
		Author     string
	}{
		Transcript: transcript,
		Title:      src.Title,
		Author:     src.Author,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func parseResponse(response string) (*ExtractionOutput, error) {
	cleaned := llm.ExtractJSONObject(response)
	if cleaned == "" {
		cleaned = strings.TrimSpace(response)
	}

	var output ExtractionOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nresponse was:\n%s", err, cleaned)
	}

	output.Summary = strings.TrimSpace(output.Summary)
	output.Tools = llm.UniqueStrings(output.Tools, true)
	output.Workflows = llm.UniqueStrings(output.Workflows, true)
	output.Opportunities = llm.UniqueStrings(output.Opportunities, true)
	output.Claims = llm.UniqueStrings(output.Claims, true)
	output.Quotes = llm.UniqueStrings(output.Quotes, true)

	return &output, nil
}

func chunkTranscript(transcript string, maxChars int) []string {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return nil
	}
	if len(transcript) <= maxChars {
		return []string{transcript}
	}

	words := strings.Fields(transcript)
	if len(words) == 0 {
		return []string{transcript}
	}

	chunks := make([]string, 0)
	var current strings.Builder
	for _, word := range words {
		if current.Len() > 0 && current.Len()+1+len(word) > maxChars {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func mergeOutputs(outputs []*ExtractionOutput) *ExtractionOutput {
	if len(outputs) == 0 {
		return &ExtractionOutput{}
	}
	if len(outputs) == 1 {
		return outputs[0]
	}

	summaries := make([]string, 0, len(outputs))
	var tools, workflows, opportunities, claims, quotes []string
	for _, out := range outputs {
		if out == nil {
			continue
		}
		if s := strings.TrimSpace(out.Summary); s != "" {
			summaries = append(summaries, s)
		}
		tools = append(tools, out.Tools...)
		workflows = append(workflows, out.Workflows...)
		opportunities = append(opportunities, out.Opportunities...)
		claims = append(claims, out.Claims...)
		quotes = append(quotes, out.Quotes...)
	}

	return &ExtractionOutput{
		Summary:       strings.Join(summaries, "\n\n"),
		Tools:         llm.UniqueStrings(tools, true),
		Workflows:     llm.UniqueStrings(workflows, true),
		Opportunities: llm.UniqueStrings(opportunities, true),
		Claims:        llm.UniqueStrings(claims, true),
		Quotes:        llm.UniqueStrings(quotes, true),
	}
}
