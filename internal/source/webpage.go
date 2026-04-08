package source

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/slug"
)

// WebpageAdapter ingests articles and blog posts from HTTP/HTTPS URLs.
type WebpageAdapter struct {
	client  *http.Client
	timeout time.Duration
}

const (
	defaultWebpageTimeout = 30 * time.Second
	maxWebpageBodyBytes   = 5 * 1024 * 1024
)

func init() {
	RegisterWithPriority(&WebpageAdapter{
		client:  &http.Client{Timeout: defaultWebpageTimeout},
		timeout: defaultWebpageTimeout,
	}, PriorityMedium)
}

func (w *WebpageAdapter) Detect(input string) bool {
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	return !IsYouTubeURL(input)
}

func (w *WebpageAdapter) Ingest(rawURL string) (*schema.SourceRecord, error) {
	timeout := w.timeout
	if timeout <= 0 {
		timeout = defaultWebpageTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "breakdown-cli (content research tool)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWebpageBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	rawHTML := string(body)
	title := extractTitle(rawHTML)
	author := extractAuthor(rawHTML)
	text := htmlToText(rawHTML)

	if len(strings.TrimSpace(text)) < 100 {
		return nil, fmt.Errorf("extracted text too short (possible JS-rendered page or paywall): %s", rawURL)
	}

	canonical := canonicalURL(rawURL, rawHTML)
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
	reSpace    = regexp.MustCompile(`[ \t]+`)
	reNewlines = regexp.MustCompile(`\n{3,}`)
)

func extractTitle(rawHTML string) string {
	for _, re := range []*regexp.Regexp{reOGTitle, reOGTitleR, reTitle} {
		if m := re.FindStringSubmatch(rawHTML); len(m) > 1 {
			return strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	return "Untitled"
}

func extractAuthor(rawHTML string) string {
	for _, re := range []*regexp.Regexp{reAuthor, reAuthorR} {
		if m := re.FindStringSubmatch(rawHTML); len(m) > 1 {
			return strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	return ""
}

func canonicalURL(rawURL, rawHTML string) string {
	if m := reCanon.FindStringSubmatch(rawHTML); len(m) > 1 {
		c := strings.TrimSpace(m[1])
		if strings.HasPrefix(c, "http") {
			return c
		}
	}
	return rawURL
}

func htmlToText(rawHTML string) string {
	// Strip noise blocks
	rawHTML = reScript.ReplaceAllString(rawHTML, " ")
	rawHTML = reStyle.ReplaceAllString(rawHTML, " ")
	rawHTML = reNav.ReplaceAllString(rawHTML, " ")
	rawHTML = reHeader.ReplaceAllString(rawHTML, " ")
	rawHTML = reFooter.ReplaceAllString(rawHTML, " ")

	// Block elements → newlines
	rawHTML = regexp.MustCompile(`(?i)<(p|div|h[1-6]|li|br|blockquote|article|section|tr)[^>]{0,100}>`).ReplaceAllString(rawHTML, "\n")
	rawHTML = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|blockquote|article|section|tr)>`).ReplaceAllString(rawHTML, "\n")

	// Strip remaining tags
	rawHTML = reTag.ReplaceAllString(rawHTML, "")

	// Decode all named + numeric HTML entities via stdlib
	rawHTML = html.UnescapeString(rawHTML)

	// Normalise whitespace
	lines := strings.Split(rawHTML, "\n")
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
		return slug.Make(rawURL, 32)
	}
	combined := u.Hostname() + u.Path
	return slug.Make(combined, 32)
}
