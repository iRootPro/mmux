package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestNotificationBadge(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "c1", Unread: 2},
			{ID: "d1", Unread: 1, Mentions: 1},
		},
		recentEvents: []domain.Post{{ID: "p1"}, {ID: "p2"}},
	}
	got := m.notificationBadge()
	for _, want := range []string{"mentions 2", "@1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("badge %q missing %q", got, want)
		}
	}
}

func TestActivityStatus(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Alice"}}}
	got := m.activityStatus(domain.Post{ChannelID: "d1", Username: "Bob", Message: "hi"})
	if got != "new message: @Alice · Bob" {
		t.Fatalf("status = %q", got)
	}
}
