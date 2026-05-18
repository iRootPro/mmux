package app

import (
	"testing"
	"time"

	"band-tui/internal/domain"
)

func TestBuildTriageItemsOrdersMentionThreadUnread(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	m := Model{
		channels: []domain.Channel{
			{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3},
			{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1},
		},
		recentEvents: []domain.Post{{ID: "p1", ChannelID: "dev", Username: "Artyom", Message: "Need help", Mentioned: true, CreateAt: now.Add(-2 * time.Minute).UnixMilli()}},
		postsByChannel: map[string][]domain.Post{
			"alerts": {{ID: "a1", ChannelID: "alerts", Username: "bot", Message: "3 unread", Unread: true, CreateAt: now.Add(-10 * time.Minute).UnixMilli()}},
			"dev":    {{ID: "t1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "new reply", ThreadUnread: true, CreateAt: now.Add(-5 * time.Minute).UnixMilli()}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 3 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if items[0].Kind != triageMention || items[1].Kind != triageThreadReply || items[2].Kind != triageUnreadChannel {
		t.Fatalf("unexpected order: %#v", items)
	}
}

func TestBuildTriageItemsPrefersMentionOverWeakerUnreadDuplicate(t *testing.T) {
	m := Model{
		channels:     []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1, Unread: 4}},
		recentEvents: []domain.Post{{ID: "p1", ChannelID: "dev", Username: "Artyom", Message: "Need help", Mentioned: true, CreateAt: 100}},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageMention {
		t.Fatalf("expected one mention item, got %#v", items)
	}
	if items[0].PostID != "p1" || items[0].Actor != "Artyom" || items[0].Preview != "Need help" || items[0].MentionCount != 1 {
		t.Fatalf("expected active mention to be enriched by matching recent event, got %#v", items[0])
	}
}

func TestBuildTriageItemsDropsHistoricalMentionWhenChannelMentionsCleared(t *testing.T) {
	m := Model{
		channels:     []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 0, Unread: 0}},
		recentEvents: []domain.Post{{ID: "p1", ChannelID: "dev", Username: "Artyom", Message: "Need help", Mentioned: true, CreateAt: 100}},
	}

	items := buildTriageItems(m)
	if len(items) != 0 {
		t.Fatalf("expected historical mention to be dropped after channel mentions cleared, got %#v", items)
	}
}

func TestBuildTriageItemsFallsBackToChannelLevelMentionWithoutRecentEvent(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 2, Unread: 5, LastPostAt: 100}},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageMention {
		t.Fatalf("expected one channel-level mention item, got %#v", items)
	}
	item := items[0]
	if item.PostID != "" || item.Actor != "" || item.Preview != "" || item.MentionCount != 2 || item.UnreadCount != 5 || item.CreateAt != 100 {
		t.Fatalf("expected channel-level mention signal, got %#v", item)
	}
}

func TestBuildTriageItemsUsesNewestKnownReplyForRootMarkedThreadUnread(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 2},
				{ID: "reply-old", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "older reply", CreateAt: 200},
				{ID: "reply-new", ChannelID: "dev", RootID: "root", Username: "Ola", Message: "newer reply", CreateAt: 300},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	item := items[0]
	if item.Kind != triageThreadReply {
		t.Fatalf("Kind = %v, want %v", item.Kind, triageThreadReply)
	}
	if item.RootID != "root" {
		t.Fatalf("RootID = %q, want root", item.RootID)
	}
	if item.PostID != "reply-new" {
		t.Fatalf("PostID = %q, want newest known reply", item.PostID)
	}
	if item.CreateAt != 300 {
		t.Fatalf("CreateAt = %d, want newest known reply timestamp", item.CreateAt)
	}
}

func TestBuildTriageItemsFallsBackToRootWhenRootMarkedThreadUnreadHasNoKnownReply(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 2}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	if items[0].PostID != "root" || items[0].CreateAt != 100 {
		t.Fatalf("item should fall back to root post identity and freshness, got %#v", items[0])
	}
}

func TestBuildTriageItemsOrdersRootMarkedThreadUnreadByNewestKnownReply(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "alpha", Type: "O", DisplayName: "alpha", Unread: 1},
			{ID: "beta", Type: "O", DisplayName: "beta", Unread: 1},
		},
		postsByChannel: map[string][]domain.Post{
			"alpha": {
				{ID: "alpha-root", ChannelID: "alpha", ThreadUnread: true, CreateAt: 100, ReplyCount: 1},
				{ID: "alpha-reply", ChannelID: "alpha", RootID: "alpha-root", CreateAt: 500},
			},
			"beta": {
				{ID: "beta-root", ChannelID: "beta", ThreadUnread: true, CreateAt: 400, ReplyCount: 1},
				{ID: "beta-reply", ChannelID: "beta", RootID: "beta-root", CreateAt: 450},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %#v", len(items), items)
	}
	if items[0].PostID != "alpha-reply" || items[1].PostID != "beta-reply" {
		t.Fatalf("thread items should sort by newest known reply, got %#v", items)
	}
}

