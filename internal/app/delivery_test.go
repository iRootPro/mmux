package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestChannelSendShowsPendingThenDelivered(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me", Username: "me"}}
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("hello")

	updated, _ := m.handleKey(draftKey("enter"))
	got := updated.(Model)
	if len(got.posts) != 1 || got.posts[0].Delivery != domain.DeliveryPending || !strings.HasPrefix(got.posts[0].ID, "pending:") {
		t.Fatalf("pending post not shown: %#v", got.posts)
	}
	if rendered, _ := got.renderPosts(); !strings.Contains(rendered, "sending") {
		t.Fatalf("pending delivery badge missing:\n%s", rendered)
	}
	pendingID := pendingID(got.pendingSends)
	updated, _ = got.Update(postSentMsg{channelID: "dev", draftKey: channelDraftKey("dev"), pendingID: pendingID, text: "hello", post: domain.Post{ID: "p1", ChannelID: "dev", UserID: "me", Message: "hello"}})
	got = updated.(Model)
	if len(got.posts) != 1 || got.posts[0].ID != "p1" || got.posts[0].Delivery != domain.DeliverySent {
		t.Fatalf("pending post not replaced with delivered post: %#v", got.posts)
	}
	if rendered, _ := got.renderPosts(); !strings.Contains(rendered, "✓") || strings.Contains(rendered, "sending") {
		t.Fatalf("delivered badge missing or pending left behind:\n%s", rendered)
	}
}

func TestFailedChannelSendMarksMessageAndRestoresDraft(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me", Username: "me"}}
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("hello")

	updated, _ := m.handleKey(draftKey("enter"))
	got := updated.(Model)
	pendingID := pendingID(got.pendingSends)
	updated, _ = got.Update(postSentMsg{channelID: "dev", draftKey: channelDraftKey("dev"), pendingID: pendingID, text: "hello", err: assertErr{}})
	got = updated.(Model)
	if len(got.posts) != 1 || got.posts[0].Delivery != domain.DeliveryFailed {
		t.Fatalf("failed send should mark pending message failed: %#v", got.posts)
	}
	if got.composer.Value() != "hello" {
		t.Fatalf("failed send should restore draft, got %q", got.composer.Value())
	}
	if rendered, _ := got.renderPosts(); !strings.Contains(rendered, "failed") {
		t.Fatalf("failed delivery badge missing:\n%s", rendered)
	}
}

func TestThreadReplyShowsPendingThenDelivered(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me", Username: "me"}}
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.focus = focusComposer
	m.threadPosts = []domain.Post{{ID: "root", ChannelID: "dev", Message: "root"}}
	m.loadDraft(threadDraftKey("dev", "root"))
	m.composer.SetValue("reply")

	updated, _ := m.handleThreadKey(draftKey("enter"))
	got := updated.(Model)
	if len(got.threadPosts) != 2 || got.threadPosts[1].Delivery != domain.DeliveryPending {
		t.Fatalf("pending reply not shown: %#v", got.threadPosts)
	}
	pendingID := pendingID(got.pendingSends)
	updated, _ = got.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadDraftKey("dev", "root"), pendingID: pendingID, text: "reply", post: domain.Post{ID: "r1", ChannelID: "dev", RootID: "root", UserID: "me", Message: "reply"}})
	got = updated.(Model)
	if len(got.threadPosts) != 2 || got.threadPosts[1].ID != "r1" || got.threadPosts[1].Delivery != domain.DeliverySent {
		t.Fatalf("pending reply not replaced: %#v", got.threadPosts)
	}
}

func TestDeliveryReadRendersDoubleCheck(t *testing.T) {
	m := Model{posts: []domain.Post{{ID: "p1", ChannelID: "d1", UserID: "me", Username: "me", Message: "hello", Delivery: domain.DeliveryRead}}}
	got, _ := m.renderPosts()
	if !strings.Contains(got, "✓✓") {
		t.Fatalf("read delivery badge missing:\n%s", got)
	}
}

func TestReplySentAfterWebsocketEchoDoesNotDuplicatePendingReply(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me", Username: "me"}}
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.focus = focusComposer
	m.threadPosts = []domain.Post{{ID: "root", ChannelID: "dev", Message: "root"}}
	m.postsByChannel = map[string][]domain.Post{"dev": {{ID: "root", ChannelID: "dev", Message: "root"}}}
	m.loadDraft(threadDraftKey("dev", "root"))
	m.composer.SetValue("reply")

	updated, _ := m.handleThreadKey(draftKey("enter"))
	got := updated.(Model)
	pendingID := pendingID(got.pendingSends)
	realPost := domain.Post{ID: "r1", ChannelID: "dev", RootID: "root", UserID: "me", Message: "reply"}
	updated, _ = got.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: realPost}})
	got = updated.(Model)
	updated, _ = got.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadDraftKey("dev", "root"), pendingID: pendingID, text: "reply", post: realPost})
	got = updated.(Model)

	count := 0
	for _, post := range got.threadPosts {
		if post.ID == "r1" {
			count++
		}
		if strings.HasPrefix(post.ID, "pending:") {
			t.Fatalf("pending reply left after ack: %#v", got.threadPosts)
		}
	}
	if count != 1 {
		t.Fatalf("reply duplicated after websocket echo + ack: %#v", got.threadPosts)
	}
}
