package emit

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

const (
	FormatVault   = "vault"
	FormatSummary = "summary"
	FormatPRD     = "prd"
	FormatTasks   = "tasks"
)

func SupportedFormats() []string {
	return []string{FormatVault, FormatSummary, FormatPRD, FormatTasks}
}

func Render(format string, src *schema.SourceRecord, ext *schema.ExtractionRecord, lens *schema.LensResult) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", FormatVault:
		return VaultNote(src, ext, lens), nil
	case FormatSummary:
		return Summary(src, ext, lens), nil
	case FormatPRD:
		return PRDSeed(src, ext, lens), nil
	case FormatTasks:
		return TaskList(src, ext, lens), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s (supported: %s)", format, strings.Join(SupportedFormats(), ", "))
	}
}

func Summary(src *schema.SourceRecord, ext *schema.ExtractionRecord, lens *schema.LensResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Executive Summary\n\n", src.Title)
	fmt.Fprintf(&b, "- **Source:** %s\n", src.CanonicalURL)
	fmt.Fprintf(&b, "- **Lens:** %s\n", lens.LensID)
	fmt.Fprintf(&b, "- **Relevance:** %.2f\n\n", lens.RelevanceScore)
	b.WriteString("## Summary\n\n")
	b.WriteString(strings.TrimSpace(ext.Summary))
	b.WriteString("\n\n")
	if len(lens.RankedIdeas) > 0 {
		b.WriteString("## Top Ideas\n\n")
		for i, idea := range lens.RankedIdeas {
			fmt.Fprintf(&b, "%d. **%s** (%.1f) — %s\n", i+1, idea.Title, idea.Score, firstNonEmpty(idea.WhyItMatters, idea.Rationale))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func PRDSeed(src *schema.SourceRecord, ext *schema.ExtractionRecord, lens *schema.LensResult) string {
	var b strings.Builder
	now := time.Now().Format("2006-01-02")
	fmt.Fprintf(&b, "# PRD Seed — %s\n\n", src.Title)
	fmt.Fprintf(&b, "**Date:** %s  \n", now)
	fmt.Fprintf(&b, "**Source:** %s  \n", src.CanonicalURL)
	fmt.Fprintf(&b, "**Lens:** %s  \n", lens.LensID)
	fmt.Fprintf(&b, "**Relevance:** %.2f\n\n", lens.RelevanceScore)
	b.WriteString("## Problem\n\n")
	b.WriteString(strings.TrimSpace(lens.Rationale))
	b.WriteString("\n\n")
	b.WriteString("## Opportunity\n\n")
	for _, item := range topIdeas(lens.RankedIdeas, 3) {
		fmt.Fprintf(&b, "- **%s** — %s\n", item.Title, firstNonEmpty(item.WhyItMatters, item.Rationale))
	}
	b.WriteString("\n## Proposed Scope\n\n")
	for _, item := range topIdeas(lens.RankedIdeas, 5) {
		fmt.Fprintf(&b, "- %s\n", item.Title)
	}
	b.WriteString("\n## Inputs from Source\n\n")
	for _, w := range ext.Workflows {
		fmt.Fprintf(&b, "- Workflow: %s\n", w)
	}
	for _, t := range ext.Tools {
		fmt.Fprintf(&b, "- Tool/Pattern: %s\n", t)
	}
	b.WriteString("\n## Open Questions\n\n")
	for _, q := range seedQuestions(lens, ext) {
		fmt.Fprintf(&b, "- %s\n", q)
	}
	b.WriteString("\n## Next Steps\n\n")
	for _, a := range lens.RecommendedArtifacts {
		fmt.Fprintf(&b, "- %s\n", a)
	}
	return b.String()
}

func TaskList(src *schema.SourceRecord, ext *schema.ExtractionRecord, lens *schema.LensResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Task List — %s\n\n", src.Title)
	fmt.Fprintf(&b, "Source: %s\n\n", src.CanonicalURL)
	b.WriteString("## Build Tasks\n\n")
	for i, item := range topIdeas(lens.RankedIdeas, 6) {
		fmt.Fprintf(&b, "- [ ] %s\n", item.Title)
		if item.ImplementationFit != "" {
			fmt.Fprintf(&b, "  - Fit: %s\n", item.ImplementationFit)
		}
		if item.WhyItMatters != "" {
			fmt.Fprintf(&b, "  - Why: %s\n", item.WhyItMatters)
		}
		if i == 2 {
			b.WriteString("\n## Follow-ups\n\n")
		}
	}
	if len(lens.RecommendedArtifacts) > 0 {
		b.WriteString("\n## Artifact Tasks\n\n")
		for _, a := range lens.RecommendedArtifacts {
			fmt.Fprintf(&b, "- [ ] %s\n", a)
		}
	}
	return b.String()
}

func topIdeas(ideas []schema.RankedIdea, limit int) []schema.RankedIdea {
	if len(ideas) == 0 || limit <= 0 {
		return nil
	}
	cloned := append([]schema.RankedIdea(nil), ideas...)
	sort.SliceStable(cloned, func(i, j int) bool { return cloned[i].Score > cloned[j].Score })
	if len(cloned) > limit {
		cloned = cloned[:limit]
	}
	return cloned
}

func seedQuestions(lens *schema.LensResult, ext *schema.ExtractionRecord) []string {
	questions := []string{
		"Which idea should be prototyped first?",
		"What existing OpenClaw or adjacent infrastructure can this reuse?",
		"What is the smallest useful slice to validate demand?",
	}
	if len(ext.Claims) > 0 {
		questions = append(questions, "Which source claims need validation before implementation?")
	}
	if lens.RelevanceScore < 0.7 {
		questions = append(questions, "Is this differentiated enough to justify building now?")
	}
	return questions
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
