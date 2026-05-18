package app

import "band-tui/internal/domain"

func importantPost(post domain.Post) bool {
	return mentionPost(post) || unreadPost(post) || post.ThreadUnread
}

func mentionPost(post domain.Post) bool {
	return post.Mentioned
}

func unreadPost(post domain.Post) bool {
	return post.Unread
}

func (m *Model) applyChannelRead(channelID string) {
	if channelID == "" {
		return
	}
	m.clearChannelImportance(channelID)
	m.reconcileChannelImportance(channelID)
}

func (m *Model) applyThreadRead(channelID, rootID string) {
	if channelID == "" || rootID == "" {
		return
	}
	m.clearThreadImportance(channelID, rootID)
	m.reconcileChannelImportance(channelID)
}

func (m *Model) reconcileChannelImportance(channelID string) {
	idx := m.channelIndexByID(channelID)
	if idx < 0 {
		return
	}

	unread, mentions := reconcileChannelPosts(m.channelImportancePosts(channelID))
	m.channels[idx].Unread = unread
	m.channels[idx].Mentions = mentions
}

func (m *Model) reconcileAllImportance() {
	for i := range m.channels {
		m.reconcileChannelImportance(m.channels[i].ID)
	}
}

func (m *Model) clearChannelImportance(channelID string) {
	for i := range m.posts {
		if m.posts[i].ChannelID == channelID {
			clearImportantFlags(&m.posts[i])
		}
	}
	if m.postsByChannel != nil {
		if posts, ok := m.postsByChannel[channelID]; ok {
			for i := range posts {
				clearImportantFlags(&posts[i])
			}
			m.postsByChannel[channelID] = posts
		}
	}
	for i := range m.threadPosts {
		if m.threadPosts[i].ChannelID == channelID {
			clearImportantFlags(&m.threadPosts[i])
		}
	}
}

func (m *Model) clearThreadImportance(channelID, rootID string) {
	for i := range m.posts {
		if postInThread(m.posts[i], channelID, rootID) {
			clearImportantFlags(&m.posts[i])
		}
	}
	if m.postsByChannel != nil {
		if posts, ok := m.postsByChannel[channelID]; ok {
			for i := range posts {
				if postInThread(posts[i], channelID, rootID) {
					clearImportantFlags(&posts[i])
				}
			}
			m.postsByChannel[channelID] = posts
		}
	}
	for i := range m.threadPosts {
		if postInThread(m.threadPosts[i], channelID, rootID) {
			clearImportantFlags(&m.threadPosts[i])
		}
	}
}

func (m *Model) channelImportancePosts(channelID string) []domain.Post {
	if m.postsByChannel != nil {
		if posts, ok := m.postsByChannel[channelID]; ok {
			return posts
		}
	}
	if m.currentChannelID() == channelID {
		return m.posts
	}
	return nil
}

func reconcileChannelPosts(posts []domain.Post) (unread int, mentions int) {
	for i := range posts {
		post := posts[i]
		if mentionPost(post) {
			mentions++
		}
		if unreadPost(post) {
			unread++
			continue
		}
		if threadRootUnreadPost(posts, i) {
			unread++
		}
	}
	return unread, mentions
}

func threadRootUnreadPost(posts []domain.Post, idx int) bool {
	post := posts[idx]
	if !post.ThreadUnread || post.ID == "" || postThreadRootID(post) != post.ID || mentionPost(post) || unreadPost(post) {
		return false
	}
	for i := range posts {
		if i == idx || postThreadRootID(posts[i]) != post.ID {
			continue
		}
		if mentionPost(posts[i]) || unreadPost(posts[i]) {
			return false
		}
	}
	return true
}

func postInThread(post domain.Post, channelID, rootID string) bool {
	return post.ChannelID == channelID && postThreadRootID(post) == rootID
}

func clearImportantFlags(post *domain.Post) {
	post.Mentioned = false
	post.Unread = false
	post.ThreadUnread = false
}

func threadChannelID(posts []domain.Post) string {
	for _, post := range posts {
		if post.ChannelID != "" {
			return post.ChannelID
		}
	}
	return ""
}
