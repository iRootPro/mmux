package app

import (
	"testing"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func actionKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
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

func TestQuoteSelectedPostIntoEmptyComposer(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.quoteSelectedPost()
	got := updated.(Model)
	if got.focus != focusComposer {
		t.Fatalf("focus = %v, want composer", got.focus)
	}
	want := "> Alice:\n> Hello\n\n"
	if got.composer.Value() != want {
		t.Fatalf("composer = %q, want %q", got.composer.Value(), want)
	}
}

func TestQuoteSelectedPostAppendsBelowExistingDraft(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0
	m.composer.SetValue("draft")

	updated, _ := m.quoteSelectedPost()
	got := updated.(Model)
	want := "draft\n> Alice:\n> Hello\n\n"
	if got.composer.Value() != want {
		t.Fatalf("composer = %q, want %q", got.composer.Value(), want)
	}
}

func TestHandleTimelineKeyRQuotesSelectedPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.handleKey(actionKey("r"))
	got := updated.(Model)
	if got.focus != focusComposer || got.composer.Value() == "" {
		t.Fatalf("quote not inserted, focus=%v composer=%q", got.focus, got.composer.Value())
	}
}
