package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		wantLen  int
		contains string
	}{
		{"Simple Title", 40, 13, "simple-title"},
		{"Already-lowercase", 40, 17, "already-lowercase"},
		{"With!@#Special$%Chars", 40, 22, "with-special-chars"},
		{"Multiple---Dashes", 40, 16, "multiple-dashes"},
		{"  Leading Trailing  ", 40, 14, "leading-trailing"},
		{"Very Long Title That Should Be Truncated", 20, 20, "very-long-title-that"},
		{"ALL CAPS TITLE", 40, 14, "all-caps-title"},
		{"123 Numbers 456", 40, 15, "123-numbers-456"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := slugify(tc.input, tc.maxLen)
			if len(got) > tc.maxLen {
				t.Errorf("slug length %d exceeds max %d", len(got), tc.maxLen)
			}
			if tc.contains != "" && !contains(got, tc.contains) {
				t.Errorf("slug %q doesn't contain %q", got, tc.contains)
			}
			// Should not start or end with hyphen
			if len(got) > 0 && (got[0] == '-' || got[len(got)-1] == '-') {
				t.Errorf("slug %q has leading/trailing hyphen", got)
			}
		})
	}
}

func TestDecodeEntities(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"Hello &amp; World", "Hello & World"},
		{"&lt;tag&gt;", "<tag>"},
		{"&quot;quoted&quot;", `"quoted"`},
		{"&apos;single&apos;", "'single'"},
		{"&nbsp;space", " space"},
		{"no entities", "no entities"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := decodeEntities(tc.input)
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

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
	if len(rec.Transcript) < 50 {
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

func TestHtmlToText(t *testing.T) {
	html := `<html><head><title>Test</title><style>body{color:red}</style></head>
<body><nav>nav stuff</nav><h1>Hello World</h1><p>This is a paragraph about interesting things.</p>
<script>alert('x')</script><footer>footer stuff</footer></body></html>`
	text := htmlToText(html)
	if text == "" {
		t.Fatal("htmlToText returned empty string")
	}
	if contains(text, "nav stuff") || contains(text, "footer stuff") || contains(text, "alert") {
		t.Fatalf("htmlToText should strip nav/footer/script, got: %s", text)
	}
	if !contains(text, "Hello World") || !contains(text, "paragraph") {
		t.Fatalf("htmlToText missing expected content: %s", text)
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

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
