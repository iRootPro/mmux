package app

import (
	"context"
	"testing"

	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMarkChannelReadClearsUnreadAndMentions(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "c1", Unread: 3, Mentions: 2}}}
	m.markChannelRead("c1")
	if m.channels[0].Unread != 0 || m.channels[0].Mentions != 0 {
		t.Fatalf("channel not marked read: %#v", m.channels[0])
	}
}

func TestClearThreadReadSignalReconcilesChannelUnreadAndMentions(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Unread: 3, Mentions: 1}},
		posts: []domain.Post{
			{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 2},
			{ID: "reply-1", ChannelID: "dev", RootID: "root", Unread: true},
			{ID: "reply-2", ChannelID: "dev", RootID: "root", Mentioned: true},
			{ID: "other", ChannelID: "dev", Unread: true},
		},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 2},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Unread: true},
				{ID: "reply-2", ChannelID: "dev", RootID: "root", Mentioned: true},
				{ID: "other", ChannelID: "dev", Unread: true},
			},
		},
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 2},
			{ID: "reply-1", ChannelID: "dev", RootID: "root", Unread: true},
			{ID: "reply-2", ChannelID: "dev", RootID: "root", Mentioned: true},
		},
	}

	m.clearThreadReadSignal("dev", "root")

	if m.channels[0].Unread != 1 || m.channels[0].Mentions != 0 {
		t.Fatalf("channel counters not reconciled after clearing thread read signal: %#v", m.channels[0])
	}
	for _, post := range m.posts {
		if post.ID != "other" && (post.Unread || post.Mentioned || post.ThreadUnread) {
			t.Fatalf("current posts retain thread flags after thread read: %#v", m.posts)
		}
	}
	for _, post := range m.postsByChannel["dev"] {
		if post.ID != "other" && (post.Unread || post.Mentioned || post.ThreadUnread) {
			t.Fatalf("cached posts retain thread flags after thread read: %#v", m.postsByChannel["dev"])
		}
	}
	for _, post := range m.threadPosts {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("loaded thread retains thread flags after thread read: %#v", m.threadPosts)
		}
	}
}

func TestMarkChannelReadClearsImportantFlagsAndLoadedThreadState(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Unread: 2, Mentions: 1}},
		posts: []domain.Post{
			{ID: "root", ChannelID: "dev", ThreadUnread: true},
			{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
		},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true},
				{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
			},
		},
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", ChannelID: "dev", ThreadUnread: true},
			{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
		},
	}

	m.markChannelRead("dev")

	if m.channels[0].Unread != 0 || m.channels[0].Mentions != 0 {
		t.Fatalf("channel not cleared after markChannelRead: %#v", m.channels[0])
	}
	for _, post := range m.posts {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("current posts retain important flags after channel read: %#v", m.posts)
		}
	}
	for _, post := range m.postsByChannel["dev"] {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("cached posts retain important flags after channel read: %#v", m.postsByChannel["dev"])
		}
	}
	for _, post := range m.threadPosts {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("loaded thread retains important flags after channel read: %#v", m.threadPosts)
		}
	}
}
func TestOpenCurrentChannelClearsChannelImportantStateOnce(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2, Mentions: 1}}
	m.selectedChannel = 0
	m.postsByChannel = map[string][]domain.Post{
		"dev": {
			{ID: "a", ChannelID: "dev", Unread: true},
			{ID: "b", ChannelID: "dev", Mentioned: true},
		},
	}

	updated, _ := m.openCurrentChannel()
	got := updated.(Model)
	if got.channels[0].Unread != 0 || got.channels[0].Mentions != 0 {
		t.Fatalf("openCurrentChannel did not clear important state: %#v", got.channels[0])
	}
	for _, post := range got.postsByChannel["dev"] {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("cached posts retain important flags after openCurrentChannel: %#v", got.postsByChannel["dev"])
		}
	}

	updated, _ = got.Update(postsLoadedMsg{channelID: "dev", posts: []domain.Post{
		{ID: "a", ChannelID: "dev", Unread: true},
		{ID: "b", ChannelID: "dev", Mentioned: true},
	}})
	got = updated.(Model)
	if got.channels[0].Unread != 0 || got.channels[0].Mentions != 0 {
		t.Fatalf("postsLoadedMsg did not preserve cleared state: %#v", got.channels[0])
	}
	for _, post := range got.postsByChannel["dev"] {
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("loaded posts retain important flags after channel open: %#v", got.postsByChannel["dev"])
		}
	}
}

