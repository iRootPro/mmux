package app

import (
	"strings"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	"band-tui/internal/mock"
)

func TestRenderMainOmitsPausedComposerWhenThreadOpen(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.width = 160
	m.height = 40
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadPosts = []domain.Post{{ID: "root", Username: "Alice", Message: "root"}}
	m.resize()

	got := m.renderMain(100, 30)

	if !strings.Contains(got, "# Town") {
		t.Fatalf("main thread layout lost channel header: %q", got)
	}
	if strings.Contains(got, "Main composer is paused") || strings.Contains(got, "message # Town") {
		t.Fatalf("main thread layout should omit paused composer: %q", got)
	}
}
