package app

import (
	"context"
	"strings"
	"testing"

	"band-tui/internal/domain"
)

type noopBackend struct{}

func (noopBackend) Connect(context.Context) (*domain.Session, error)               { return nil, nil }
func (noopBackend) LoadChannels(context.Context, string) ([]domain.Channel, error) { return nil, nil }
func (noopBackend) LoadPosts(context.Context, string, int) ([]domain.Post, error)  { return nil, nil }
func (noopBackend) LoadPostsBefore(context.Context, string, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (noopBackend) ViewChannel(context.Context, string) error                 { return nil }
func (noopBackend) LoadThread(context.Context, string) ([]domain.Post, error) { return nil, nil }
func (noopBackend) SendPost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (noopBackend) SendReply(context.Context, string, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (noopBackend) UpdatePost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (noopBackend) DeletePost(context.Context, string) error              { return nil }
func (noopBackend) WatchPosts(context.Context, chan<- domain.Event) error { return nil }
func (noopBackend) Close() error                                          { return nil }

func TestPostSentMsgWithEmptyPostDoesNotAppendMessage(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.posts = []domain.Post{{ID: "existing", ChannelID: "dev", Message: "existing"}}
	m.loadDraft(channelDraftKey("dev"))

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: channelDraftKey("dev"), post: domain.Post{}})
	got := updated.(Model)
	if len(got.posts) != 1 || got.posts[0].ID != "existing" {
		t.Fatalf("empty control post should not append, got %#v", got.posts)
	}
}

func TestReplySentMsgWithEmptyPostDoesNotAppendMessage(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadPosts = []domain.Post{{ID: "root", ChannelID: "dev", Message: "root"}}

	updated, _ := m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadDraftKey("dev", "root"), post: domain.Post{}})
	got := updated.(Model)
	if len(got.threadPosts) != 1 || got.threadPosts[0].ID != "root" {
		t.Fatalf("empty control reply should not append, got %#v", got.threadPosts)
	}
}

func TestBackendEventStateUpdatesStatus(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventState, State: domain.ConnectionOffline, Message: "showing cached messages"}})
	got := updated.(Model)
	if got.connectionState != domain.ConnectionOffline {
		t.Fatalf("state event should update explicit connection state, got %q", got.connectionState)
	}
	if rendered := got.renderStatus(120); !strings.Contains(rendered, "offline") {
		t.Fatalf("rendered status should surface connection state, got %q", rendered)
	}
}