func TestTriageDismissKeyChangesWhenRootMarkedThreadUnreadGetsNewerKnownReply(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true, CreateAt: 100, ReplyCount: 1},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", CreateAt: 200},
			},
		},
	}
	items := buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	before := triageDismissKey(items[0])

	m.postsByChannel["dev"][0].ReplyCount = 2
	m.postsByChannel["dev"] = append(m.postsByChannel["dev"], domain.Post{ID: "reply-2", ChannelID: "dev", RootID: "root", CreateAt: 300})
	items = buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	if items[0].PostID != "reply-2" {
		t.Fatalf("PostID = %q, want newer reply", items[0].PostID)
	}
	if before == triageDismissKey(items[0]) {
		t.Fatal("dismiss key should change when root-marked thread unread gets a newer known reply")
	}
}

func TestBuildTriageItemsMergesMatchingLoadedThreadForRootMarkedUnread(t *testing.T) {
	m := Model{
		channels:     []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", CreateAt: 100},
			{ID: "reply-old", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "older reply", CreateAt: 200},
			{ID: "reply-new", ChannelID: "dev", RootID: "root", Username: "Ola", Message: "newer reply", CreateAt: 300},
		},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 2}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	item := items[0]
	if item.Kind != triageThreadReply {
		t.Fatalf("Kind = %v, want %v", item.Kind, triageThreadReply)
	}
	if item.PostID != "reply-new" {
		t.Fatalf("PostID = %q, want newest loaded reply", item.PostID)
	}
	if item.CreateAt != 300 {
		t.Fatalf("CreateAt = %d, want newest loaded reply timestamp", item.CreateAt)
	}
	if item.Actor != "Ola" {
		t.Fatalf("Actor = %q, want newest loaded reply author", item.Actor)
	}
}

func TestTriageDismissKeyChangesWhenSignalChanges(t *testing.T) {
	base := triageItem{Kind: triageUnreadChannel, ChannelID: "alerts", UnreadCount: 3, CreateAt: 100}
	changed := base
	changed.UnreadCount = 5
	if triageDismissKey(base) == triageDismissKey(changed) {
		t.Fatal("dismiss key should change when signal changes")
	}
}

func TestTriageDismissKeyChangesWhenRootOnlyThreadSignalGetsMoreReplies(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", ThreadUnread: true, CreateAt: 100, ReplyCount: 1}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	before := triageDismissKey(items[0])

	m.postsByChannel["dev"][0].ReplyCount = 2
	items = buildTriageItems(m)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1: %#v", len(items), items)
	}
	if before == triageDismissKey(items[0]) {
		t.Fatal("dismiss key should change when root-only thread signal gets more replies")
	}
}

func TestBuildTriageItemsDropsWeakerUnreadChannelWhenThreadItemRepresentsSameWork(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 1}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageThreadReply {
		t.Fatalf("expected only thread item, got %#v", items)
	}
}

func TestBuildTriageItemsDropsThreadItemWhenMentionAlreadyRepresentsSameReplyThread(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1, Unread: 2}},
		recentEvents: []domain.Post{
			{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Artyom", Message: "need help", Mentioned: true, CreateAt: 200},
		},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 1},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Artyom", Message: "need help", CreateAt: 200},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageMention {
		t.Fatalf("expected only mention item, got %#v", items)
	}
}

func TestBuildTriageItemsKeepsNewerThreadReplyWhenMentionIsOlderReplyInSameThread(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1, Unread: 2}},
		recentEvents: []domain.Post{
			{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Artyom", Message: "older mentioned reply", Mentioned: true, CreateAt: 200},
		},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 2},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Artyom", Message: "older mentioned reply", CreateAt: 200},
				{ID: "reply-2", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "newer unread reply", CreateAt: 300},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 2 || items[0].Kind != triageMention || items[1].Kind != triageThreadReply || items[1].PostID != "reply-2" {
		t.Fatalf("expected mention plus newer thread reply, got %#v", items)
	}
}

