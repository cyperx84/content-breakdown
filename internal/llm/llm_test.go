package llm

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "claude", []string{"claude"}, false},
		{"flags", "claude --print -p", []string{"claude", "--print", "-p"}, false},
		{
			name:  "double-quoted arg with spaces",
			input: `claude --system-prompt "be concise and direct"`,
			want:  []string{"claude", "--system-prompt", "be concise and direct"},
		},
		{
			name:  "single-quoted arg",
			input: `claude --p 'with spaces'`,
			want:  []string{"claude", "--p", "with spaces"},
		},
		{
			name:  "mixed quotes",
			input: `sh -c "echo 'hello world'"`,
			want:  []string{"sh", "-c", "echo 'hello world'"},
		},
		{
			name:  "escaped space",
			input: `foo bar\ baz qux`,
			want:  []string{"foo", "bar baz", "qux"},
		},
		{
			name:    "unterminated quote",
			input:   `claude "oops`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SplitCommand(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    bool
		contain string
	}{
		{"clean", `{"a":1}`, true, `"a":1`},
		{"fenced", "```json\n{\"a\":1}\n```", true, `"a":1`},
		{"plain fence", "```\n{\"a\":1}\n```", true, `"a":1`},
		{"with preamble", "Here it is:\n{\"a\":1}", true, `"a":1`},
		{"with postamble", `{"a":1}\nlet me know if...`, true, `"a":1`},
		{"no object", "none", false, ""},
		{"open brace only", "{", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractJSONObject(tc.input)
			if tc.want {
				if got == "" {
					t.Fatal("expected non-empty")
				}
				if !strings.Contains(got, tc.contain) {
					t.Fatalf("got %q, want substring %q", got, tc.contain)
				}
			} else if got != "" {
				t.Fatalf("expected empty, got %q", got)
			}
		})
	}
}

func TestUniqueStringsSorted(t *testing.T) {
	in := []string{"b", "A", "a ", "B", "", "c"}
	got := UniqueStrings(in, true)
	want := []string{"A", "b", "c"}
	// case-insensitive dedup keeps first occurrence ("b" then "B" → "b"; "A" then "a" → "A")
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := 1; i < len(got); i++ {
		if strings.ToLower(got[i-1]) > strings.ToLower(got[i]) {
			t.Fatalf("not sorted: %v", got)
		}
	}
}

func TestUniqueStringsUnsorted(t *testing.T) {
	in := []string{"zeta", "Alpha", "alpha", "Beta"}
	got := UniqueStrings(in, false)
	want := []string{"zeta", "Alpha", "Beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
