// Package slug produces filesystem-safe slugs from arbitrary strings.
package slug

import "strings"

// Make returns a lowercased, hyphen-separated slug truncated to maxLen.
// Runs of non-[a-z0-9] characters collapse into a single hyphen and
// leading/trailing hyphens are stripped.
func Make(s string, maxLen int) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			if b.Len() > 0 {
				last := b.String()[b.Len()-1]
				if last != '-' {
					b.WriteByte('-')
				}
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if maxLen > 0 && len(out) > maxLen {
		out = strings.TrimRight(out[:maxLen], "-")
	}
	return out
}

// Title is a convenience wrapper with the 40-char limit used for YouTube titles.
func Title(s string) string {
	return Make(s, 40)
}
