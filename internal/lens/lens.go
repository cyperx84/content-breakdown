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
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/cyperx84/content-breakdown/internal/llm"
	"github.com/cyperx84/content-breakdown/internal/schema"
)

// LensDefinition represents a lens configuration file.
type LensDefinition struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Purpose             string              `json:"purpose"`
	Questions           []string            `json:"questions"`
	RankingDimensions   []string            `json:"rankingDimensions"`
	IgnoreRules         []string            `json:"ignoreRules"`
	ArtifactRules       map[string][]string `json:"artifactRules"`
	ProjectContextHints []string            `json:"projectContextHints"`
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
	RelevanceScore       float64             `json:"relevanceScore"`
	Rationale            string              `json:"rationale"`
	RankedIdeas          []schema.RankedIdea `json:"rankedIdeas"`
	RecommendedArtifacts []string            `json:"recommendedArtifacts"`
	IgnoredItems         []string            `json:"ignoredItems"`
}

// Run performs the lens pass on an extraction record.
func Run(src *schema.SourceRecord, ext *schema.ExtractionRecord, lensDef *LensDefinition, opts Options) (*schema.LensResult, error) {
	prompt, err := buildPrompt(src, ext, lensDef)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "[lens] Running lens pass...")
	}

	response, err := llm.Call(prompt, llm.Options{
		LLMCmd:    opts.LLMCmd,
		Verbose:   opts.Verbose,
		LogPrefix: "[lens]",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	output, err := parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &schema.LensResult{
		SourceID:             src.ID,
		LensID:               lensDef.ID,
		LensName:             lensDef.Name,
		RelevanceScore:       output.RelevanceScore,
		Rationale:            strings.TrimSpace(output.Rationale),
		RankedIdeas:          output.RankedIdeas,
		RecommendedArtifacts: llm.UniqueStrings(output.RecommendedArtifacts, false),
		IgnoredItems:         llm.UniqueStrings(output.IgnoredItems, false),
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
		LensName            string
		LensPurpose         string
		Questions           []string
		RankingDimensions   []string
		IgnoreRules         []string
		ProjectContextHints []string
		ArtifactRules       map[string][]string
		Title               string
		Author              string
		Summary             string
		Tools               []string
		Workflows           []string
		Opportunities       []string
		Claims              []string
		Quotes              []string
	}{
		LensName:            lensDef.Name,
		LensPurpose:         lensDef.Purpose,
		Questions:           lensDef.Questions,
		RankingDimensions:   lensDef.RankingDimensions,
		IgnoreRules:         lensDef.IgnoreRules,
		ProjectContextHints: lensDef.ProjectContextHints,
		ArtifactRules:       lensDef.ArtifactRules,
		Title:               src.Title,
		Author:              src.Author,
		Summary:             ext.Summary,
		Tools:               ext.Tools,
		Workflows:           ext.Workflows,
		Opportunities:       ext.Opportunities,
		Claims:              ext.Claims,
		Quotes:              ext.Quotes,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func parseResponse(response string) (*LensOutput, error) {
	cleaned := llm.ExtractJSONObject(response)
	if cleaned == "" {
		cleaned = strings.TrimSpace(response)
	}

	var output LensOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nresponse was:\n%s", err, cleaned)
	}

	for i := range output.RankedIdeas {
		output.RankedIdeas[i].Title = strings.TrimSpace(output.RankedIdeas[i].Title)
		output.RankedIdeas[i].Rationale = strings.TrimSpace(output.RankedIdeas[i].Rationale)
		output.RankedIdeas[i].WhyItMatters = strings.TrimSpace(output.RankedIdeas[i].WhyItMatters)
		output.RankedIdeas[i].ImplementationFit = strings.TrimSpace(output.RankedIdeas[i].ImplementationFit)
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
