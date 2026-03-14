package source

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

// WebpageAdapter ingests articles and blog posts from HTTP/HTTPS URLs.
type WebpageAdapter struct {
	client *http.Client
}

func init() {
	Register(&WebpageAdapter{
		client: &http.Client{Timeout: 30 * time.Second},
	})
}

func (w *WebpageAdapter) Detect(input string) bool {
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	// Exclude YouTube — handled by the YouTube adapter
	return !isYouTubeURL(input)
}

func (w *WebpageAdapter) Ingest(rawURL string) (*schema.SourceRecord, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "breakdown-cli/1.0 (content research tool)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	html := string(body)
	title := extractTitle(html)
	author := extractAuthor(html)
	text := htmlToText(html)

	if len(strings.TrimSpace(text)) < 100 {
		return nil, fmt.Errorf("extracted text too short (possible JS-rendered page or paywall): %s", rawURL)
	}

	canonical := canonicalURL(rawURL, html)
	now := time.Now()

	return &schema.SourceRecord{
		ID:           fmt.Sprintf("web_%s", urlID(canonical)),
		Type:         "webpage",
		CanonicalURL: canonical,
		Title:        title,
		Author:       author,
		Transcript:   text,
		Metadata: schema.SourceMetadata{
			ExtractedAt: now,
			Extractor:   "breakdown-http",
		},
	}, nil
}

var (
	reTitle    = regexp.MustCompile(`(?i)<title[^>]*>([^<]{1,300})</title>`)
	reOGTitle  = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']{1,300})["']`)
	reOGTitleR = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']{1,300})["'][^>]+property=["']og:title["']`)
	reAuthor   = regexp.MustCompile(`(?i)<meta[^>]+name=["']author["'][^>]+content=["']([^"']{1,200})["']`)
	reAuthorR  = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']{1,200})["'][^>]+name=["']author["']`)
	reCanon    = regexp.MustCompile(`(?i)<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']{1,500})["']`)
	reTag      = regexp.MustCompile(`<[^>]{0,200}>`)
	reScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNav      = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	reHeader   = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	reFooter   = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	reEntity   = regexp.MustCompile(`&[a-zA-Z0-9#]{1,10};`)
	reSpace    = regexp.MustCompile(`[ \t]+`)
	reNewlines = regexp.MustCompile(`\n{3,}`)
)

func extractTitle(html string) string {
	for _, re := range []*regexp.Regexp{reOGTitle, reOGTitleR, reTitle} {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return strings.TrimSpace(decodeEntities(m[1]))
		}
	}
	return "Untitled"
}

func extractAuthor(html string) string {
	for _, re := range []*regexp.Regexp{reAuthor, reAuthorR} {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return strings.TrimSpace(decodeEntities(m[1]))
		}
	}
	return ""
}

func canonicalURL(rawURL, html string) string {
	if m := reCanon.FindStringSubmatch(html); len(m) > 1 {
		c := strings.TrimSpace(m[1])
		if strings.HasPrefix(c, "http") {
			return c
		}
	}
	return rawURL
}

func htmlToText(html string) string {
	// Strip noise blocks
	html = reScript.ReplaceAllString(html, " ")
	html = reStyle.ReplaceAllString(html, " ")
	html = reNav.ReplaceAllString(html, " ")
	html = reHeader.ReplaceAllString(html, " ")
	html = reFooter.ReplaceAllString(html, " ")

	// Block elements → newlines
	html = regexp.MustCompile(`(?i)<(p|div|h[1-6]|li|br|blockquote|article|section|tr)[^>]{0,100}>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|blockquote|article|section|tr)>`).ReplaceAllString(html, "\n")

	// Strip remaining tags
	html = reTag.ReplaceAllString(html, "")

	// Decode entities
	html = decodeEntities(html)

	// Normalise whitespace
	lines := strings.Split(html, "\n")
	var kept []string
	for _, line := range lines {
		line = reSpace.ReplaceAllString(line, " ")
		line = strings.TrimSpace(line)
		if isPrintableLine(line) {
			kept = append(kept, line)
		}
	}
	text := strings.Join(kept, "\n")
	text = reNewlines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func decodeEntities(s string) string {
	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   `"`,
		"&#39;":    "'",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "…",
	}
	for enc, dec := range entities {
		s = strings.ReplaceAll(s, enc, dec)
	}
	// Remove remaining numeric entities
	s = reEntity.ReplaceAllStringFunc(s, func(m string) string {
		return ""
	})
	return s
}

func isPrintableLine(s string) bool {
	if len(s) < 3 {
		return false
	}
	printable := 0
	for _, r := range s {
		if unicode.IsPrint(r) && !unicode.IsSpace(r) {
			printable++
		}
	}
	return printable >= 3
}

func urlID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return slugify(rawURL, 32)
	}
	combined := u.Hostname() + u.Path
	return slugify(combined, 32)
}

func slugify(s string, maxLen int) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			last := rune(b.String()[b.Len()-1])
			if last != '-' {
				b.WriteRune('-')
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > maxLen {
		result = result[:maxLen]
		result = strings.TrimRight(result, "-")
	}
	return result
}

func isYouTubeURL(u string) bool {
	return strings.Contains(u, "youtube.com/watch") ||
		strings.Contains(u, "youtu.be/") ||
		strings.Contains(u, "youtube.com/shorts")
}
