package app

import (
	"strings"
	"testing"
)

func TestRenderMarkdownMessage(t *testing.T) {
	got := renderMarkdownMessage("hello **world** `code` [site](https://example.com) @alice", 80)
	for _, want := range []string{"world", "code", "site", "https://example.com", "@alice"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered markdown missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "**") || strings.Contains(got, "[site]") {
		t.Fatalf("markdown markers leaked: %q", got)
	}
}

func TestRenderMarkdownMessagePreservesCodeFence(t *testing.T) {
	got := renderMarkdownMessage("```go\nfmt.Println(1)\n```", 80)
	if !strings.Contains(got, "fmt.Println(1)") {
		t.Fatalf("missing code: %q", got)
	}
	if strings.Contains(got, "```") {
		t.Fatalf("fence marker leaked: %q", got)
	}
}

func TestSanitizeMessageTextKeepsNewlines(t *testing.T) {
	got := sanitizeMessageText("a\nb\x1b]bad")
	if got != "a\nb]bad" {
		t.Fatalf("got %q", got)
	}
}
