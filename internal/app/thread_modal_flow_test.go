package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOpenSelectedThreadStartsInReplyComposer(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.selectedPost = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", ReplyCount: 2}}

	updated, _ := m.openSelectedThread()
	got := updated.(Model)
	if !got.threadOpen || !got.threadFocusComposer || got.focus != focusComposer {
		t.Fatalf("thread should open as reply modal: open=%v composer=%v focus=%v", got.threadOpen, got.threadFocusComposer, got.focus)
	}
}

func TestTimelineROpensThreadReplyModal(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusTimeline
	m.selectedPost = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", Message: "root"}}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root" || !got.threadFocusComposer {
		t.Fatalf("r should open thread reply modal: open=%v root=%q composer=%v", got.threadOpen, got.threadRootID, got.threadFocusComposer)
	}
}

func TestTimelineEnterOpensThreadWhenSelectedPostHasReplies(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusTimeline
	m.selectedPost = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", ReplyCount: 1}}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root" || !got.threadFocusComposer {
		t.Fatalf("enter should open thread modal for reply roots: open=%v root=%q composer=%v", got.threadOpen, got.threadRootID, got.threadFocusComposer)
	}
}

func TestThreadViewRendersModalWithComposerHint(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.width = 120
	m.height = 40
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.threadPosts = []domain.Post{{ID: "root", ChannelID: "dev", Username: "Alice", Message: "root"}}
	m.refreshThreadViewport()

	got := m.View()
	if !strings.Contains(got, "Thread") || !strings.Contains(got, "enter reply") || !strings.Contains(got, "Write a reply") || !strings.Contains(got, "thread reply") {
		t.Fatalf("thread modal missing expected chat hints:\n%s", got)
	}
}

func TestEscFromThreadModalReturnsToTimelineSelection(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusTimeline
	m.selectedPost = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", ReplyCount: 1}, {ID: "other", ChannelID: "dev"}}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.threadOpen || got.focus != focusTimeline || got.selectedPost != 0 {
		t.Fatalf("esc should close modal back to timeline selection: open=%v focus=%v selected=%d", got.threadOpen, got.focus, got.selectedPost)
	}
}

func TestNInThreadModalOpensNextUnreadThread(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2}}
	m.selectedChannel = 0
	m.postsByChannel = map[string][]domain.Post{"dev": {
		{ID: "root-1", ChannelID: "dev", ThreadUnread: true, CreateAt: 100},
		{ID: "root-2", ChannelID: "dev", ThreadUnread: true, CreateAt: 200},
	}}
	m.threadOpen = true
	m.threadRootID = "root-2"
	m.threadFocusComposer = true

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root-1" || !got.threadFocusComposer {
		t.Fatalf("n should open next unread thread by triage order, root=%q open=%v composer=%v", got.threadRootID, got.threadOpen, got.threadFocusComposer)
	}
}
