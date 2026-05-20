package app

import (
	"sort"
	"strings"

	"band-tui/internal/domain"
)

func (m *Model) mergeThreadSignals(channelID string, signals []domain.ThreadSignal) {
	if channelID == "" || len(signals) == 0 {
		return
	}
	if m.threadSignalsByChannel == nil {
		m.threadSignalsByChannel = map[string][]domain.ThreadSignal{}
	}
	merged := make(map[string]domain.ThreadSignal, len(m.threadSignalsByChannel[channelID])+len(signals))
	for _, signal := range m.threadSignalsByChannel[channelID] {
		if signal.RootID == "" {
			continue
		}
		merged[signal.RootID] = signal
	}
	for _, signal := range signals {
		signal = normalizeThreadSignal(channelID, signal)
		if signal.RootID == "" {
			continue
		}
		current, ok := merged[signal.RootID]
		if !ok {
			merged[signal.RootID] = signal
			continue
		}
		merged[signal.RootID] = mergeThreadSignal(current, signal)
	}
	out := make([]domain.ThreadSignal, 0, len(merged))
	for _, signal := range merged {
		out = append(out, signal)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreateAt != out[j].CreateAt {
			return out[i].CreateAt > out[j].CreateAt
		}
		return out[i].RootID < out[j].RootID
	})
	m.threadSignalsByChannel[channelID] = out
}

func threadSignalFromPost(post domain.Post, rootLoaded bool) domain.ThreadSignal {
	mentionCount := 0
	if post.Mentioned {
		mentionCount = 1
	}
	return domain.ThreadSignal{
		ChannelID:    post.ChannelID,
		RootID:       postThreadRootID(post),
		PostID:       post.ID,
		Actor:        post.Username,
		Preview:      post.Message,
		CreateAt:     post.CreateAt,
		UnreadCount:  1,
		MentionCount: mentionCount,
		RootLoaded:   rootLoaded,
	}
}

func normalizeThreadSignal(channelID string, signal domain.ThreadSignal) domain.ThreadSignal {
	if signal.ChannelID == "" {
		signal.ChannelID = channelID
	}
	if signal.RootID == "" {
		signal.RootID = signal.PostID
	}
	if signal.UnreadCount <= 0 {
		signal.UnreadCount = 1
	}
	signal.Actor = strings.TrimSpace(signal.Actor)
	signal.Preview = strings.TrimSpace(signal.Preview)
	return signal
}

func mergeThreadSignal(a, b domain.ThreadSignal) domain.ThreadSignal {
	out := a
	out.UnreadCount = max(max(1, a.UnreadCount), max(1, b.UnreadCount))
	out.MentionCount = max(a.MentionCount, b.MentionCount)
	out.RootLoaded = a.RootLoaded || b.RootLoaded
	if b.CreateAt > a.CreateAt || out.PostID == "" {
		out.PostID = b.PostID
		out.Actor = b.Actor
		out.Preview = b.Preview
		out.CreateAt = b.CreateAt
	}
	return out
}

func (m *Model) applyThreadSignalsToLoadedPosts(channelID string) {
	if channelID == "" || len(m.threadSignals(channelID)) == 0 {
		return
	}
	for i := range m.posts {
		if m.posts[i].ChannelID == channelID && m.hasThreadSignal(channelID, m.posts[i].ID) {
			m.posts[i].ThreadUnread = true
		}
	}
}

func (m *Model) clearThreadSignal(channelID, rootID string) {
	if channelID == "" || rootID == "" || len(m.threadSignalsByChannel) == 0 {
		return
	}
	signals := m.threadSignalsByChannel[channelID]
	out := signals[:0]
	for _, signal := range signals {
		if signal.RootID != rootID {
			out = append(out, signal)
		}
	}
	if len(out) == 0 {
		delete(m.threadSignalsByChannel, channelID)
		return
	}
	m.threadSignalsByChannel[channelID] = out
}

func (m Model) threadSignals(channelID string) []domain.ThreadSignal {
	if len(m.threadSignalsByChannel) == 0 {
		return nil
	}
	return m.threadSignalsByChannel[channelID]
}

func (m Model) hasThreadSignal(channelID, rootID string) bool {
	for _, signal := range m.threadSignals(channelID) {
		if signal.RootID == rootID {
			return true
		}
	}
	return false
}

func (m Model) hiddenThreadSignals(channelID string) []domain.ThreadSignal {
	posts := m.channelImportancePosts(channelID)
	out := make([]domain.ThreadSignal, 0)
	for _, signal := range m.threadSignals(channelID) {
		if threadSignalRootLoaded(posts, signal.RootID) {
			continue
		}
		out = append(out, signal)
	}
	return out
}

func (m Model) unresolvedThreadSignalCount(channelID string) int {
	count := 0
	posts := m.channelImportancePosts(channelID)
	for _, signal := range m.threadSignals(channelID) {
		if threadSignalCoveredByPosts(posts, signal) {
			continue
		}
		count += max(1, signal.UnreadCount)
	}
	return count
}

func (m Model) unresolvedThreadSignalMentions(channelID string) int {
	count := 0
	posts := m.channelImportancePosts(channelID)
	for _, signal := range m.threadSignals(channelID) {
		if threadSignalCoveredByPosts(posts, signal) {
			continue
		}
		count += signal.MentionCount
	}
	return count
}

func threadSignalRootLoaded(posts []domain.Post, rootID string) bool {
	if rootID == "" {
		return false
	}
	for _, post := range posts {
		if post.ID == rootID && post.RootID == "" {
			return true
		}
	}
	return false
}

func threadSignalCoveredByPosts(posts []domain.Post, signal domain.ThreadSignal) bool {
	for _, post := range posts {
		if postThreadRootID(post) != signal.RootID {
			continue
		}
		if post.Mentioned || post.Unread || post.ThreadUnread {
			return true
		}
	}
	return false
}
