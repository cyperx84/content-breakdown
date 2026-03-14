package youtube

import (
	"strings"
	"testing"
)

func TestParseVTT(t *testing.T) {
	input := `WEBVTT

00:00:00.000 --> 00:00:02.000
Hello world

1
00:00:02.000 --> 00:00:04.000 position:50%,start
Another line
`
	got := parseVTT(input)
	want := "Hello world Another line"
	if got != want {
		t.Fatalf("parseVTT() = %q, want %q", got, want)
	}
}

func TestParseJSON3(t *testing.T) {
	input := []byte(`{"events":[{"segs":[{"utf8":"Hello "},{"utf8":"world"}]},{"segs":[{"utf8":"\n"},{"utf8":"Again"}]}]}`)
	got := parseJSON3(input)
	want := "Hello  world Again"
	if got != want {
		t.Fatalf("parseJSON3() = %q, want %q", got, want)
	}
}

func TestSlug(t *testing.T) {
	got := Slug("OpenClaw: MVP / Breakdown??? with WAY too many characters to keep")
	if strings.ContainsAny(got, "/?:") {
		t.Fatalf("slug contains unsafe chars: %q", got)
	}
	if len(got) > 40 {
		t.Fatalf("slug too long: %d (%q)", len(got), got)
	}
	if got == "" {
		t.Fatal("slug should not be empty")
	}
}
