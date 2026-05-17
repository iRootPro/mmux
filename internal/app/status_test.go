package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderStatusShowsScope(t *testing.T) {
	m := Model{status: "ready", session: &domain.Session{Teams: []domain.Team{{DisplayName: "Infra Projects"}}}}
	got := m.renderStatus(120)
	if !strings.Contains(got, "scope: Infra Projects") || !strings.Contains(got, "ready") {
		t.Fatalf("status = %q", got)
	}
}

func TestRenderStatusShowsFocusHint(t *testing.T) {
	m := Model{status: "ready", focus: focusTimeline}
	got := m.renderStatus(120)
	if !strings.Contains(got, "timeline") || !strings.Contains(got, "t thread") || !strings.Contains(got, "n unread") {
		t.Fatalf("timeline status hint missing: %q", got)
	}

	m.focus = focusSidebar
	got = m.renderStatus(120)
	if !strings.Contains(got, "sidebar") || !strings.Contains(got, "/ filter") || !strings.Contains(got, "enter open") {
		t.Fatalf("sidebar status hint missing: %q", got)
	}
}

func TestRenderStatusShowsComposerAndThreadHints(t *testing.T) {
	m := Model{status: "ready", focus: focusComposer}
	got := m.renderStatus(120)
	if !strings.Contains(got, "at latest") {
		t.Fatalf("composer status should show scroll context: %q", got)
	}
	for _, duplicate := range []string{"composer", "enter send", "ctrl+j newline", "tab nav"} {
		if strings.Contains(got, duplicate) {
			t.Fatalf("composer status duplicates composer hint %q in: %q", duplicate, got)
		}
	}
	if h := lipgloss.Height(got); h != 1 {
		t.Fatalf("status height = %d, want 1", h)
	}

	m.threadOpen = true
	m.threadPosts = []domain.Post{{ID: "root"}, {ID: "r1", RootID: "root"}}
	m.threadFocusComposer = false
	got = m.renderStatus(120)
	if !strings.Contains(got, "thread messages") || !strings.Contains(got, "2 messages") || !strings.Contains(got, "tab reply") || !strings.Contains(got, "esc close") {
		t.Fatalf("thread messages status hint missing: %q", got)
	}
	if strings.Count(got, "thread messages") != 1 {
		t.Fatalf("thread messages status should not duplicate mode label: %q", got)
	}

	m.threadFocusComposer = true
	m.status = "thread reply"
	got = m.renderStatus(120)
	if !strings.Contains(got, "thread reply") || !strings.Contains(got, "2 messages") || !strings.Contains(got, "tab messages") || !strings.Contains(got, "esc close") {
		t.Fatalf("thread reply status hint missing: %q", got)
	}
	if strings.Count(got, "thread reply") != 1 || strings.Contains(got, "reply right") || strings.Contains(got, "tab thread") {
		t.Fatalf("thread status has stale, duplicated, or ambiguous hint: %q", got)
	}
}

func TestRenderStatusDefaultsEmptyStatusToReady(t *testing.T) {
	m := Model{}

	got := m.renderStatus(120)

	if !strings.Contains(got, "ready") || strings.Contains(got, "·   ") {
		t.Fatalf("empty status should render ready without dangling separator: %q", got)
	}
}
