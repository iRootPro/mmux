package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestRenderPostsGroupsConsecutiveMessagesFromSameAuthor(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		posts: []domain.Post{
			{ID: "p1", UserID: "u1", Username: "Alice", Message: "first", CreateAt: base},
			{ID: "p2", UserID: "u1", Username: "Alice", Message: "second", CreateAt: base + 60_000},
			{ID: "p3", UserID: "u2", Username: "Bob", Message: "third", CreateAt: base + 120_000},
		},
		selectedPost: -1,
	}

	got, offsets := m.renderPosts()

	if strings.Count(got, "Alice") != 1 {
		t.Fatalf("Alice header count = %d, want 1 in:\n%s", strings.Count(got, "Alice"), got)
	}
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") || !strings.Contains(got, "Bob") {
		t.Fatalf("grouped timeline lost content:\n%s", got)
	}
	if strings.Contains(got, "first\n\n  second") {
		t.Fatalf("grouped messages should not have a blank line inside the group:\n%s", got)
	}
	if !strings.Contains(got, "second\n\n") {
		t.Fatalf("different message groups should keep a blank separator:\n%s", got)
	}
	if len(offsets) != len(m.posts) || offsets[1] <= offsets[0] || offsets[2] <= offsets[1] {
		t.Fatalf("bad post offsets: %#v", offsets)
	}
}

func TestRenderPostsDoesNotGroupImportantMessages(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		posts: []domain.Post{
			{ID: "p1", UserID: "u1", Username: "Alice", Message: "old", CreateAt: base},
			{ID: "p2", UserID: "u1", Username: "Alice", Message: "new", CreateAt: base + 60_000, Unread: true},
			{ID: "p3", UserID: "u1", Username: "Alice", Message: "reply root", CreateAt: base + 120_000, ReplyCount: 2},
		},
		selectedPost: 1,
	}

	got, _ := m.renderPosts()

	if strings.Count(got, "Alice") != 3 {
		t.Fatalf("important messages should keep headers, got:\n%s", got)
	}
	if !strings.Contains(got, "new messages") || !strings.Contains(got, "↳ 2 replies") {
		t.Fatalf("important indicators missing:\n%s", got)
	}
}

func TestRenderPostsDoesNotGroupMentionedOrThreadUnreadMessages(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		posts: []domain.Post{
			{ID: "p1", UserID: "u1", Username: "Alice", Message: "old", CreateAt: base},
			{ID: "p2", UserID: "u1", Username: "Alice", Message: "mentioned", CreateAt: base + 60_000, Mentioned: true},
			{ID: "p3", UserID: "u1", Username: "Alice", Message: "thread unread", CreateAt: base + 120_000, ThreadUnread: true},
		},
	}

	got, _ := m.renderPosts()

	if strings.Count(got, "Alice") != 3 {
		t.Fatalf("mentioned/thread-unread messages should keep headers, got:\n%s", got)
	}
	if strings.Count(got, "●") < 2 {
		t.Fatalf("important message markers missing:\n%s", got)
	}
}
