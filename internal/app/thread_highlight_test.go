package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"

	"github.com/charmbracelet/x/ansi"
)

func TestTimelineUnreadThreadUsesChipAndBackground(t *testing.T) {
	m := Model{posts: []domain.Post{{ID: "root", ChannelID: "dev", Username: "Alice", Message: "root", ReplyCount: 4, ThreadUnread: true}}}
	got, _ := m.renderPosts()
	plain := stripANSI(got)
	if !strings.Contains(plain, "↳4 new") {
		t.Fatalf("unread thread chip missing:\n%s", got)
	}
}

func TestSelectedUnreadThreadStatusHint(t *testing.T) {
	m := Model{focus: focusTimeline, selectedPost: 0, posts: []domain.Post{{ID: "root", ChannelID: "dev", ThreadUnread: true}}}
	got := m.renderStatus(120)
	if !strings.Contains(got, "unread thread") || !strings.Contains(got, "t open") {
		t.Fatalf("status should hint opening unread thread: %q", got)
	}
}

func TestThreadViewShowsNewRepliesSeparator(t *testing.T) {
	m := Model{threadRootID: "root", threadPosts: []domain.Post{
		{ID: "root", ChannelID: "dev", Username: "Alice", Message: "root"},
		{ID: "r1", ChannelID: "dev", RootID: "root", Username: "Bob", Message: "old"},
		{ID: "r2", ChannelID: "dev", RootID: "root", Username: "Bob", Message: "new", Unread: true},
	}}
	got := stripANSI(m.renderThreadPosts(80))
	if !strings.Contains(got, "new replies") {
		t.Fatalf("thread should show new replies separator:\n%s", got)
	}
	if strings.Index(got, "new replies") > strings.Index(got, "new") {
		t.Fatalf("separator should appear before unread reply:\n%s", got)
	}
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}
