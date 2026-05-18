package app

import (
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"band-tui/internal/domain"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) openSelectedThread() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	rootID := m.posts[idx].ID
	if m.posts[idx].RootID != "" {
		rootID = m.posts[idx].RootID
	}
	channelID := m.posts[idx].ChannelID
	if channelID == "" {
		channelID = m.currentChannelID()
	}
	m.clearEditingState()
	m.clearPendingDelete()
	m.saveActiveDraft()
	m.applyThreadRead(channelID, rootID)
	m.rebuildTriageItems()
	m.threadOpen = true
	m.threadRootID = rootID
	m.loadDraft(threadDraftKey(channelID, rootID))
	m.threadLoading = true
	m.threadPosts = nil
	m.threadFocusComposer = false
	m.focus = focusComposer
	m.applyFocus()
	m.resize()
	m.refreshViewport()
	m.refreshThreadViewport()
	m.scrollSelectedPostIntoView()
	m.status = "loading thread…"
	return m, loadThreadCmd(m.ctx, m.backend, rootID)
}

func (m Model) selectRelativePost(delta int) (tea.Model, tea.Cmd) {
	return m.selectPost(m.selectedPost + delta)
}

func (m Model) selectRelativeImportantPost(delta int) (tea.Model, tea.Cmd) {
	if len(m.posts) == 0 {
		return m, nil
	}
	if delta == 0 {
		delta = 1
	}
	start := m.selectedPost
	if start < 0 || start >= len(m.posts) {
		start = 0
		if delta < 0 {
			start = len(m.posts) - 1
		}
	}
	idx := relativeImportantPost(m.posts, start, delta)
	if idx < 0 {
		m.status = "no more unread messages"
		return m, nil
	}
	m.status = "jumped to unread"
	return m.selectPost(idx)
}

func (m Model) selectPost(index int) (tea.Model, tea.Cmd) {
	if len(m.posts) == 0 {
		return m, nil
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.posts) {
		index = len(m.posts) - 1
	}
	if index == m.selectedPost {
		return m, nil
	}
	m.selectedPost = index
	m.pendingDeletePostID = ""
	m.refreshViewport()
	m.scrollSelectedPostIntoView()
	return m, nil
}

func (m *Model) scrollSelectedPostIntoView() {
	if m.selectedPost < 0 || m.selectedPost >= len(m.postLineOffsets) {
		return
	}
	top := m.postLineOffsets[m.selectedPost]
	bottom := top
	if m.selectedPost+1 < len(m.postLineOffsets) {
		bottom = max(top, m.postLineOffsets[m.selectedPost+1]-2)
	} else {
		bottom = top + 2
	}

	viewTop := m.viewport.YOffset
	viewBottom := m.viewport.YOffset + max(1, m.viewport.Height) - 1
	if top < viewTop {
		m.viewport.SetYOffset(top)
		return
	}
	if bottom > viewBottom {
		m.viewport.SetYOffset(max(0, bottom-m.viewport.Height+1))
	}
}

func (m Model) selectedPostIndex() (int, bool) {
	if m.selectedPost < 0 || m.selectedPost >= len(m.posts) {
		return 0, false
	}
	return m.selectedPost, true
}

func (m Model) openSelectedPostLink() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	link := firstURL(m.posts[idx].Message)
	if link == "" {
		m.status = "no link in selected message"
		return m, nil
	}
	m.status = "opening link…"
	return m, func() tea.Msg {
		return actionDoneMsg{err: openExternal(link), status: "link opened"}
	}
}

func (m Model) copySelectedPostText() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	text := strings.TrimSpace(m.posts[idx].Message)
	if text == "" {
		m.status = "selected message is empty"
		return m, nil
	}
	m.status = "copying message…"
	return m, func() tea.Msg {
		return actionDoneMsg{err: clipboard.WriteAll(text), status: "message copied"}
	}
}

func (m Model) quoteSelectedPost() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	quote := formatQuotedReply(m.posts[idx])
	if quote == "" {
		m.status = "selected message is empty"
		return m, nil
	}
	current := m.composer.Value()
	if current != "" && !strings.HasSuffix(current, "\n") {
		current += "\n"
	}
	m.composer.SetValue(current + quote)
	m.focus = focusComposer
	m.applyFocus()
	m.status = "quote inserted"
	return m, nil
}

