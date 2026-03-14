// Package lens performs the lens LLM pass on extraction records.
//
// It takes an ExtractionRecord and a Lens definition, then uses an LLM
// to apply the lens perspective and produce ranked, actionable insights.
//
// The LLM call is stdin/stdout based for keyless operation.
package lens

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

// LensDefinition represents a lens configuration file.
type LensDefinition struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Purpose           string   `json:"purpose"`
	Questions         []string `json:"questions"`
	RankingDimensions []string `json:"rankingDimensions"`
	IgnoreRules       []string `json:"ignoreRules"`
	ArtifactRules     map[string][]string `json:"artifactRules"`
	ProjectContextHints []string `json:"projectContextHints"`
}

// Options configures the lens pass.
type Options struct {
	// LLMCmd is an external command to pipe prompts through.
	LLMCmd string

	// Verbose prints progress to stderr.
	Verbose bool
}

// LensOutput is the LLM response structure.
type LensOutput struct {
	RelevanceScore       float64 `json:"relevanceScore"`
	Rationale            string  `json:"rationale"`
	RankedIdeas          []schema.RankedIdea `json:"rankedIdeas"`
	RecommendedArtifacts []string `json:"recommendedArtifacts"`
	IgnoredItems         []string `json:"ignoredItems"`
}

// Run performs the lens pass on an extraction record.
func Run(src *schema.SourceRecord, ext *schema.ExtractionRecord, lensDef *LensDefinition, opts Options) (*schema.LensResult, error) {
	// Build the prompt
	prompt, err := buildPrompt(src, ext, lensDef)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "[lens] Running lens pass...")
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

	// Build lens result
	return &schema.LensResult{
		SourceID:             src.ID,
		LensID:               lensDef.ID,
		RelevanceScore:       output.RelevanceScore,
		Rationale:            output.Rationale,
		RankedIdeas:          output.RankedIdeas,
		RecommendedArtifacts: output.RecommendedArtifacts,
		IgnoredItems:         output.IgnoredItems,
		Metadata: schema.LensMetadata{
			GeneratedAt: time.Now(),
		},
	}, nil
}

func buildPrompt(src *schema.SourceRecord, ext *schema.ExtractionRecord, lensDef *LensDefinition) (string, error) {
	tmpl, err := template.New("lens").Parse(LensPromptTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		LensName         string
		LensPurpose      string
		Questions        []string
		RankingDimensions []string
		IgnoreRules      []string
		Title            string
		Author           string
		Summary          string
		Tools            []string
		Workflows        []string
		Opportunities    []string
		Claims           []string
		Quotes           []string
	}{
		LensName:         lensDef.Name,
		LensPurpose:      lensDef.Purpose,
		Questions:        lensDef.Questions,
		RankingDimensions: lensDef.RankingDimensions,
		IgnoreRules:      lensDef.IgnoreRules,
		Title:            src.Title,
		Author:           src.Author,
		Summary:          ext.Summary,
		Tools:            ext.Tools,
		Workflows:        ext.Workflows,
		Opportunities:    ext.Opportunities,
		Claims:           ext.Claims,
		Quotes:           ext.Quotes,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func callLLMStdio(prompt string, verbose bool) (string, error) {
	if verbose {
		fmt.Fprintln(os.Stderr, "[lens] Awaiting LLM response on stdin...")
		fmt.Fprintln(os.Stderr, "[lens] Prompt written to stdout.")
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
		fmt.Fprintf(os.Stderr, "[lens] Running LLM command: %s\n", cmdStr)
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

func parseResponse(response string) (*LensOutput, error) {
	// Clean up response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var output LensOutput
	if err := json.Unmarshal([]byte(response), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nresponse was:\n%s", err, response)
	}

	return &output, nil
}

// LoadLens reads a lens definition from a JSON file.
func LoadLens(path string) (*LensDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lens file: %w", err)
	}

	var lens LensDefinition
	if err := json.Unmarshal(data, &lens); err != nil {
		return nil, fmt.Errorf("parse lens file: %w", err)
	}

	return &lens, nil
}
