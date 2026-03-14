package source

import (
	"os"
	"path/filepath"
	"testing"
)

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
