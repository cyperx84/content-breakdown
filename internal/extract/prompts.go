// Package extract contains prompts for the extraction LLM pass.
package extract

// ExtractionPromptTemplate is the prompt sent to the LLM to extract
// structured findings from a source transcript.
const ExtractionPromptTemplate = `You are an expert analyst extracting structured findings from content.

Analyze the following transcript and extract key information. Return ONLY valid JSON with no additional text.

TRANSCRIPT:
{{.Transcript}}

METADATA:
- Title: {{.Title}}
- Author: {{.Author}}

Return a JSON object with this exact structure:
{
  "summary": "2-3 sentence summary of the main content and key takeaways",
  "tools": ["list of tools, technologies, or frameworks mentioned"],
  "workflows": ["list of workflows, processes, or patterns described"],
  "opportunities": ["list of actionable opportunities or ideas presented"],
  "claims": ["list of specific claims or assertions made (with speaker attribution if possible)"],
  "quotes": ["list of notable quotes worth preserving"]
}

Rules:
- Be specific and concrete (avoid vague summaries)
- Include proper nouns and technical terms exactly as mentioned
- Each item in arrays should be a single, complete thought
- If something is not mentioned, use an empty array []
- Focus on actionable and implementable ideas
- Capture the "how" not just the "what"
`
