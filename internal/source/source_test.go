package source

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsPrintableLine(t *testing.T) {
	tests := []struct {
		line   string
		expect bool
	}{
		{"normal text line", true},
		{"ab", false},  // too short
		{"   ", false}, // only spaces
		{"", false},
		{"~~~", true}, // punctuation is printable
	}

	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			got := isPrintableLine(tc.line)
			if got != tc.expect {
				t.Errorf("isPrintableLine(%q) = %v, want %v", tc.line, got, tc.expect)
			}
		})
	}
}

func TestWebpageDetect(t *testing.T) {
	a := &WebpageAdapter{}

	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/article", true},
		{"http://blog.example.com/post", true},
		{"https://www.youtube.com/watch?v=abc", false},
		{"https://youtu.be/abc123", false},
		{"https://youtube.com/shorts/xyz", false},
		{"/local/path/file.md", false},
		{"not-a-url", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := a.Detect(tc.input)
			if got != tc.want {
				t.Errorf("Detect(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLocalFileDetect(t *testing.T) {
	a := &LocalFileAdapter{}

	tests := []struct {
		input string
		want  bool
	}{
		{"file.md", true},
		{"file.txt", true},
		{"file.text", true},
		{"file.markdown", true},
		{"file.pdf", true},
		{"file.go", false},
		{"https://example.com", false},
		{"file.json", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := a.Detect(tc.input)
			if got != tc.want {
				t.Errorf("Detect(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestInferTitle(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		content  string
		expected string
	}{
		{
			name:     "markdown with H1",
			path:     "/path/to/my-note.md",
			content:  "# Real Title\n\nSome content here.",
			expected: "Real Title",
		},
		{
			name:     "no H1, use filename",
			path:     "/path/to/my-note.md",
			content:  "Just content without heading.",
			expected: "My note",
		},
		{
			name:     "filename with underscores",
			path:     "/path/to/my_awesome_note.md",
			content:  "Content.",
			expected: "My awesome note",
		},
		{
			name:     "txt file, no H1",
			path:     "/path/to/article.txt",
			content:  "Plain text content.",
			expected: "Article",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inferTitle(tc.path, tc.content)
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestLocalFileIngestMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-note.md")
	content := "# Agent Patterns\n\nThis is a note about building agent workflows with persistence and structured output.\n\nAgents need state management, tool calling, and reliable JSON output to be practical."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := &LocalFileAdapter{}
	rec, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest error: %v", err)
	}
	if rec.Title != "Agent Patterns" {
		t.Fatalf("title = %q, want %q", rec.Title, "Agent Patterns")
	}
	if rec.Type != "markdown" {
		t.Fatalf("type = %q, want markdown", rec.Type)
	}
	if len(rec.Transcript) < MinLocalFileChars {
		t.Fatalf("transcript too short: %d", len(rec.Transcript))
	}
}

func TestLocalFileTitleFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-article-title.txt")
	if err := os.WriteFile(path, []byte("This is some plain text content about various interesting topics that has enough characters."), 0644); err != nil {
		t.Fatal(err)
	}
	a := &LocalFileAdapter{}
	rec, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest error: %v", err)
	}
	if rec.Title != "My article title" {
		t.Fatalf("title fallback = %q, want %q", rec.Title, "My article title")
	}
}

func TestLocalFileTooShort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.md")
	if err := os.WriteFile(path, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	a := &LocalFileAdapter{}
	if _, err := a.Ingest(path); err == nil {
		t.Fatal("expected too-short error")
	}
}

func TestHtmlToText(t *testing.T) {
	rawHTML := `<html><head><title>Test</title><style>body{color:red}</style></head>
<body><nav>nav stuff</nav><h1>Hello World</h1><p>This is a paragraph about interesting things.</p>
<script>alert('x')</script><footer>footer stuff</footer></body></html>`
	text := htmlToText(rawHTML)
	if text == "" {
		t.Fatal("htmlToText returned empty string")
	}
	if strings.Contains(text, "nav stuff") || strings.Contains(text, "footer stuff") || strings.Contains(text, "alert") {
		t.Fatalf("htmlToText should strip nav/footer/script, got: %s", text)
	}
	if !strings.Contains(text, "Hello World") || !strings.Contains(text, "paragraph") {
		t.Fatalf("htmlToText missing expected content: %s", text)
	}
}

func TestHtmlToTextDecodesNumericEntities(t *testing.T) {
	rawHTML := "<p>Caf&#233; &amp; r&#xe9;sum&#233; with &#x2014; dash and &nbsp; nbsp.</p>"
	got := htmlToText(rawHTML)
	for _, want := range []string{"Café", "résumé", "& ", "—"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "og:title preferred",
			html: `<html><head><title>fallback</title><meta property="og:title" content="OG Title Wins"></head>`,
			want: "OG Title Wins",
		},
		{
			name: "title fallback",
			html: `<html><head><title>Plain Title</title></head>`,
			want: "Plain Title",
		},
		{
			name: "decodes entities",
			html: `<title>Tom &amp; Jerry &#8212; Cartoon</title>`,
			want: "Tom & Jerry — Cartoon",
		},
		{
			name: "missing",
			html: `<html><head></head>`,
			want: "Untitled",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTitle(tc.html)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractAuthor(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{"name first", `<meta name="author" content="Jane Doe">`, "Jane Doe"},
		{"content first", `<meta content="Jane Doe" name="author">`, "Jane Doe"},
		{"missing", `<meta name="description" content="x">`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAuthor(tc.html)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCanonicalURLPrefersLink(t *testing.T) {
	html := `<head><link rel="canonical" href="https://example.com/canonical"></head>`
	got := canonicalURL("https://example.com/raw?x=1", html)
	if got != "https://example.com/canonical" {
		t.Errorf("got %q", got)
	}
}

func TestCanonicalURLFallbackOnRelative(t *testing.T) {
	html := `<head><link rel="canonical" href="/relative-path"></head>`
	got := canonicalURL("https://example.com/raw", html)
	if got != "https://example.com/raw" {
		t.Errorf("expected fallback to raw URL, got %q", got)
	}
}

func TestDetectYouTube(t *testing.T) {
	cases := []string{
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/abc123",
		"https://youtube.com/shorts/abc123",
	}
	a := &YouTubeAdapter{}
	for _, c := range cases {
		if !a.Detect(c) {
			t.Fatalf("YouTubeAdapter.Detect(%q) = false, want true", c)
		}
	}
}

func TestDetectWebpage(t *testing.T) {
	a := &WebpageAdapter{}
	if !a.Detect("https://blog.example.com/post/1") {
		t.Fatal("WebpageAdapter should detect HTTPS URLs")
	}
	if a.Detect("https://www.youtube.com/watch?v=abc") {
		t.Fatal("WebpageAdapter should not detect YouTube URLs")
	}
	if a.Detect("/some/local/file.md") {
		t.Fatal("WebpageAdapter should not detect local paths")
	}
}

func TestDetectLocalFile(t *testing.T) {
	a := &LocalFileAdapter{}
	for _, ext := range []string{".md", ".txt", ".pdf"} {
		if !a.Detect("file" + ext) {
			t.Fatalf("LocalFileAdapter.Detect(%q) = false", "file"+ext)
		}
	}
	if a.Detect("https://example.com/article") {
		t.Fatal("LocalFileAdapter should not detect HTTP URLs")
	}
}

// TestAdapterRegistrationOrder asserts the priority invariant: YouTube must be
// consulted before the generic webpage adapter, otherwise YouTube URLs would
// fall through to webpage and lose video-specific handling.
func TestAdapterRegistrationOrder(t *testing.T) {
	adapters := Adapters()
	var ytIdx, webIdx, fileIdx int = -1, -1, -1
	for i, a := range adapters {
		switch a.(type) {
		case *YouTubeAdapter:
			ytIdx = i
		case *WebpageAdapter:
			webIdx = i
		case *LocalFileAdapter:
			fileIdx = i
		}
	}
	if ytIdx == -1 || webIdx == -1 || fileIdx == -1 {
		t.Fatalf("missing adapter — yt:%d web:%d file:%d", ytIdx, webIdx, fileIdx)
	}
	if !(ytIdx < webIdx && webIdx < fileIdx) {
		t.Fatalf("adapters out of priority order: yt=%d web=%d file=%d", ytIdx, webIdx, fileIdx)
	}
}

func TestWebpageIngestEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>
<head>
<title>Sample Post</title>
<meta name="author" content="Test Author">
<link rel="canonical" href="` + "https://canonical.example.com/post" + `">
</head>
<body>
<nav>menu</nav>
<article>
<h1>Sample Post Heading</h1>
<p>This is a long enough body of an article so that the extracted text passes the 100 character minimum threshold.</p>
<p>Another paragraph with content for clarity and length so the test stays robust.</p>
</article>
<footer>copyright</footer>
</body>
</html>`))
	}))
	defer srv.Close()

	a := &WebpageAdapter{
		client:  &http.Client{Timeout: 5 * time.Second},
		timeout: 5 * time.Second,
	}
	rec, err := a.Ingest(srv.URL)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if rec.Title != "Sample Post" {
		t.Errorf("title = %q", rec.Title)
	}
	if rec.Author != "Test Author" {
		t.Errorf("author = %q", rec.Author)
	}
	if rec.CanonicalURL != "https://canonical.example.com/post" {
		t.Errorf("canonical = %q", rec.CanonicalURL)
	}
	if !strings.Contains(rec.Transcript, "Sample Post Heading") {
		t.Errorf("transcript missing heading: %q", rec.Transcript)
	}
	if strings.Contains(rec.Transcript, "menu") || strings.Contains(rec.Transcript, "copyright") {
		t.Errorf("transcript should drop nav/footer: %q", rec.Transcript)
	}
}
