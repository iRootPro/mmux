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
