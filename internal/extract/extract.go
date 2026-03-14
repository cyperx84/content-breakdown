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
	"strings"
	"text/template"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

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
	Summary     string   `json:"summary"`
	Tools       []string `json:"tools"`
	Workflows   []string `json:"workflows"`
	Opportunities []string `json:"opportunities"`
	Claims      []string `json:"claims"`
	Quotes      []string `json:"quotes"`
}

// Run performs the extraction pass on a source record.
func Run(src *schema.SourceRecord, opts Options) (*schema.ExtractionRecord, error) {
	// Build the prompt
	prompt, err := buildPrompt(src)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "[extract] Running extraction pass...")
	}

	// Call LLM
	var response string
	if opts.LLMCmd != "" {
		response, err = callLLMCmd(prompt, opts.LLMCmd, opts.Verbose)
	} else {
		response, err = callLLMStdio(prompt, opts.Verbose)
	}
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	// Parse response
	output, err := parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Build extraction record
	return &schema.ExtractionRecord{
		SourceID:    src.ID,
		Summary:     output.Summary,
		Tools:       output.Tools,
		Workflows:   output.Workflows,
		Opportunities: output.Opportunities,
		Claims:      output.Claims,
		Quotes:      output.Quotes,
		Metadata: schema.ExtractionMetadata{
			GeneratedAt: time.Now(),
		},
	}, nil
}

func buildPrompt(src *schema.SourceRecord) (string, error) {
	tmpl, err := template.New("extract").Parse(ExtractionPromptTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Transcript string
		Title      string
		Author     string
	}{
		Transcript: src.Transcript,
		Title:      src.Title,
		Author:     src.Author,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func callLLMStdio(prompt string, verbose bool) (string, error) {
	if verbose {
		fmt.Fprintln(os.Stderr, "[extract] Awaiting LLM response on stdin...")
		fmt.Fprintln(os.Stderr, "[extract] Prompt written to stdout.")
	}

	// Write prompt to stdout
	fmt.Print(prompt)

	// Read response from stdin
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

	// Parse command string into parts
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
	// Clean up response (remove markdown code blocks if present)
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var output ExtractionOutput
	if err := json.Unmarshal([]byte(response), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nresponse was:\n%s", err, response)
	}

	return &output, nil
}
