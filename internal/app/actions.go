package app

import (
	"os/exec"
	"runtime"
	"strings"

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
	for step := 1; step <= len(m.posts); step++ {
		idx := start + step*delta
		if idx < 0 || idx >= len(m.posts) {
			break
		}
		if isImportantPost(m.posts[idx]) {
			m.status = "jumped to unread"
			return m.selectPost(idx)
		}
	}
	m.status = "no more unread messages"
	return m, nil
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
