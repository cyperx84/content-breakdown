package youtube

import (
	"strings"
	"testing"
)

func TestParseVTT(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantHas []string
	}{
		{
			name:    "basic",
			input:   "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n\n00:00:02.000 --> 00:00:04.000\nSecond line\n",
			wantHas: []string{"Hello world", "Second line"},
		},
		{
			name:    "with position cue",
			input:   "WEBVTT\n\n1\n00:00:00.000 --> 00:00:02.000 position:50%,start\nLine with cue\n",
			wantHas: []string{"Line with cue"},
		},
		{
			name:    "with header comments",
			input:   "WEBVTT\nKind: captions\nLanguage: en\n\n00:00:00.000 --> 00:00:02.000\nContent here\nNOTE This is a note\n",
			wantHas: []string{"Content here"},
		},
		{
			name:    "numeric cue identifier",
			input:   "WEBVTT\n\n1\n00:00:00.000 --> 00:00:02.000\nFirst\n2\n00:00:02.000 --> 00:00:04.000\nSecond\n",
			wantHas: []string{"First", "Second"},
		},
		{
			name:    "empty lines ignored",
			input:   "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\n\n\nText\n\n\n",
			wantHas: []string{"Text"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVTT(tc.input)
			for _, want := range tc.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("output %q missing %q", got, want)
				}
			}
		})
	}
}

func TestParseJSON3(t *testing.T) {
	input := `{"events":[{"segs":[{"utf8":"Hello "},{"utf8":"world"}]},{"segs":[{"utf8":"\n"},{"utf8":"Next"}]}]}`
	got := parseJSON3([]byte(input))
	if !strings.Contains(got, "Hello") {
		t.Error("missing Hello")
	}
	if !strings.Contains(got, "world") {
		t.Error("missing world")
	}
	if !strings.Contains(got, "Next") {
		t.Error("missing Next")
	}
}

func TestStripTimestamps(t *testing.T) {
	input := "00:00:00.000 --> 00:00:02.000\nText line\n\n00:00:02.000 --> 00:00:04.000\nAnother line\n"
	got := stripTimestamps(input)
	if strings.Contains(got, "-->") {
		t.Error("timestamps not stripped")
	}
	if !strings.Contains(got, "Text line") {
		t.Error("content missing")
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"", false},
		{"12a3", false},
		{" 123", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := isNumeric(tc.input)
			if got != tc.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
