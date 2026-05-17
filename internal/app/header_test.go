package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderHeaderShowsChannelMetaAndTopic(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town", Header: "Deploy coordination", MemberCount: 42, Mentions: 2}}, selectedChannel: 0}
	got := m.renderHeader(120)
	for _, want := range []string{"# Town", "42 members", "@2", "Deploy coordination"} {
		if !strings.Contains(got, want) {
			t.Fatalf("header %q missing %q", got, want)
		}
	}
}

func TestRenderHeaderShowsDMStatus(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Alice", Status: "online", MemberCount: 1}}, selectedChannel: 0}
	got := m.renderHeader(100)
	if !strings.Contains(got, "●") || !strings.Contains(got, "online") {
		t.Fatalf("header = %q", got)
	}
}

func TestRenderHeaderHeightIsStableWithoutTopic(t *testing.T) {
	withTopic := Model{channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town", Header: "**Important** [docs](https://example.com)", MemberCount: 42}}, selectedChannel: 0}
	withoutTopic := Model{channels: []domain.Channel{{ID: "c2", Type: "O", DisplayName: "Empty", MemberCount: 2}}, selectedChannel: 0}
	if lipgloss.Height(withTopic.renderHeader(120)) != lipgloss.Height(withoutTopic.renderHeader(120)) {
		t.Fatalf("header height differs: with=%q without=%q", withTopic.renderHeader(120), withoutTopic.renderHeader(120))
	}
	got := withTopic.renderHeader(120)
	if !strings.Contains(got, "# Town") || !strings.Contains(got, "Important") || !strings.Contains(got, "docs") || !strings.Contains(got, "https://example.com") {
		t.Fatalf("markdown topic not rendered: %q", got)
	}
}

func TestRenderHeaderKeepsTitleWithLongTopic(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Евгения Мусанова", Status: "offline", Header: "Meet <https://stream.wb.ru/room/1_1_jenya____sasha>"}}, selectedChannel: 0}
	got := m.renderHeader(100)
	if !strings.Contains(got, "@ Евгения Мусанова") || !strings.Contains(got, "Meet") {
		t.Fatalf("header lost title/topic: %q", got)
	}
}

func TestRenderHeaderDoesNotExpandHugeMarkdownTopic(t *testing.T) {
	topic := "ознакомьтесь\n```\nsigningkey = ~/.ssh/wbkey_ed25519.pub\n[core]\n    sshCommand = \"ssh -i ~/.ssh/wbkey_ed25519 -F /dev/null\"\n[commit]\n    gpgsign = true\n```\nочень длинная ссылка https://gitlab.wildberries.ru/advertising/ads/file-responder/-/jobs/129155630"
	m := Model{channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "DevSecOps", Header: topic, MemberCount: 100}}, selectedChannel: 0}
	got := m.renderHeader(120)
	if lipgloss.Height(got) != 2 {
		t.Fatalf("header expanded to %d lines: %q", lipgloss.Height(got), got)
	}
	if !strings.Contains(got, "# DevSecOps") || !strings.Contains(got, "ознакомьтесь") {
		t.Fatalf("header lost title/topic: %q", got)
	}
}

func TestRenderHeaderShowsLoadedMessageCount(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town", MemberCount: 42}},
		posts:           []domain.Post{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}},
		selectedChannel: 0,
	}

	got := m.renderHeader(120)

	if !strings.Contains(got, "42 members") || !strings.Contains(got, "3 messages") {
		t.Fatalf("header missing metadata: %q", got)
	}
}

func TestRenderHeaderKeepsDMStatusAndMessageCount(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Alice", Status: "offline"}},
		posts:           []domain.Post{{ID: "p1"}},
		selectedChannel: 0,
	}

	got := m.renderHeader(120)

	if !strings.Contains(got, "offline") || !strings.Contains(got, "1 message") {
		t.Fatalf("DM header missing status/count: %q", got)
	}
}