func (m Model) selectedPostPermalink() (string, bool) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return "", false
	}
	serverURL := ""
	if m.session != nil {
		serverURL = strings.TrimRight(strings.TrimSpace(m.session.ServerURL), "/")
	}
	if serverURL == "" {
		return "", false
	}
	post := m.posts[idx]
	if post.ID == "" {
		return "", false
	}
	path := "/pl/" + url.PathEscape(post.ID)
	if chIdx := m.channelIndexByID(post.ChannelID); chIdx >= 0 {
		teamID := m.channels[chIdx].TeamID
		if m.session != nil && teamID != "" {
			for _, team := range m.session.Teams {
				if team.ID != teamID {
					continue
				}
				slug := strings.TrimSpace(team.Name)
				if slug == "" {
					slug = strings.TrimSpace(team.DisplayName)
				}
				if slug != "" {
					path = "/" + url.PathEscape(slug) + path
				}
				break
			}
		}
	}
	return serverURL + path, true
}

func (m Model) copySelectedPostPermalink() (tea.Model, tea.Cmd) {
	link, ok := m.selectedPostPermalink()
	if !ok {
		m.status = "no permalink for selected message"
		return m, nil
	}
	m.status = "copying permalink…"
	return m, func() tea.Msg {
		return actionDoneMsg{err: clipboard.WriteAll(link), status: "permalink copied"}
	}
}

func (m Model) editSelectedPost() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	post := m.posts[idx]
	if !m.isOwnPost(post) {
		m.status = "can only edit your own messages"
		return m, nil
	}
	m.pendingDeletePostID = ""
	m.suspendedDraftKey = m.activeDraftKey
	m.suspendedDraftValue = m.composer.Value()
	m.activeDraftKey = ""
	m.composer.SetValue(post.Message)
	m.editingPostID = post.ID
	m.focus = focusComposer
	m.applyFocus()
	m.status = "editing message"
	return m, nil
}

func (m Model) deleteSelectedPost() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	post := m.posts[idx]
	if !m.isOwnPost(post) {
		m.status = "can only delete your own messages"
		return m, nil
	}
	if m.pendingDeletePostID != post.ID {
		m.pendingDeletePostID = post.ID
		m.status = "press D again to delete"
		return m, nil
	}
	m.pendingDeletePostID = ""
	m.status = "deleting message…"
	return m, deletePostCmd(m.ctx, m.backend, post.ID)
}

func formatQuotedReply(post domain.Post) string {
	message := strings.TrimSpace(post.Message)
	if message == "" {
		return ""
	}
	author := strings.TrimSpace(post.Username)
	if author == "" && post.UserID != "" {
		author = shortID(post.UserID)
	}
	if author == "" {
		author = "unknown"
	}
	lines := strings.Split(message, "\n")
	var b strings.Builder
	b.WriteString("> ")
	b.WriteString(author)
	b.WriteString(":\n")
	for _, line := range lines {
		b.WriteString("> ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func reactionState(post domain.Post, emojiName string) (domain.PostReaction, bool) {
	for _, reaction := range post.Reactions {
		if reaction.Name == emojiName {
			return reaction, true
		}
	}
	return domain.PostReaction{}, false
}

func mergeAddedReaction(post domain.Post, emojiName string) domain.Post {
	reactions := append([]domain.PostReaction(nil), post.Reactions...)
	for i := range reactions {
		if reactions[i].Name != emojiName {
			continue
		}
		reactions[i].Count++
		reactions[i].Reacted = true
		post.Reactions = reactions
		return post
	}
	reactions = append(reactions, domain.PostReaction{Name: emojiName, Count: 1, Reacted: true})
	post.Reactions = reactions
	return post
}

func mergeRemovedReaction(post domain.Post, emojiName string) domain.Post {
	reactions := append([]domain.PostReaction(nil), post.Reactions...)
	for i := range reactions {
		if reactions[i].Name != emojiName {
			continue
		}
		if reactions[i].Count <= 1 {
			post.Reactions = append(reactions[:i], reactions[i+1:]...)
			return post
		}
		reactions[i].Count--
		reactions[i].Reacted = false
		post.Reactions = reactions
		return post
	}
	post.Reactions = reactions
	return post
}

func firstURL(s string) string {
	if match := markdownLinkRE.FindStringSubmatch(s); len(match) == 3 {
		return match[2]
	}
	return bareURLRE.FindString(s)
}

func openExternal(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
