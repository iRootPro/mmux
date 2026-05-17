package app

import (
	"strings"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"

	"github.com/charmbracelet/lipgloss"
)

func TestSidebarTitleKeepsWorkspaceWhenTeamLooksLikeUser(t *testing.T) {
	m := Model{
		cfg: config.Config{ServerURL: "https://band.wb.ru"},
		session: &domain.Session{
			User:  domain.User{DisplayName: "Александр Неупокоев"},
			Teams: []domain.Team{{DisplayName: "Александр Неупокоев"}},
		},
	}
	if got := m.sidebarTitle(); got != "WB" {
		t.Fatalf("sidebarTitle = %q", got)
	}
}

func TestRenderSidebarShowsScopeLabel(t *testing.T) {
	m := Model{width: 120, session: &domain.Session{User: domain.User{DisplayName: "Sasha"}, Teams: []domain.Team{{DisplayName: "Infra"}}}}
	got := m.renderSidebar(32, 10)
	if !strings.Contains(got, "scope: Infra") || !strings.Contains(got, "F2 scopes") {
		t.Fatalf("sidebar = %q", got)
	}
}

func TestSidebarChannelLineWithPresenceKeepsNameWhenSelected(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Евгения Мусанова", Status: "offline"}}, selectedChannel: 0}
	line := m.renderSidebarChannelLine(0, 30)
	if !strings.Contains(line, "Евгения") || !strings.Contains(line, "○") {
		t.Fatalf("line = %q", line)
	}
}

func TestRenderSidebarChannelLineKeepsMentionBadgeVisible(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Very Long Channel Name That Must Truncate", Mentions: 12, Unread: 99}},
	}

	got := m.renderSidebarChannelLine(0, 24)

	if !strings.Contains(got, "@12") {
		t.Fatalf("mention badge not visible in narrow row: %q", got)
	}
	if strings.Contains(got, "99") {
		t.Fatalf("mentions must take priority over unread count: %q", got)
	}
}

func TestRenderSidebarSelectedBadgeFitsWidth(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Very Long Channel Name That Must Truncate", Mentions: 12}},
		selectedChannel: 0,
	}

	got := m.renderSidebarChannelLine(0, 24)

	if !strings.Contains(got, "@12") {
		t.Fatalf("selected mention badge not visible: %q", got)
	}
	if width := lipgloss.Width(got); width > 24 {
		t.Fatalf("selected row width = %d, want <= 24: %q", width, got)
	}
}

func TestRenderSidebarChannelLineShowsUnreadCount(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "General", Unread: 7}},
	}

	got := m.renderSidebarChannelLine(0, 24)

	if !strings.Contains(got, "7") {
		t.Fatalf("unread count not visible: %q", got)
	}
}

func TestCropSidebarLinesShowsHiddenCounts(t *testing.T) {
	items := []sidebarLine{
		{Text: "▾ ЛИЧНЫЕ", Section: "ЛИЧНЫЕ", Header: true},
		{Text: "one", Section: "ЛИЧНЫЕ"},
		{Text: "two", Section: "ЛИЧНЫЕ"},
		{Text: "three", Section: "ЛИЧНЫЕ"},
		{Text: "four", Section: "ЛИЧНЫЕ"},
		{Text: "five", Section: "ЛИЧНЫЕ"},
		{Text: "six", Section: "ЛИЧНЫЕ"},
	}

	got := strings.Join(cropSidebarLines(items, 5, 4), "\n")

	if !strings.Contains(got, "↑ ещё 4 · личные") || !strings.Contains(got, "five") {
		t.Fatalf("crop labels should include accurate hidden count and selected section: %q", got)
	}
}

func TestCropSidebarLinesShowsBelowHiddenCount(t *testing.T) {
	items := []sidebarLine{
		{Text: "▾ ЛИЧНЫЕ", Section: "ЛИЧНЫЕ", Header: true},
		{Text: "one", Section: "ЛИЧНЫЕ"},
		{Text: "two", Section: "ЛИЧНЫЕ"},
		{Text: "three", Section: "ЛИЧНЫЕ"},
		{Text: "four", Section: "ЛИЧНЫЕ"},
		{Text: "five", Section: "ЛИЧНЫЕ"},
		{Text: "six", Section: "ЛИЧНЫЕ"},
	}

	got := strings.Join(cropSidebarLines(items, 1, 4), "\n")

	if !strings.Contains(got, "↓ ещё 4 · личные") || !strings.Contains(got, "one") {
		t.Fatalf("crop labels should include accurate below count and selected row: %q", got)
	}
}

func TestCropSidebarLinesKeepsSelectedRowVisibleInCompactHeight(t *testing.T) {
	items := []sidebarLine{
		{Text: "▾ ЛИЧНЫЕ", Section: "ЛИЧНЫЕ", Header: true},
		{Text: "one", Section: "ЛИЧНЫЕ"},
		{Text: "two", Section: "ЛИЧНЫЕ"},
		{Text: "three", Section: "ЛИЧНЫЕ"},
		{Text: "four", Section: "ЛИЧНЫЕ"},
	}

	got := strings.Join(cropSidebarLines(items, 2, 3), "\n")

	if !strings.Contains(got, "two") || !strings.Contains(got, "↑ ещё 1 · личные") || !strings.Contains(got, "↓ ещё 2 · личные") {
		t.Fatalf("compact crop should keep selected row with hidden-count labels: %q", got)
	}
}
