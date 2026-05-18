package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestRenderPostsShowsNewMessagesSeparator(t *testing.T) {
	m := Model{
		posts: []domain.Post{
			{ID: "p1", Username: "Alice", Message: "old"},
			{ID: "p2", Username: "Bob", Message: "new", Unread: true},
		},
		selectedPost: 1,
	}
	got, _ := m.renderPosts()
	if !strings.Contains(got, "new messages") {
		t.Fatalf("missing separator: %q", got)
	}
}

func TestInitialSelectedPostPriority(t *testing.T) {
	m := Model{posts: []domain.Post{
		{ID: "p1", ThreadUnread: true},
		{ID: "p2", Unread: true},
		{ID: "p3", Mentioned: true},
	}}
	if got := m.initialSelectedPost("c1"); got != 2 {
		t.Fatalf("selected = %d, want mention index 2", got)
	}
}

func TestInitialSelectedPostAndUnreadNavigationUseSamePriority(t *testing.T) {
	m := Model{posts: []domain.Post{
		{ID: "p1"},
		{ID: "p2", ThreadUnread: true},
		{ID: "p3", Unread: true},
		{ID: "p4", Mentioned: true},
	}}

	initial := m.initialSelectedPost("c1")
	if initial != 3 {
		t.Fatalf("initial selected = %d, want mention index 3", initial)
	}

	updated, _ := m.selectRelativeImportantPost(1)
	got := updated.(Model)
	if got.selectedPost != initial {
		t.Fatalf("unread navigation selected = %d, want same priority target as initialSelectedPost (%d)", got.selectedPost, initial)
	}
}
func TestSelectRelativeImportantPostUsesSamePriorityAsInitialSelection(t *testing.T) {
	m := Model{
		selectedPost: 1,
		posts: []domain.Post{
			{ID: "p1"},
			{ID: "p2", ThreadUnread: true},
			{ID: "p3", Unread: true},
			{ID: "p4", Mentioned: true},
		},
	}

	initial := m.initialSelectedPost("c1")
	if initial != 3 {
		t.Fatalf("initial selected = %d, want mention index 3", initial)
	}

	updated, _ := m.selectRelativeImportantPost(1)
	got := updated.(Model)
	if got.selectedPost != initial {
		t.Fatalf("unread navigation selected = %d, want same priority target as initialSelectedPost (%d)", got.selectedPost, initial)
	}
}

func TestInitialSelectionPrefersThreadUnreadOverPlainUnread(t *testing.T) {
	m := Model{posts: []domain.Post{
		{ID: "thread-root", ThreadUnread: true},
		{ID: "plain-unread", Unread: true},
	}}
	if got := m.initialSelectedPost("c1"); got != 0 {
		t.Fatalf("initial selected = %d, want thread-unread index 0", got)
	}
}
