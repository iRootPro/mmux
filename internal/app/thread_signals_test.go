package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestThreadSignalKeepsUnreadWhenRootIsNotLoaded(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}},
		threadSignalsByChannel: map[string][]domain.ThreadSignal{
			"dev": {{ChannelID: "dev", RootID: "root-old", PostID: "r1", Actor: "Alice", Preview: "new hidden reply", CreateAt: 10, UnreadCount: 1}},
		},
		postsByChannel: map[string][]domain.Post{"dev": {{ID: "visible", ChannelID: "dev", Message: "visible"}}},
	}
	m.reconcileChannelImportance("dev")
	if m.channels[0].Unread != 1 {
		t.Fatalf("thread signal should count as unread: %#v", m.channels[0])
	}
	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageThreadReply || items[0].RootID != "root-old" || items[0].Preview != "new hidden reply" {
		t.Fatalf("thread signal should create triage item: %#v", items)
	}
}

func TestOpeningChannelDoesNotClearUnresolvedThreadSignal(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.postsByChannel = map[string][]domain.Post{"dev": {{ID: "visible", ChannelID: "dev", Unread: true}}}
	m.threadSignalsByChannel = map[string][]domain.ThreadSignal{"dev": {{ChannelID: "dev", RootID: "root-old", PostID: "r1", UnreadCount: 1}}}

	updated, _ := m.openCurrentChannel()
	got := updated.(Model)
	if len(got.threadSignalsByChannel["dev"]) != 1 {
		t.Fatalf("open channel cleared unresolved thread signal: %#v", got.threadSignalsByChannel)
	}
	if got.channels[0].Unread != 1 {
		t.Fatalf("channel unread should preserve unresolved thread signal: %#v", got.channels[0])
	}
}

func TestOpeningThreadClearsThreadSignal(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}}
	m.selectedChannel = 0
	m.selectedPost = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", ThreadUnread: true}}
	m.postsByChannel = map[string][]domain.Post{"dev": {{ID: "root", ChannelID: "dev", ThreadUnread: true}}}
	m.threadSignalsByChannel = map[string][]domain.ThreadSignal{"dev": {{ChannelID: "dev", RootID: "root", PostID: "r1", UnreadCount: 1}}}

	updated, _ := m.openSelectedThread()
	got := updated.(Model)
	if len(got.threadSignalsByChannel["dev"]) != 0 {
		t.Fatalf("thread signal not cleared after opening thread: %#v", got.threadSignalsByChannel)
	}
	if got.channels[0].Unread != 0 {
		t.Fatalf("channel unread not reconciled after thread open: %#v", got.channels[0])
	}
}

func TestSidebarHeaderAndTimelineExposeThreadSignals(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}},
		selectedChannel: 0,
		posts:           []domain.Post{{ID: "visible", ChannelID: "dev", Message: "visible"}},
		postsByChannel:  map[string][]domain.Post{"dev": {{ID: "visible", ChannelID: "dev", Message: "visible"}}},
		threadSignalsByChannel: map[string][]domain.ThreadSignal{
			"dev": {{ChannelID: "dev", RootID: "root-old", PostID: "r1", Preview: "hidden", UnreadCount: 2}},
		},
	}
	m.reconcileChannelImportance("dev")
	if got := m.renderSidebarChannelLine(0, 40); !strings.Contains(got, "↳2") {
		t.Fatalf("sidebar should show thread badge, got %q", got)
	}
	if got := m.channelMeta(m.channels[0]); !strings.Contains(got, "↳2 unread threads") {
		t.Fatalf("header meta should show unread threads, got %q", got)
	}
	posts, _ := m.renderPosts()
	if !strings.Contains(posts, "2 unread threads outside loaded timeline") {
		t.Fatalf("timeline should show hidden thread banner, got %q", posts)
	}
}
