package app

import (
	"testing"

	"band-tui/internal/domain"
)

func TestFormatQuotedReplySingleLine(t *testing.T) {
	post := domain.Post{Username: "Alice", Message: "Hello"}
	got := formatQuotedReply(post)
	want := "> Alice:\n> Hello\n\n"
	if got != want {
		t.Fatalf("quote = %q, want %q", got, want)
	}
}

func TestFormatQuotedReplyMultiline(t *testing.T) {
	post := domain.Post{Username: "Alice", Message: "line 1\nline 2"}
	got := formatQuotedReply(post)
	want := "> Alice:\n> line 1\n> line 2\n\n"
	if got != want {
		t.Fatalf("quote = %q, want %q", got, want)
	}
}

func TestFormatQuotedReplyUsesUnknownWhenAuthorMissing(t *testing.T) {
	post := domain.Post{Message: "Hello"}
	got := formatQuotedReply(post)
	want := "> unknown:\n> Hello\n\n"
	if got != want {
		t.Fatalf("quote = %q, want %q", got, want)
	}
}