func TestBuildTriageItemsKeepsUnreadChannelRowWhenThreadItemDoesNotExplainAllUnreadWork(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "old-unread", ChannelID: "dev", Username: "bot", Message: "older unread", Unread: true, CreateAt: 100},
				{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", ThreadUnread: true, CreateAt: 150, ReplyCount: 1},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "newer unread reply", Unread: true, CreateAt: 200},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 2 || items[0].Kind != triageThreadReply || items[1].Kind != triageUnreadChannel || items[1].UnreadCount != 2 {
		t.Fatalf("expected thread item plus degraded unread row, got %#v", items)
	}
	if items[1].PostID != "" || items[1].Preview != "" || items[1].Actor != "" {
		t.Fatalf("remaining unread row should degrade to channel-level signal, got %#v", items[1])
	}
}

func TestBuildTriageItemsDropsStaleThreadSignalAfterChannelRead(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 0, Mentions: 0}},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Artyom", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 1}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 0 {
		t.Fatalf("expected stale thread signal to be dropped after channel read, got %#v", items)
	}
}

func TestMarkChannelReadClearsCachedThreadUnreadSignals(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
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
	}

	m.markChannelRead("dev")

	for _, post := range m.posts {
		if post.ThreadUnread || post.Unread || post.Mentioned {
			t.Fatalf("current posts retain read flags after channel read: %#v", m.posts)
		}
	}
	for _, post := range m.postsByChannel["dev"] {
		if post.ThreadUnread || post.Unread || post.Mentioned {
			t.Fatalf("cached posts retain read flags after channel read: %#v", m.postsByChannel["dev"])
		}
	}
}

func TestBuildTriageItemsDoesNotResurrectReadThreadWhenUnrelatedUnreadArrives(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", ThreadUnread: true, CreateAt: 100, ReplyCount: 1}},
		},
	}
	m.markChannelRead("dev")
	m.channels[0].Unread = 1
	m.postsByChannel["dev"] = append(m.postsByChannel["dev"], domain.Post{ID: "other", ChannelID: "dev", Unread: true, Message: "unrelated unread", CreateAt: 200})

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageUnreadChannel || items[0].PostID != "other" {
		t.Fatalf("expected only unrelated unread item after read thread was cleared, got %#v", items)
	}
}
func TestBuildTriageItemsDoesNotResurrectThreadAfterThreadOpenClearsSameWork(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1, CreateAt: 100},
				{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, ThreadUnread: true, CreateAt: 200},
			},
		},
	}

	m.clearThreadReadSignal("dev", "root")

	items := buildTriageItems(m)
	if len(items) != 0 {
		t.Fatalf("cleared thread work should not resurrect in triage: %#v", items)
	}
}

func TestRebuildTriageItemsSkipsDismissedButReaddsChangedSignal(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3}},
		dismissedTriage: map[string]struct{}{},
	}

	m.rebuildTriageItems()
	if len(m.triageItems) != 1 {
		t.Fatalf("triage len = %d", len(m.triageItems))
	}
	key := triageDismissKey(m.triageItems[0])
	m.dismissedTriage[key] = struct{}{}
	m.rebuildTriageItems()
	if len(m.triageItems) != 0 {
		t.Fatalf("dismissed item should be hidden: %#v", m.triageItems)
	}

	m.channels[0].Unread = 5
	m.rebuildTriageItems()
	if len(m.triageItems) != 1 {
		t.Fatalf("changed signal should reappear: %#v", m.triageItems)
	}
}

func TestRebuildTriageItemsClampsSelection(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "a", Type: "O", DisplayName: "a", Unread: 1},
			{ID: "b", Type: "O", DisplayName: "b", Unread: 1},
		},
		triageSelected: 1,
	}
	m.rebuildTriageItems()
	if m.triageSelected != 1 {
		t.Fatalf("selected = %d", m.triageSelected)
	}
	m.channels = m.channels[:1]
	m.rebuildTriageItems()
	if m.triageSelected != 0 {
		t.Fatalf("selected should clamp to 0, got %d", m.triageSelected)
	}
}

func TestOpenCurrentChannelRefreshesTriageAfterClearingUnread(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3}},
		postsByChannel:  map[string][]domain.Post{},
		dismissedTriage: map[string]struct{}{},
	}
	m.rebuildTriageItems()
	if len(m.triageItems) != 1 {
		t.Fatalf("precondition triage len = %d", len(m.triageItems))
	}

	updated, _ := m.openCurrentChannel()
	got := updated.(Model)
	if len(got.triageItems) != 0 {
		t.Fatalf("triage should clear after opening read channel, got %#v", got.triageItems)
	}
}

