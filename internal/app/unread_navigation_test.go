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

func TestSelectRelativeImportantPost(t *testing.T) {
	m := Model{posts: []domain.Post{{ID: "p1"}, {ID: "p2", Unread: true}, {ID: "p3"}, {ID: "p4", Mentioned: true}}, selectedPost: 0}
	updated, _ := m.selectRelativeImportantPost(1)
	got := updated.(Model)
	if got.selectedPost != 1 {
		t.Fatalf("selected = %d", got.selectedPost)
	}
	updated, _ = got.selectRelativeImportantPost(1)
	got = updated.(Model)
	if got.selectedPost != 3 {
		t.Fatalf("selected = %d", got.selectedPost)
	}
	updated, _ = got.selectRelativeImportantPost(-1)
	got = updated.(Model)
	if got.selectedPost != 1 {
		t.Fatalf("selected previous = %d", got.selectedPost)
	}
}
