package app

import (
	"strings"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	"band-tui/internal/mock"
)

func TestThreadHeaderShowsRootPreviewAndReplyCount(t *testing.T) {
	m := Model{
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", Username: "Alice", Message: "Root message text"},
			{ID: "r1", RootID: "root", Username: "Bob", Message: "reply"},
			{ID: "r2", RootID: "root", Username: "Alice", Message: "reply 2"},
		},
	}
	got := m.renderThreadHeader(80)
	if !strings.HasPrefix(got, "\n") {
		t.Fatalf("thread header should reserve first row: %q", got)
	}
	for _, want := range []string{"Thread · 2 replies", "root: Alice · Root message text", "tab reply"} {
		if !strings.Contains(got, want) {
			t.Fatalf("header missing %q in %q", want, got)
		}
	}
}

func TestRenderThreadPostsIncludesSelectableRoot(t *testing.T) {
	m := Model{
		threadRootID:   "root",
		threadSelected: 0,
		threadPosts: []domain.Post{
			{ID: "root", Username: "Alice", Message: "Root message text"},
			{ID: "r1", RootID: "root", Username: "Bob", Message: "reply"},
		},
	}
	got := m.renderThreadPosts(80)
	if !strings.Contains(got, "Root message text") || !strings.Contains(got, "reply") || !strings.Contains(got, "┃ ") {
		t.Fatalf("thread posts = %q", got)
	}
}

func TestThreadComposerLabel(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.threadRootID = "root"
	m.threadPosts = []domain.Post{{ID: "root", Username: "Alice", Message: "Root message text"}}
	m.threadFocusComposer = true
	got := m.renderThreadComposer(80)
	if !strings.Contains(got, "reply to: Alice") || !strings.Contains(got, "Root message text") || !strings.Contains(got, "Write a reply…") {
		t.Fatalf("composer = %q", got)
	}
	if strings.Contains(got, "Write a message…") {
		t.Fatalf("thread composer should use reply placeholder: %q", got)
	}
}

func TestThreadComposerShowsInactiveStateOutsideReplyFocus(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.threadRootID = "root"
	m.threadPosts = []domain.Post{{ID: "root", Username: "Alice", Message: "Root message text"}}
	m.threadFocusComposer = false

	got := m.renderThreadComposer(80)

	if !strings.Contains(got, "reply composer inactive") || !strings.Contains(got, "tab reply") {
		t.Fatalf("inactive thread composer label missing: %q", got)
	}
	if strings.Count(got, "reply composer inactive") != 1 || strings.Contains(got, "Reply composer inactive") || strings.Contains(got, "reply to: Alice") || strings.Contains(got, "enter send") || strings.Contains(got, "Write a reply…") {
		t.Fatalf("inactive thread composer should show one inactive label and no active/inactive placeholder: %q", got)
	}
}

func TestRenderThreadPostsGroupsConsecutiveReplies(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", Username: "Alice", Message: "Root"},
			{ID: "r1", RootID: "root", UserID: "u2", Username: "Bob", Message: "one", CreateAt: base},
			{ID: "r2", RootID: "root", UserID: "u2", Username: "Bob", Message: "two", CreateAt: base + 60_000},
			{ID: "r3", RootID: "root", UserID: "u3", Username: "Alice", Message: "three", CreateAt: base + 120_000},
		},
	}

	got := m.renderThreadPosts(80)

	if strings.Count(got, "Bob") != 1 {
		t.Fatalf("thread replies should group same author, got:\n%s", got)
	}
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") || !strings.Contains(got, "Alice") || !strings.Contains(got, "three") {
		t.Fatalf("thread grouping lost content:\n%s", got)
	}
	if strings.Contains(got, "one\n\n  two") {
		t.Fatalf("grouped thread replies should not have a blank line inside the group:\n%s", got)
	}
	if !strings.Contains(got, "two\n\n") {
		t.Fatalf("different thread reply groups should keep a blank separator:\n%s", got)
	}
}

func TestRenderThreadPostsDoesNotGroupImportantReplies(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", Username: "Alice", Message: "Root"},
			{ID: "r1", RootID: "root", UserID: "u2", Username: "Bob", Message: "one", CreateAt: base},
			{ID: "r2", RootID: "root", UserID: "u2", Username: "Bob", Message: "unread", CreateAt: base + 60_000, Unread: true},
			{ID: "r3", RootID: "root", UserID: "u2", Username: "Bob", Message: "mentioned", CreateAt: base + 120_000, Mentioned: true},
			{ID: "r4", RootID: "root", UserID: "u2", Username: "Bob", Message: "thread unread", CreateAt: base + 180_000, ThreadUnread: true},
			{ID: "r5", RootID: "root", UserID: "u2", Username: "Bob", Message: "reply count stays hidden", CreateAt: base + 240_000, ReplyCount: 2},
		},
	}

	got := m.renderThreadPosts(80)

	if strings.Count(got, "Bob") != 5 {
		t.Fatalf("important thread replies should keep headers, got:\n%s", got)
	}
	for _, want := range []string{"unread", "mentioned", "thread unread", "reply count stays hidden"} {
		if !strings.Contains(got, want) {
			t.Fatalf("important thread reply missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, "●") < 3 {
		t.Fatalf("important thread reply markers missing:\n%s", got)
	}
	if strings.Contains(got, "↳ 2 replies") {
		t.Fatalf("thread reply headers should not show nested reply counts:\n%s", got)
	}
}
