package app

import (
	"strings"
	"testing"
	"time"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func connectionKey(s string) tea.KeyMsg {
	switch s {
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
func TestBackendEventStateReconnectingUpdatesConnectionState(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.status = "42 messages"

	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventState, State: domain.ConnectionReconnecting, Attempt: 2, RetryIn: 5 * time.Second}})
	got := updated.(Model)
	if got.connectionState != domain.ConnectionReconnecting || got.connectionAttempt != 2 {
		t.Fatalf("unexpected connection state: %#v", got)
	}
}

func TestRenderStatusShowsAuthExpiredAction(t *testing.T) {
	m := Model{
		connectionState:   domain.ConnectionAuthExpired,
		connectionMessage: "refresh token and restart",
		status:            "42 messages",
		width:             120,
	}
	got := m.renderStatus(120)
	if !strings.Contains(got, "auth expired") || !strings.Contains(got, "refresh token") {
		t.Fatalf("status = %q", got)
	}
}

func TestCtrlRReconnectsWhenOffline(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.connectionState = domain.ConnectionOffline
	m.focus = focusSidebar

	updated, cmd := m.handleKey(connectionKey("ctrl+r"))
	got := updated.(Model)
	if got.connectionState != domain.ConnectionConnecting {
		t.Fatalf("ctrl+r should move offline state back to connecting, got %q", got.connectionState)
	}
	if cmd == nil {
		t.Fatal("expected reconnect command")
	}
}

func TestCtrlRStillReloadsCurrentChannelWhenConnected(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.connectionState = domain.ConnectionConnected
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusSidebar

	_, cmd := m.handleKey(connectionKey("ctrl+r"))
	if cmd == nil {
		t.Fatal("expected reload command")
	}
}

func TestAuthExpiredDoesNotPretendReconnectSucceeded(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	updated, _ := m.Update(sessionLoadedMsg{err: &domain.BackendError{Kind: domain.BackendErrorAuth, Retryable: false}})
	got := updated.(Model)
	if got.connectionState != domain.ConnectionAuthExpired {
		t.Fatalf("expected auth expired state, got %q", got.connectionState)
	}
}

func TestHelpTextMentionsReconnectBehavior(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "ctrl+r") || !strings.Contains(got, "reconnect") {
		t.Fatalf("help text missing reconnect hint: %q", got)
	}
}
