// Package extract performs the LLM extraction pass on source records.
//
// It takes a SourceRecord (with transcript) and uses an LLM to extract
// structured findings: summary, tools, workflows, opportunities, claims, quotes.
//
// The LLM call is stdin/stdout based for keyless operation:
// - Prompt is written to stdout
// - LLM response is read from stdin
// - Or use --llm-cmd to pipe through an external command
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

const maxTranscriptChars = 80000

// Options configures the extraction pass.
type Options struct {
	// LLMCmd is an external command to pipe prompts through.
	// If empty, uses stdin/stdout directly.
	LLMCmd string

	// Verbose prints progress to stderr.
	Verbose bool
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

	chunks := chunkTranscript(transcript, maxTranscriptChars)
	outputs := make([]*ExtractionOutput, 0, len(chunks))

	for i, chunk := range chunks {
		if opts.Verbose && len(chunks) > 1 {
			fmt.Fprintf(os.Stderr, "[extract] Chunk %d/%d...\n", i+1, len(chunks))
		}

		prompt, err := buildPrompt(src, chunk)
		if err != nil {
			return nil, fmt.Errorf("build prompt: %w", err)
		}

		response, err := callLLM(prompt, opts)
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

func callLLM(prompt string, opts Options) (string, error) {
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "[extract] Running extraction pass...")
	}

	if opts.LLMCmd != "" {
		return callLLMCmd(prompt, opts.LLMCmd, opts.Verbose)
	}
	return callLLMStdio(prompt, opts.Verbose)
}

func callLLMStdio(prompt string, verbose bool) (string, error) {
	if verbose {
		fmt.Fprintln(os.Stderr, "[extract] Awaiting LLM response on stdin...")
		fmt.Fprintln(os.Stderr, "[extract] Prompt written to stdout.")
	}

	fmt.Print(prompt)

	var response bytes.Buffer
	if _, err := io.Copy(&response, os.Stdin); err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}

	return response.String(), nil
}

func callLLMCmd(prompt, cmdStr string, verbose bool) (string, error) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[extract] Running LLM command: %s\n", cmdStr)
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty LLM command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func parseResponse(response string) (*ExtractionOutput, error) {
	cleaned := extractJSONObject(response)
	if cleaned == "" {
		cleaned = strings.TrimSpace(response)
	}

	var output ExtractionOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nresponse was:\n%s", err, cleaned)
	}

	output.Summary = strings.TrimSpace(output.Summary)
	output.Tools = uniqueStrings(output.Tools)
	output.Workflows = uniqueStrings(output.Workflows)
	output.Opportunities = uniqueStrings(output.Opportunities)
	output.Claims = uniqueStrings(output.Claims)
	output.Quotes = uniqueStrings(output.Quotes)

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
		Tools:         uniqueStrings(tools),
		Workflows:     uniqueStrings(workflows),
		Opportunities: uniqueStrings(opportunities),
		Claims:        uniqueStrings(claims),
		Quotes:        uniqueStrings(quotes),
	}
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func extractJSONObject(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(response[start : end+1])
}
