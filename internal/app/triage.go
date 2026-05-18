package app

import (
	"fmt"
	"sort"
	"strings"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

type triageKind int

const (
	triageMention triageKind = iota
	triageThreadReply
	triageUnreadChannel
)

type triageItem struct {
	Kind         triageKind
	ChannelID    string
	RootID       string
	PostID       string
	Title        string
	Actor        string
	Preview      string
	CreateAt     int64
	Score        int
	UnreadCount  int
	MentionCount int
	ReplyCount   int
}

func buildTriageItems(m Model) []triageItem {
	mentions := buildMentionTriageItems(m)
	blocked := make(map[string]struct{}, len(mentions))
	for _, item := range mentions {
		blocked[item.ChannelID] = struct{}{}
	}
	threads := filterThreadTriageItems(buildThreadReplyTriageItems(m), mentions)
	items := make([]triageItem, 0, len(mentions)+len(threads)+len(m.channels))
	items = append(items, mentions...)
	items = append(items, threads...)
	items = append(items, buildUnreadChannelTriageItems(m, blocked, threads)...)
	sort.SliceStable(items, func(i, j int) bool {
		return triageSortLess(items[i], items[j])
	})
	return items
}

func (m *Model) rebuildTriageItems() {
	built := buildTriageItems(*m)
	items := make([]triageItem, 0, len(built))
	for _, item := range built {
		if _, dismissed := m.dismissedTriage[triageDismissKey(item)]; dismissed {
			continue
		}
		items = append(items, item)
	}
	items = m.appendDismissedMentionUnreadFallbacks(built, items)
	sort.SliceStable(items, func(i, j int) bool {
		return triageSortLess(items[i], items[j])
	})
	m.triageItems = items
	m.clampTriageSelection()
}

func (m Model) openTriageOverlay() Model {
	m.rebuildTriageItems()
	m.triageOpen = true
	m.triageSelected = 0
	m.activityOpen = false
	m.switcherOpen = false
	return m
}

func (m *Model) clampTriageSelection() {
	if len(m.triageItems) == 0 {
		m.triageSelected = 0
		return
	}
	if m.triageSelected < 0 {
		m.triageSelected = 0
		return
	}
	if m.triageSelected >= len(m.triageItems) {
		m.triageSelected = len(m.triageItems) - 1
	}
}

func (m *Model) dismissCurrentTriageItem() bool {
	if m.triageSelected < 0 || m.triageSelected >= len(m.triageItems) {
		return false
	}
	if m.dismissedTriage == nil {
		m.dismissedTriage = map[string]struct{}{}
	}
	m.dismissedTriage[triageDismissKey(m.triageItems[m.triageSelected])] = struct{}{}
	m.rebuildTriageItems()
	return true
}

func (m Model) appendDismissedMentionUnreadFallbacks(built, visible []triageItem) []triageItem {
	if len(m.dismissedTriage) == 0 {
		return visible
	}
	for _, item := range built {
		if item.Kind != triageMention {
			continue
		}
		if _, dismissed := m.dismissedTriage[triageDismissKey(item)]; !dismissed {
			continue
		}
		channel, ok := triageChannelByID(m, item.ChannelID)
		if !ok {
			continue
		}
		represented := channel.Mentions + triageThreadUnreadCoverageForChannel(m, visible, channel.ID)
		if channel.Unread <= represented || triageUnreadExistsForChannel(visible, channel.ID) {
			continue
		}
		visible = append(visible, triageUnreadChannelItem(m, channel, visible))
	}
	return visible
}

func triageUnreadExistsForChannel(items []triageItem, channelID string) bool {
	for _, item := range items {
		if item.ChannelID == channelID && item.Kind == triageUnreadChannel {
			return true
		}
	}
	return false
}

func (m Model) openTriageItem(item triageItem) (tea.Model, tea.Cmd) {
	m.triageOpen = false
	if item.Kind == triageThreadReply {
		return m.openTriageThread(item)
	}
	if idx := m.channelIndexByID(item.ChannelID); idx >= 0 {
		m.selectedChannel = idx
	}
	m.threadOpen = false
	m.threadRootID = ""
	m.threadPosts = nil

	switch item.Kind {
	case triageMention:
		m.pendingJumpChannelID = item.ChannelID
		if item.RootID != "" {
			m.pendingJumpPostID = item.RootID
			m.pendingJumpThreadID = item.RootID
		} else {
			m.pendingJumpPostID = item.PostID
		}
		return m.openCurrentChannel()
	case triageUnreadChannel:
		return m.openCurrentChannel()
	default:
		return m, nil
	}
}

func (m Model) openTriageThread(item triageItem) (tea.Model, tea.Cmd) {
	rootID := item.RootID
	if rootID == "" {
		rootID = item.PostID
	}
	if rootID == "" {
		return m, nil
	}
	if item.ChannelID != "" {
		if idx := m.channelIndexByID(item.ChannelID); idx >= 0 && item.ChannelID != m.currentChannelID() {
			m.selectedChannel = idx
			m.pendingJumpChannelID = item.ChannelID
			m.pendingJumpPostID = rootID
			m.pendingJumpThreadID = rootID
			return m.openCurrentChannel()
		}
	}
	channelID := item.ChannelID
	if channelID == "" {
		channelID = m.currentChannelID()
	}
	m.saveActiveDraft()
	m.loadDraft(threadDraftKey(channelID, rootID))
	m.clearThreadReadSignal(channelID, rootID)
	m.rebuildTriageItems()
	m.threadOpen = true
	m.threadRootID = rootID
	m.threadLoading = true
	m.threadPosts = nil
	m.threadFocusComposer = false
	m.focus = focusComposer
	if m.width > 0 && m.height > 0 {
		m.resize()
		m.refreshViewport()
		m.refreshThreadViewport()
	}
	m.status = "loading thread…"
	return m, loadThreadCmd(m.ctx, m.backend, rootID)
}

func buildMentionTriageItems(m Model) []triageItem {
	items := make([]triageItem, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.Mentions <= 0 {
			continue
		}

		item := triageItem{
			Kind:         triageMention,
			ChannelID:    ch.ID,
			Title:        m.channelLabel(ch.ID),
			Score:        triageKindPriority(triageMention),
			MentionCount: ch.Mentions,
			UnreadCount:  ch.Unread,
			CreateAt:     ch.LastPostAt,
		}
		for _, post := range m.recentEvents {
			if post.ChannelID != ch.ID {
				continue
			}
			item.RootID = post.RootID
			item.PostID = post.ID
			item.Actor = triageActor(post)
			item.Preview = triagePreview(post.Message)
			item.CreateAt = post.CreateAt
			break
		}
		items = append(items, item)
	}
	return items
}

func buildThreadReplyTriageItems(m Model) []triageItem {
	latestByThread := map[string]triageItem{}
	for channelID, posts := range m.postsByChannel {
		channel, ok := triageChannelByID(m, channelID)
		if !ok || (channel.Unread <= 0 && channel.Mentions <= 0) {
			continue
		}
		for _, post := range posts {
			if !post.ThreadUnread {
				continue
			}
			rootID := triageThreadRootID(post)
			threadPosts := triageThreadPosts(m, channelID, rootID, posts)
			signalPost := triageThreadSignalPost(threadPosts, post)
			item := triageItem{
				Kind:      triageThreadReply,
				ChannelID: channelID,
				RootID:    rootID,
				PostID:    signalPost.ID,
				Title:     m.channelLabel(channelID),
				Actor:     triageActor(signalPost),
				Preview:   triageThreadPreview(threadPosts, post),
				CreateAt:  signalPost.CreateAt,
				Score:     triageKindPriority(triageThreadReply),
			}
			if root, ok := triageThreadRootPost(threadPosts, rootID); ok {
				item.ReplyCount = root.ReplyCount
			}
			key := channelID + "\x00" + rootID
			current, ok := latestByThread[key]
			if !ok || triageSortLess(item, current) {
				latestByThread[key] = item
			}
		}
	}
	items := make([]triageItem, 0, len(latestByThread))
	for _, item := range latestByThread {
		items = append(items, item)
	}
	return items
}

func triageThreadPosts(m Model, channelID, rootID string, posts []domain.Post) []domain.Post {
	if !triageLoadedThreadMatches(m, channelID, rootID) {
		return posts
	}

	merged := make([]domain.Post, 0, len(posts)+len(m.threadPosts))
	merged = append(merged, posts...)
	merged = append(merged, m.threadPosts...)
	return merged
}

func triageLoadedThreadMatches(m Model, channelID, rootID string) bool {
	if m.threadRootID != rootID || len(m.threadPosts) == 0 {
		return false
	}

	foundChannel := false
	for _, post := range m.threadPosts {
		if post.ID != rootID && post.RootID != rootID {
			return false
		}
		if post.ChannelID == "" {
			continue
		}
		if post.ChannelID != channelID {
			return false
		}
		foundChannel = true
	}
	return foundChannel
}

func triageThreadSignalPost(posts []domain.Post, post domain.Post) domain.Post {
	rootID := triageThreadRootID(post)
	if post.ID != rootID {
		return post
	}

	latest := post
	for _, candidate := range posts {
		if candidate.RootID != rootID {
			continue
		}
		if candidate.CreateAt > latest.CreateAt || (candidate.CreateAt == latest.CreateAt && candidate.ID < latest.ID) {
			latest = candidate
		}
	}
	return latest
}

func buildUnreadChannelTriageItems(m Model, blocked map[string]struct{}, threads []triageItem) []triageItem {
	items := make([]triageItem, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.Unread <= 0 {
			continue
		}
		if _, ok := blocked[ch.ID]; ok {
			continue
		}
		item := triageItem{
			Kind:        triageUnreadChannel,
			ChannelID:   ch.ID,
			Title:       m.channelLabel(ch.ID),
			CreateAt:    ch.LastPostAt,
			Score:       triageKindPriority(triageUnreadChannel),
			UnreadCount: ch.Unread,
		}
		if post, ok := triageLatestUnreadPost(m.postsByChannel[ch.ID]); ok {
			if triagePostCoveredByThread(post, threads) {
				if ch.Unread <= triageThreadUnreadCoverageForChannel(m, threads, ch.ID) {
					continue
				}
			} else {
				item.PostID = post.ID
				item.RootID = triageThreadRootID(post)
				item.Actor = triageActor(post)
				item.Preview = triagePreview(post.Message)
				item.CreateAt = post.CreateAt
			}
		} else if triageThreadExistsForChannel(threads, ch.ID) && ch.Unread <= triageThreadUnreadCoverageForChannel(m, threads, ch.ID) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func triageUnreadChannelItem(m Model, ch domain.Channel, threads []triageItem) triageItem {
	item := triageItem{
		Kind:        triageUnreadChannel,
		ChannelID:   ch.ID,
		Title:       m.channelLabel(ch.ID),
		CreateAt:    ch.LastPostAt,
		Score:       triageKindPriority(triageUnreadChannel),
		UnreadCount: ch.Unread,
	}
	if post, ok := triageLatestUnreadPost(m.postsByChannel[ch.ID]); ok && !triagePostCoveredByThread(post, threads) {
		item.PostID = post.ID
		item.RootID = triageThreadRootID(post)
		item.Actor = triageActor(post)
		item.Preview = triagePreview(post.Message)
		item.CreateAt = post.CreateAt
	}
	return item
}

func triageDismissKey(item triageItem) string {
	return fmt.Sprintf("%d|%s|%s|%s|%d|%d|%d|%d", item.Kind, item.ChannelID, item.RootID, item.PostID, item.CreateAt, item.UnreadCount, item.MentionCount, item.ReplyCount)
}

func triageSortLess(a, b triageItem) bool {
	ap := triageKindPriority(a.Kind)
	bp := triageKindPriority(b.Kind)
	if ap != bp {
		return ap < bp
	}
	if a.CreateAt != b.CreateAt {
		return a.CreateAt > b.CreateAt
	}
	if a.ChannelID != b.ChannelID {
		return a.ChannelID < b.ChannelID
	}
	if a.RootID != b.RootID {
		return a.RootID < b.RootID
	}
	if a.PostID != b.PostID {
		return a.PostID < b.PostID
	}
	if a.Title != b.Title {
		return a.Title < b.Title
	}
	if a.Actor != b.Actor {
		return a.Actor < b.Actor
	}
	return a.Preview < b.Preview
}

func triageKindPriority(kind triageKind) int {
	switch kind {
	case triageMention:
		return 0
	case triageThreadReply:
		return 1
	case triageUnreadChannel:
		return 2
	default:
		return 3
	}
}

func triageChannelByID(m Model, channelID string) (domain.Channel, bool) {
	idx := m.channelIndexByID(channelID)
	if idx < 0 {
		return domain.Channel{}, false
	}
	return m.channels[idx], true
}

func filterThreadTriageItems(threads, mentions []triageItem) []triageItem {
	items := make([]triageItem, 0, len(threads))
	for _, thread := range threads {
		if triageThreadCoveredByMention(thread, mentions) {
			continue
		}
		items = append(items, thread)
	}
	return items
}

func triageThreadCoveredByMention(thread triageItem, mentions []triageItem) bool {
	for _, mention := range mentions {
		if mention.ChannelID != thread.ChannelID {
			continue
		}
		if mention.PostID != "" && mention.PostID == thread.PostID {
			return true
		}
	}
	return false
}

func triageThreadRootPost(posts []domain.Post, rootID string) (domain.Post, bool) {
	for _, post := range posts {
		if post.ID == rootID {
			return post, true
		}
	}
	return domain.Post{}, false
}

func triageThreadExistsForChannel(threads []triageItem, channelID string) bool {
	for _, thread := range threads {
		if thread.ChannelID == channelID {
			return true
		}
	}
	return false
}

func triageThreadCountForChannel(threads []triageItem, channelID string) int {
	count := 0
	for _, thread := range threads {
		if thread.ChannelID == channelID {
			count++
		}
	}
	return count
}

func triageThreadUnreadCoverageForChannel(m Model, threads []triageItem, channelID string) int {
	if len(threads) == 0 {
		return 0
	}
	type coverage struct {
		unread       int
		threadSignal bool
	}
	roots := make(map[string]coverage, triageThreadCountForChannel(threads, channelID))
	for _, thread := range threads {
		if thread.ChannelID != channelID {
			continue
		}
		rootID := thread.RootID
		if rootID == "" {
			rootID = thread.PostID
		}
		if rootID != "" {
			roots[rootID] = coverage{}
		}
	}
	if len(roots) == 0 {
		return 0
	}
	for _, post := range m.postsByChannel[channelID] {
		rootID := triageThreadRootID(post)
		cov, ok := roots[rootID]
		if !ok {
			continue
		}
		if post.Unread || post.Mentioned {
			cov.unread++
		}
		if post.ThreadUnread {
			cov.threadSignal = true
		}
		roots[rootID] = cov
	}
	count := 0
	for _, cov := range roots {
		if cov.unread > 0 {
			count += cov.unread
		} else if cov.threadSignal {
			count++
		} else {
			count++
		}
	}
	return count
}

func triagePostCoveredByThread(post domain.Post, threads []triageItem) bool {
	rootID := triageThreadRootID(post)
	for _, thread := range threads {
		if thread.ChannelID != post.ChannelID {
			continue
		}
		if thread.PostID == post.ID {
			return true
		}
		if thread.RootID != "" && thread.RootID == rootID {
			return true
		}
	}
	return false
}

func triagePreview(message string) string {
	message = sanitizeMessageText(message)
	message = sanitizeTerminal(message)
	return strings.Join(strings.Fields(message), " ")
}

func triageThreadPreview(posts []domain.Post, post domain.Post) string {
	rootID := triageThreadRootID(post)
	for _, candidate := range posts {
		if candidate.ID == rootID {
			preview := triagePreview(candidate.Message)
			if preview != "" {
				return preview
			}
			break
		}
	}
	return triagePreview(post.Message)
}

func triageThreadRootID(post domain.Post) string {
	if post.RootID != "" {
		return post.RootID
	}
	return post.ID
}

func triageLatestUnreadPost(posts []domain.Post) (domain.Post, bool) {
	var latest domain.Post
	found := false
	for _, post := range posts {
		if !post.Unread && !post.Mentioned {
			continue
		}
		if !found || post.CreateAt > latest.CreateAt || (post.CreateAt == latest.CreateAt && post.ID < latest.ID) {
			latest = post
			found = true
		}
	}
	return latest, found
}

func triageActor(post domain.Post) string {
	if post.Username != "" {
		return post.Username
	}
	if post.UserID != "" {
		return shortID(post.UserID)
	}
	return ""
}
