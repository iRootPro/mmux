package app

import (
	"strings"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	"band-tui/internal/mock"
)

func TestThreadLayoutUsesSingleSharedComposer(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.width = 180
	m.height = 50
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.threadPosts = []domain.Post{
		{ID: "root", Username: "Alice", Message: "root"},
		{ID: "r1", RootID: "root", Username: "Bob", Message: "reply"},
	}
	m.resize()
	m.refreshViewport()
	m.refreshThreadViewport()
	got := m.renderThreadLayout(m.width, m.height)
	if strings.Count(got, "reply to: Alice") != 1 {
		t.Fatalf("expected one shared thread composer, got %d", strings.Count(got, "reply to: Alice"))
	}
	if strings.Contains(got, "Main composer is paused") {
		t.Fatalf("main paused composer should not be rendered: %q", got)
	}
}