func TestHandleTriageEnterOpensUnreadChannel(t *testing.T) {
	m := Model{
		channels:    []domain.Channel{{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3}},
		triageOpen:  true,
		triageItems: []triageItem{{Kind: triageUnreadChannel, ChannelID: "alerts", Title: "#alerts", UnreadCount: 3}},
	}

	updated, _ := m.handleTriageKey(triageKey("enter"))
	got := updated.(Model)
	if got.triageOpen {
		t.Fatal("triage should close after open")
	}
	if got.status == "" {
		t.Fatal("open should set loading/refresh status")
	}
	if got.channels[0].Unread != 0 {
		t.Fatalf("channel should be marked read on open, unread=%d", got.channels[0].Unread)
	}
}

func TestHandleTriageEnterOpensThreadRoot(t *testing.T) {
	m := Model{
		triageOpen:  true,
		triageItems: []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root-1", PostID: "reply-1"}},
	}

	updated, _ := m.handleTriageKey(triageKey("enter"))
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root-1" {
		t.Fatalf("thread not opened: threadOpen=%v root=%q", got.threadOpen, got.threadRootID)
	}
	if got.triageOpen {
		t.Fatal("triage should close after opening thread")
	}
}

func TestHandleTriageEnterCurrentThreadClearsLocalSignal(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
		selectedChannel: 0,
		triageOpen:      true,
		triageItems:     []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root", PostID: "reply"}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true, CreateAt: 100, ReplyCount: 1},
				{ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, ThreadUnread: true, CreateAt: 200},
			},
		},
	}

	updated, _ := m.handleTriageKey(triageKey("enter"))
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root" {
		t.Fatalf("thread not opened: threadOpen=%v root=%q", got.threadOpen, got.threadRootID)
	}
	if len(got.triageItems) != 0 || got.channels[0].Unread != 0 {
		t.Fatalf("opened current thread should clear local triage signal, items=%#v channel=%#v", got.triageItems, got.channels[0])
	}
	for _, post := range got.postsByChannel["dev"] {
		if post.Unread || post.ThreadUnread || post.Mentioned {
			t.Fatalf("thread flags not cleared for opened thread: %#v", got.postsByChannel["dev"])
		}
	}
}

func TestHandleTriageEnterThreadInOtherChannelRoutesThroughChannelLoad(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "town", Type: "O", DisplayName: "town"},
			{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2},
		},
		selectedChannel: 0,
		triageOpen:      true,
		triageItems:     []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root-1", PostID: "reply-1"}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root-1", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1, CreateAt: 100},
				{ID: "reply-1", ChannelID: "dev", RootID: "root-1", Unread: true, ThreadUnread: true, CreateAt: 200},
				{ID: "other", ChannelID: "dev", Unread: true, CreateAt: 300},
			},
		},
	}

	updated, _ := m.handleTriageKey(triageKey("enter"))
	got := updated.(Model)
	if got.selectedChannel != 1 || got.threadOpen {
		t.Fatalf("cross-channel thread should select target and wait for channel load, selected=%d threadOpen=%v", got.selectedChannel, got.threadOpen)
	}
	if got.pendingJumpChannelID != "dev" || got.pendingJumpThreadID != "root-1" || got.pendingJumpPostID != "root-1" {
		t.Fatalf("pending thread jump not set: channel=%q post=%q thread=%q", got.pendingJumpChannelID, got.pendingJumpPostID, got.pendingJumpThreadID)
	}
	if got.channels[1].Unread != 2 {
		t.Fatalf("cross-channel thread open should not clear the whole channel before load: %#v", got.channels[1])
	}
	if got.status == "" {
		t.Fatal("cross-channel open should start loading or refreshing")
	}

	updated, _ = got.Update(postsLoadedMsg{channelID: "dev", posts: []domain.Post{
		{ID: "root-1", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1, CreateAt: 100},
		{ID: "reply-1", ChannelID: "dev", RootID: "root-1", Unread: true, ThreadUnread: true, CreateAt: 200},
		{ID: "other", ChannelID: "dev", Unread: true, CreateAt: 300},
	}})
	got = updated.(Model)
	if !got.threadOpen || got.threadRootID != "root-1" {
		t.Fatalf("loaded cross-channel thread not opened: threadOpen=%v root=%q", got.threadOpen, got.threadRootID)
	}
	if got.channels[1].Unread != 1 {
		t.Fatalf("thread open should clear only thread work from counters: %#v", got.channels[1])
	}
	if len(got.triageItems) != 1 || got.triageItems[0].Kind != triageUnreadChannel || got.triageItems[0].ChannelID != "dev" || got.triageItems[0].PostID != "other" {
		t.Fatalf("thread open should leave only unrelated triage work: %#v", got.triageItems)
	}
}

