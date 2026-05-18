package app

import (
	"testing"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func threadKey(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestThreadSelectionDefaultsToLastReply(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.threadOpen = true
	m.threadPosts = []domain.Post{
		{ID: "root", ChannelID: "dev", Message: "root"},
		{ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply 1"},
		{ID: "r2", ChannelID: "dev", RootID: "root", Message: "reply 2"},
	}

	m.clampThreadSelection()
	if m.threadSelected != 2 {
		t.Fatalf("threadSelected = %d", m.threadSelected)
	}
}

func TestHandleThreadKeyMovesSelectedThreadPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.threadOpen = true
	m.threadPosts = []domain.Post{
		{ID: "root", ChannelID: "dev", Message: "root"},
		{ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply 1"},
	}
	m.threadSelected = 1

	updated, _ := m.handleThreadKey(threadKey("up"))
	got := updated.(Model)
	if got.threadSelected != 0 {
		t.Fatalf("threadSelected = %d", got.threadSelected)
	}
}
