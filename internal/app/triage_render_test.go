package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func triageKey(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRenderTriageShowsEmptyState(t *testing.T) {
	m := Model{triageOpen: true}
	got := m.renderTriage(120, 30)
	if !strings.Contains(got, "Triage 0") || !strings.Contains(got, "Nothing to triage.") {
		t.Fatalf("triage overlay = %q", got)
	}
}

func TestRenderTriageShowsSelectedRow(t *testing.T) {
	m := Model{
		triageOpen:  true,
		triageItems: []triageItem{{Kind: triageMention, Title: "#dev", Actor: "Artyom", Preview: "Need help", CreateAt: 100}},
	}
	got := m.renderTriage(120, 30)
	if !strings.Contains(got, "Artyom") || !strings.Contains(got, "Need help") {
		t.Fatalf("triage overlay lost row content: %q", got)
	}
}

func TestHandleKeyTogglesTriageOverlay(t *testing.T) {
	m := Model{focus: focusSidebar}
	updated, _ := m.handleKey(triageKey("u"))
	m = updated.(Model)
	if !m.triageOpen {
		t.Fatal("triage should open")
	}
	updated, _ = m.handleKey(triageKey("esc"))
	m = updated.(Model)
	if m.triageOpen {
		t.Fatal("triage should close")
	}
}

func TestHandleKeyOpensTriageFromThreadMessages(t *testing.T) {
	m := Model{
		threadOpen:          true,
		threadFocusComposer: false,
		triageItems:         []triageItem{{Kind: triageUnreadChannel, ChannelID: "dev", Title: "#dev", UnreadCount: 1}},
	}

	updated, _ := m.handleKey(triageKey("u"))
	got := updated.(Model)
	if !got.triageOpen {
		t.Fatal("triage should open from thread messages")
	}
}

func TestHandleKeyDoesNotOpenTriageFromThreadComposer(t *testing.T) {
	m := Model{
		threadOpen:          true,
		threadFocusComposer: true,
		triageItems:         []triageItem{{Kind: triageUnreadChannel, ChannelID: "dev", Title: "#dev", UnreadCount: 1}},
	}

	updated, _ := m.handleKey(triageKey("u"))
	got := updated.(Model)
	if got.triageOpen {
		t.Fatal("triage should not open while thread composer is focused")
	}
}

func TestTriageUClosesOverlayOpenedFromThreadMessages(t *testing.T) {
	m := Model{
		threadOpen:          true,
		threadFocusComposer: false,
		triageOpen:          true,
		triageItems:         []triageItem{{Kind: triageUnreadChannel, ChannelID: "dev", Title: "#dev", UnreadCount: 1}},
	}

	updated, _ := m.handleKey(triageKey("u"))
	got := updated.(Model)
	if got.triageOpen {
		t.Fatal("u should close triage overlay even when thread remains open")
	}
}

func TestHandleTriageKeysMoveSelection(t *testing.T) {
	m := Model{
		triageOpen: true,
		triageItems: []triageItem{
			{Kind: triageUnreadChannel, ChannelID: "a", Title: "#a", UnreadCount: 1},
			{Kind: triageUnreadChannel, ChannelID: "b", Title: "#b", UnreadCount: 1},
		},
	}
	updated, _ := m.handleTriageKey(triageKey("n"))
	m = updated.(Model)
	if m.triageSelected != 1 {
		t.Fatalf("selected after n = %d", m.triageSelected)
	}
	updated, _ = m.handleTriageKey(triageKey("N"))
	m = updated.(Model)
	if m.triageSelected != 0 {
		t.Fatalf("selected after N = %d", m.triageSelected)
	}
}

func TestHelpTextMentionsTriageInbox(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "u") || !strings.Contains(got, "triage") {
		t.Fatalf("help text missing triage key: %q", got)
	}
}
