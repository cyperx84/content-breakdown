// Package llm centralizes LLM prompt execution and response parsing
// for the content-breakdown pipeline.
//
// Two call modes are supported:
//   - stdio: prompt is written to stdout and the response is read from stdin
//     (keyless; the user or wrapper supplies the LLM)
//   - cmd: prompt is piped through an external command (e.g. `claude --print`)
//
// The package also contains helpers for cleaning LLM JSON responses
// (which tend to be wrapped in code fences or prose) and for deduplicating
// string slices.
package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// DefaultTimeout is the ceiling for external LLM commands. It is deliberately
// generous for long transcripts but finite so a hung subprocess cannot block
// the pipeline indefinitely.
const DefaultTimeout = 10 * time.Minute

// Options configures a single LLM invocation.
type Options struct {
	// LLMCmd is an external command to pipe the prompt through. When empty,
	// stdio mode is used: the prompt is written to stdout and the response
	// is read from stdin.
	LLMCmd string

	// Timeout bounds the external command. Zero means DefaultTimeout.
	// Only applies to command mode; stdio mode blocks on user input.
	Timeout time.Duration

	// Verbose prints progress lines to stderr using LogPrefix.
	Verbose bool

	// LogPrefix is prepended to verbose log lines, e.g. "[extract]".
	LogPrefix string
}

// Call runs the prompt through either an external command or stdio mode and
// returns the raw response text.
func Call(prompt string, opts Options) (string, error) {
	if opts.LLMCmd != "" {
		return callCmd(prompt, opts)
	}
	return callStdio(prompt, opts)
}

func callStdio(prompt string, opts Options) (string, error) {
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "%s Awaiting LLM response on stdin...\n", opts.LogPrefix)
		fmt.Fprintf(os.Stderr, "%s Prompt written to stdout.\n", opts.LogPrefix)
	}

	fmt.Print(prompt)

	var response bytes.Buffer
	if _, err := io.Copy(&response, os.Stdin); err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return response.String(), nil
}

func callCmd(prompt string, opts Options) (string, error) {
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "%s Running LLM command: %s\n", opts.LogPrefix, opts.LLMCmd)
	}

	parts, err := SplitCommand(opts.LLMCmd)
	if err != nil {
		return "", fmt.Errorf("parse llm-cmd: %w", err)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty LLM command")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("llm command timed out after %s: %s", timeout, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExtractJSONObject strips common LLM wrappings (code fences, preamble, postamble)
// and returns the outermost `{...}` object found in the response. Returns the
// empty string if no balanced object can be located.
func ExtractJSONObject(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(response[start : end+1])
}

// UniqueStrings returns the input slice with empty entries removed and
// case-insensitive duplicates collapsed. If sorted is true the result is
// sorted alphabetically; otherwise insertion order is preserved.
func UniqueStrings(items []string, sorted bool) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if sorted {
		sort.Strings(out)
	}
	return out
}

// SplitCommand parses a command-line string into argv, honoring single and
// double quotes and backslash escapes. It is intentionally minimal: enough
// to handle `claude --print --system-prompt "be concise"` without dragging
// in a shell dependency.
func SplitCommand(cmdStr string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inWord  bool
		quote   rune // 0 = not in quotes, '\'' or '"' otherwise
	)

	flush := func() {
		if inWord {
			args = append(args, current.String())
			current.Reset()
			inWord = false
		}
	}

	runes := []rune(cmdStr)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == 0 && (r == ' ' || r == '\t'):
			flush()
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
			inWord = true
		case quote != 0 && r == quote:
			quote = 0
		case r == '\\' && i+1 < len(runes):
			// escape next rune (both inside and outside quotes)
			i++
			current.WriteRune(runes[i])
			inWord = true
		default:
			current.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote in command", quote)
	}
	flush()
	return args, nil
}