func TestHandleTriageDismissHidesCurrentItemAndMovesSelection(t *testing.T) {
	m := Model{
		triageOpen:      true,
		dismissedTriage: map[string]struct{}{},
		triageItems: []triageItem{
			{Kind: triageUnreadChannel, ChannelID: "a", UnreadCount: 1},
			{Kind: triageUnreadChannel, ChannelID: "b", UnreadCount: 1},
		},
		channels: []domain.Channel{
			{ID: "a", Type: "O", DisplayName: "a", Unread: 1},
			{ID: "b", Type: "O", DisplayName: "b", Unread: 1},
		},
	}

	ok := m.dismissCurrentTriageItem()
	if !ok {
		t.Fatal("dismiss should succeed")
	}
	if len(m.triageItems) != 1 || m.triageItems[0].ChannelID != "b" {
		t.Fatalf("unexpected queue after dismiss: %#v", m.triageItems)
	}
}

func TestDismissedMentionRevealsRemainingUnreadChannelWork(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1, Unread: 3}},
		recentEvents:    []domain.Post{{ID: "mention", ChannelID: "dev", Username: "Artyom", Message: "need you", Mentioned: true, CreateAt: 200}},
		dismissedTriage: map[string]struct{}{},
	}
	m.rebuildTriageItems()
	if len(m.triageItems) != 1 || m.triageItems[0].Kind != triageMention {
		t.Fatalf("precondition expected only mention, got %#v", m.triageItems)
	}

	if !m.dismissCurrentTriageItem() {
		t.Fatal("dismiss should succeed")
	}
	if len(m.triageItems) != 1 || m.triageItems[0].Kind != triageUnreadChannel || m.triageItems[0].ChannelID != "dev" {
		t.Fatalf("dismissed mention should reveal remaining unread work, got %#v", m.triageItems)
	}
}

func TestLiveThreadReplyCreatesThreadTriageItem(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "town", Type: "O", DisplayName: "town"},
			{ID: "dev", Type: "O", DisplayName: "dev"},
		},
		selectedChannel: 0,
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", CreateAt: 100}},
		},
		events: make(chan domain.Event),
	}

	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "new reply", CreateAt: 200}}})
	got := updated.(Model)
	if len(got.triageItems) != 1 || got.triageItems[0].Kind != triageThreadReply || got.triageItems[0].PostID != "reply-1" {
		t.Fatalf("live reply should produce thread triage item, got %#v", got.triageItems)
	}
	if got.channels[1].Unread != 1 {
		t.Fatalf("thread reply should bump target channel unread, got %#v", got.channels[1])
	}
}

func TestLiveThreadReplyInCurrentChannelCreatesThreadTriageWhenThreadNotOpen(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}},
		selectedChannel: 0,
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", CreateAt: 100}},
		},
		events: make(chan domain.Event),
	}

	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "new reply", CreateAt: 200}}})
	got := updated.(Model)
	if len(got.triageItems) != 1 || got.triageItems[0].Kind != triageThreadReply || got.triageItems[0].PostID != "reply-1" {
		t.Fatalf("current-channel reply outside open thread should produce thread triage item, got %#v", got.triageItems)
	}
}

func TestLiveThreadReplyInOpenThreadDoesNotCreateTriageItem(t *testing.T) {
	m := Model{
		channels:        []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}},
		selectedChannel: 0,
		threadOpen:      true,
		threadRootID:    "root",
		postsByChannel: map[string][]domain.Post{
			"dev": {{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", CreateAt: 100}},
		},
		events: make(chan domain.Event),
	}

	updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "new reply", CreateAt: 200}}})
	got := updated.(Model)
	if len(got.triageItems) != 0 {
		t.Fatalf("visible open-thread reply should not create triage item, got %#v", got.triageItems)
	}
}

func TestBuildTriageItemsDoesNotAddUnreadChannelForMultipleUnreadRepliesInSameThread(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", Username: "Root", Message: "thread topic", ThreadUnread: true, CreateAt: 100, ReplyCount: 2},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "first reply", Unread: true, ThreadUnread: true, CreateAt: 200},
				{ID: "reply-2", ChannelID: "dev", RootID: "root", Username: "Ola", Message: "second reply", Unread: true, ThreadUnread: true, CreateAt: 300},
			},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageThreadReply || items[0].PostID != "reply-2" {
		t.Fatalf("multiple unread replies in one thread should be one thread item, got %#v", items)
	}
}
