package slug

import (
	"strings"
	"testing"
)

func TestMake(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		max    int
		expect string
	}{
		{"simple", "Simple Title", 40, "simple-title"},
		{"special chars", "Hello, World!", 40, "hello-world"},
		{"multi-dash collapse", "a--b---c", 40, "a-b-c"},
		{"leading/trailing", "!!foo bar!!", 40, "foo-bar"},
		{"unicode dropped", "résumé over here", 40, "r-sum-over-here"},
		{"truncate", strings.Repeat("a", 100), 20, "aaaaaaaaaaaaaaaaaaaa"},
		{"truncate trims dash", "aaaaaaaaaaaaaaaaaa-bb", 20, "aaaaaaaaaaaaaaaaaa-b"},
		{"only symbols", "!@#$%", 40, ""},
		{"empty", "", 40, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Make(tc.input, tc.max)
			if got != tc.expect {
				t.Errorf("Make(%q,%d) = %q, want %q", tc.input, tc.max, got, tc.expect)
			}
		})
	}
}

func TestTitle(t *testing.T) {
	got := Title("A Very Long YouTube Title That Should Get Truncated At Some Point")
	if len(got) > 40 {
		t.Fatalf("title slug too long: %d", len(got))
	}
	if strings.HasSuffix(got, "-") || strings.HasPrefix(got, "-") {
		t.Fatalf("title slug has leading/trailing dash: %q", got)
	}
}
