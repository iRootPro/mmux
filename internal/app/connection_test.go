package app

import (
	"errors"
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

func TestReconnectConnectedEventRefreshesCurrentChannel(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{Teams: []domain.Team{{ID: "t1", Name: "team"}}}
	m.connectionState = domain.ConnectionReconnecting
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.status = "reconnecting…"

	updated, cmd := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventState, State: domain.ConnectionConnected}})
	got := updated.(Model)
	if got.connectionState != domain.ConnectionConnected || got.status != "reconnected · refreshing…" {
		t.Fatalf("unexpected reconnect state/status: state=%q status=%q", got.connectionState, got.status)
	}
	if cmd == nil {
		t.Fatal("expected refresh command after reconnect")
	}
}

func TestInitialConnectedEventDoesNotShowReconnectRefresh(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{Teams: []domain.Team{{ID: "t1", Name: "team"}}}
	m.connectionState = domain.ConnectionConnected
	m.status = "42 messages"

	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventState, State: domain.ConnectionConnected}})
	got := updated.(Model)
	if got.status == "reconnected · refreshing…" {
		t.Fatal("initial/duplicate connected event should not look like reconnect refresh")
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

func TestPostsLoadFailureKeepsCachedMessagesAndShortensError(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.posts = []domain.Post{{ID: "p1", ChannelID: "dev", Message: "cached"}}
	longErr := errors.New("get posts: GET /api/v4/channels/g89ronnxppgufqscrkj/posts?page=0&per_page=80: connection reset by peer")

	updated, _ := m.Update(postsLoadedMsg{channelID: "dev", err: longErr})
	got := updated.(Model)
	if len(got.posts) != 1 || got.posts[0].ID != "p1" {
		t.Fatalf("cached posts not preserved: %#v", got.posts)
	}
	if got.status != "refresh failed · showing cached messages" {
		t.Fatalf("status = %q", got.status)
	}
	if strings.Contains(got.err, "/api/") || strings.Contains(got.err, "g89ron") {
		t.Fatalf("error should be concise, got %q", got.err)
	}
	rendered := got.renderStatus(180)
	if strings.Contains(rendered, "/api/") || strings.Contains(rendered, "g89ron") {
		t.Fatalf("rendered status leaked full request: %q", rendered)
	}
}

func TestNetworkLoadFailureMarksConnectionReconnecting(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	err := &domain.BackendError{Kind: domain.BackendErrorNetwork, Retryable: true, Err: errors.New("connection reset by peer")}

	updated, _ := m.Update(postsLoadedMsg{channelID: "dev", err: err})
	got := updated.(Model)
	if got.connectionState != domain.ConnectionReconnecting || got.err != "network error" {
		t.Fatalf("unexpected connection/error: state=%q err=%q", got.connectionState, got.err)
	}
}
