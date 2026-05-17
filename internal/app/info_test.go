package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderInfoBodyShowsChannelDetailsAndMarkdown(t *testing.T) {
	m := Model{}
	ch := domain.Channel{
		ID:          "c1",
		Type:        "O",
		DisplayName: "DevSecOps",
		Header:      "**Important** [docs](https://example.com)",
		Purpose:     "- keep secure",
		MemberCount: 42,
		Unread:      2,
		Mentions:    1,
	}
	got := m.renderInfoBody(ch, 80)
	for _, want := range []string{"# DevSecOps", "members: 42", "unread: 2", "mentions: 1", "Important", "docs", "https://example.com", "keep secure"} {
		if !strings.Contains(got, want) {
			t.Fatalf("info body missing %q in %q", want, got)
		}
	}
}

func TestInfoKeyDoesNotOpenWhileTyping(t *testing.T) {
	m := Model{focus: focusComposer}
	model, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if model.(Model).infoOpen {
		t.Fatal("info opened while composer focused")
	}
}
