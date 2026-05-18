package app

import (
	"testing"

	"band-tui/internal/domain"
)

func TestRecordActivityOnlyMentions(t *testing.T) {
	m := Model{session: &domain.Session{User: domain.User{ID: "me", Username: "alex"}}}
	m.recordActivity(domain.Post{ID: "p1", UserID: "me", Message: "@alex own"})
	if len(m.recentEvents) != 0 {
		t.Fatalf("own message recorded: %#v", m.recentEvents)
	}
	m.recordActivity(domain.Post{ID: "p2", UserID: "u2", Message: "hello"})
	if len(m.recentEvents) != 0 {
		t.Fatalf("non-mention recorded: %#v", m.recentEvents)
	}
	m.recordActivity(domain.Post{ID: "p3", UserID: "u2", Message: "ping @alex"})
	m.recordActivity(domain.Post{ID: "p4", UserID: "u3", Message: "@channel deploy"})
	m.recordActivity(domain.Post{ID: "p5", UserID: "u4", Message: "display-name mention", Mentioned: true})
	if len(m.recentEvents) != 3 || m.recentEvents[0].ID != "p5" || m.recentEvents[1].ID != "p4" || m.recentEvents[2].ID != "p3" {
		t.Fatalf("mentions not recorded: %#v", m.recentEvents)
	}
}

func TestBumpUnreadDoesNotMarkDMAsMention(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D"}}}
	m.bumpUnread("d1")
	if m.channels[0].Unread != 1 || m.channels[0].Mentions != 0 {
		t.Fatalf("bad unread/mention: %#v", m.channels[0])
	}
}

func TestActivityItemsIncludeMentionChannelsWithoutLiveEvents(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "c1", Type: "O", Unread: 3}, {ID: "c2", Type: "O", Mentions: 2}}}
	items := m.activityItems()
	if len(items) != 1 || items[0].ChannelID != "c2" || items[0].HasPost {
		t.Fatalf("bad activity items: %#v", items)
	}
}

func TestInitialSelectedPostPrefersUnreadOrPendingJump(t *testing.T) {
	m := Model{posts: []domain.Post{{ID: "p1"}, {ID: "p2", ThreadUnread: true}, {ID: "p3", Unread: true}}}
	if got := m.initialSelectedPost("c1"); got != 1 {
		t.Fatalf("selected unread = %d", got)
	}
	m.pendingJumpChannelID = "c1"
	m.pendingJumpPostID = "p3"
	if got := m.initialSelectedPost("c1"); got != 2 {
		t.Fatalf("selected pending = %d", got)
	}
}
