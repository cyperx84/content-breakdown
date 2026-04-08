// Package lens contains prompts for the lens LLM pass.
package lens

// LensPromptTemplate is the prompt sent to the LLM to apply a lens
// to extracted findings and produce ranked, actionable insights.
const LensPromptTemplate = `You are an expert analyst applying a specific lens to extracted content findings.

LENS: {{.LensName}}
PURPOSE: {{.LensPurpose}}

LENS QUESTIONS:
{{range .Questions}}- {{.}}
{{end}}

RANKING DIMENSIONS:
{{range .RankingDimensions}}- {{.}}
{{end}}

IGNORE RULES:
{{range .IgnoreRules}}- {{.}}
{{end}}
{{if .ProjectContextHints}}
PROJECT CONTEXT (use these to bias what counts as relevant):
{{range .ProjectContextHints}}- {{.}}
{{end}}{{end}}
{{if .ArtifactRules}}
ARTIFACT RULES (recommend artifacts according to the bucket the relevance score lands in):
{{range $bucket, $items := .ArtifactRules}}- {{$bucket}}: {{range $items}}{{.}}; {{end}}
{{end}}{{end}}
EXTRACTED FINDINGS:
Source: {{.Title}} by {{.Author}}

Summary:
{{.Summary}}

Tools:
{{range .Tools}}- {{.}}
{{end}}

Workflows:
{{range .Workflows}}- {{.}}
{{end}}

Opportunities:
{{range .Opportunities}}- {{.}}
{{end}}

{{if .Claims}}Claims:
{{range .Claims}}- {{.}}
{{end}}
{{end}}

{{if .Quotes}}Notable Quotes:
{{range .Quotes}}- "{{.}}"
{{end}}
{{end}}

---

Apply this lens to the findings. Return ONLY valid JSON with this structure:
{
  "relevanceScore": 0.0-1.0,
  "rationale": "Brief explanation of why this content matters (or doesn't) for this lens",
  "rankedIdeas": [
    {
      "title": "Idea title",
      "rationale": "Why this idea is important",
      "whyItMatters": "Concrete impact this could have",
      "implementationFit": "How well this fits current capabilities",
      "score": 0.0-1.0
    }
  ],
  "recommendedArtifacts": ["list of specific next steps or artifacts to create"],
  "ignoredItems": ["items from the extraction that should be ignored per lens rules"]
}

Rules:
- Be ruthless in filtering: apply ignore rules strictly
- relevanceScore should reflect overall value for this specific lens
- Only include ideas that score above 0.5
- Each ranked idea should be actionable and specific
- recommendedArtifacts should be concrete (e.g., "Create PRD for X", "Test Y pattern")
- If nothing is relevant, set relevanceScore to 0.1 and explain in rationale
`
