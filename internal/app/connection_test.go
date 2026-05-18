package app

import (
	"strings"
	"testing"
	"time"

	"band-tui/internal/domain"
)

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