func TestOpenSelectedThreadClearsThreadImportantState(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2, Mentions: 1}}
	m.selectedChannel = 0
	m.selectedPost = 1
	m.posts = []domain.Post{
		{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
		{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
		{ID: "other", ChannelID: "dev", Unread: true},
	}
	m.postsByChannel = map[string][]domain.Post{
		"dev": {
			{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
			{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
			{ID: "other", ChannelID: "dev", Unread: true},
		},
	}

	updated, _ := m.openSelectedThread()
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root" {
		t.Fatalf("thread not opened: threadOpen=%v root=%q", got.threadOpen, got.threadRootID)
	}
	if got.channels[0].Unread != 1 || got.channels[0].Mentions != 0 {
		t.Fatalf("thread open did not clear same work from channel counters: %#v", got.channels[0])
	}
	for _, post := range got.postsByChannel["dev"] {
		if post.ID == "other" {
			if !post.Unread {
				t.Fatalf("unrelated unread should remain after thread open: %#v", got.postsByChannel["dev"])
			}
			continue
		}
		if post.Unread || post.Mentioned || post.ThreadUnread {
			t.Fatalf("opened thread retained important flags: %#v", got.postsByChannel["dev"])
		}
	}
}

func TestLivePostInCurrentChannelSendsViewChannel(t *testing.T) {
	backend := &viewRecordingBackend{}
	m := Model{
		backend:         backend,
		ctx:             context.Background(),
		events:          make(chan domain.Event),
		channels:        []domain.Channel{{ID: "c1", Unread: 3, Mentions: 1}},
		selectedChannel: 0,
		postsByChannel:  map[string][]domain.Post{},
		selectedPost:    -1,
	}
	updated, cmd := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "p1", ChannelID: "c1", UserID: "u2", Message: "hi"}}})
	got := updated.(Model)
	if got.channels[0].Unread != 0 || got.channels[0].Mentions != 0 {
		t.Fatalf("current channel not cleared locally: %#v", got.channels[0])
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected wait+view batch, got %#v", batch)
	}
	msg := batch[1]()
	viewed, ok := msg.(channelViewedMsg)
	if !ok || viewed.channelID != "c1" || viewed.err != nil {
		t.Fatalf("bad viewed msg: %#v", msg)
	}
	if backend.viewed != "c1" {
		t.Fatalf("backend viewed = %q", backend.viewed)
	}
}

type viewRecordingBackend struct{ viewed string }

func (b *viewRecordingBackend) Connect(context.Context) (*domain.Session, error) { return nil, nil }
func (b *viewRecordingBackend) LoadChannels(context.Context, string) ([]domain.Channel, error) {
	return nil, nil
}
func (b *viewRecordingBackend) LoadPosts(context.Context, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *viewRecordingBackend) LoadPostsBefore(context.Context, string, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *viewRecordingBackend) ViewChannel(_ context.Context, channelID string) error {
	b.viewed = channelID
	return nil
}
func (b *viewRecordingBackend) LoadThread(context.Context, string) ([]domain.Post, error) {
	return nil, nil
}
func (b *viewRecordingBackend) SendPost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *viewRecordingBackend) SendReply(context.Context, string, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *viewRecordingBackend) WatchPosts(context.Context, chan<- domain.Event) error { return nil }
func (b *viewRecordingBackend) Close() error                                          { return nil }
