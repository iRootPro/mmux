package app

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"band-tui/internal/domain"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

type reactionOption struct {
	Name  string
	Glyph string
}

var defaultReactions = []reactionOption{
	{Name: "+1", Glyph: "👍"},
	{Name: "eyes", Glyph: "👀"},
	{Name: "white_check_mark", Glyph: "✅"},
	{Name: "heart", Glyph: "❤️"},
	{Name: "tada", Glyph: "🎉"},
}

var standardReactionCatalog = []reactionOption{
	{Name: "-1", Glyph: "👎"},
	{Name: "smile", Glyph: "😄"},
	{Name: "smiley", Glyph: "😃"},
	{Name: "grinning", Glyph: "😀"},
	{Name: "joy", Glyph: "😂"},
	{Name: "rofl", Glyph: "🤣"},
	{Name: "slightly_smiling_face", Glyph: "🙂"},
	{Name: "wink", Glyph: "😉"},
	{Name: "blush", Glyph: "😊"},
	{Name: "thinking_face", Glyph: "🤔"},
	{Name: "face_with_monocle", Glyph: "🧐"},
	{Name: "neutral_face", Glyph: "😐"},
	{Name: "expressionless", Glyph: "😑"},
	{Name: "confused", Glyph: "😕"},
	{Name: "disappointed", Glyph: "😞"},
	{Name: "cry", Glyph: "😢"},
	{Name: "sob", Glyph: "😭"},
	{Name: "angry", Glyph: "😠"},
	{Name: "rage", Glyph: "😡"},
	{Name: "scream", Glyph: "😱"},
	{Name: "open_mouth", Glyph: "😮"},
	{Name: "astonished", Glyph: "😲"},
	{Name: "partying_face", Glyph: "🥳"},
	{Name: "sunglasses", Glyph: "😎"},
	{Name: "nerd_face", Glyph: "🤓"},
	{Name: "facepalm", Glyph: "🤦"},
	{Name: "shrug", Glyph: "🤷"},
	{Name: "pray", Glyph: "🙏"},
	{Name: "clap", Glyph: "👏"},
	{Name: "raised_hands", Glyph: "🙌"},
	{Name: "muscle", Glyph: "💪"},
	{Name: "ok_hand", Glyph: "👌"},
	{Name: "wave", Glyph: "👋"},
	{Name: "point_up", Glyph: "☝️"},
	{Name: "point_up_2", Glyph: "👆"},
	{Name: "point_down", Glyph: "👇"},
	{Name: "point_left", Glyph: "👈"},
	{Name: "point_right", Glyph: "👉"},
	{Name: "fire", Glyph: "🔥"},
	{Name: "boom", Glyph: "💥"},
	{Name: "sparkles", Glyph: "✨"},
	{Name: "star", Glyph: "⭐"},
	{Name: "100", Glyph: "💯"},
	{Name: "heavy_check_mark", Glyph: "✔️"},
	{Name: "x", Glyph: "❌"},
	{Name: "warning", Glyph: "⚠️"},
	{Name: "question", Glyph: "❓"},
	{Name: "grey_question", Glyph: "❔"},
	{Name: "exclamation", Glyph: "❗"},
	{Name: "grey_exclamation", Glyph: "❕"},
	{Name: "bulb", Glyph: "💡"},
	{Name: "rocket", Glyph: "🚀"},
	{Name: "bug", Glyph: "🐛"},
	{Name: "eyes", Glyph: "👀"},
	{Name: "heart", Glyph: "❤️"},
	{Name: "broken_heart", Glyph: "💔"},
	{Name: "blue_heart", Glyph: "💙"},
	{Name: "green_heart", Glyph: "💚"},
	{Name: "yellow_heart", Glyph: "💛"},
	{Name: "purple_heart", Glyph: "💜"},
	{Name: "coffee", Glyph: "☕"},
	{Name: "beer", Glyph: "🍺"},
	{Name: "beers", Glyph: "🍻"},
	{Name: "pizza", Glyph: "🍕"},
}

func reactionDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, option := range defaultReactions {
		if option.Name == name {
			return option.Glyph
		}
	}
	for _, option := range standardReactionCatalog {
		if option.Name == name {
			return option.Glyph
		}
	}
	return ":" + name + ":"
}

func (m Model) reactionOptionsForPost(post domain.Post) []reactionOption {
	query := strings.ToLower(strings.TrimSpace(m.reactionPickerQuery))
	seen := map[string]struct{}{}
	options := make([]reactionOption, 0, len(defaultReactions)+len(standardReactionCatalog)+len(post.Reactions))
	add := func(option reactionOption) {
		option.Name = strings.TrimSpace(option.Name)
		if option.Name == "" {
			return
		}
		if option.Glyph == "" {
			option.Glyph = reactionDisplayName(option.Name)
		}
		if _, ok := seen[option.Name]; ok {
			return
		}
		if query != "" {
			needle := strings.ToLower(option.Name + " " + option.Glyph)
			if !strings.Contains(needle, query) {
				return
			}
		}
		seen[option.Name] = struct{}{}
		options = append(options, option)
	}
	for _, option := range defaultReactions {
		add(option)
	}
	for _, reaction := range post.Reactions {
		add(reactionOption{Name: reaction.Name})
	}
	if m.session != nil {
		for _, emoji := range m.session.Emojis {
			add(reactionOption{Name: emoji.Name, Glyph: emoji.Glyph})
		}
	}
	for _, option := range standardReactionCatalog {
		add(option)
	}
	if query != "" {
		name := sanitizeReactionName(query)
		if name != "" {
			if _, ok := seen[name]; !ok {
				options = append(options, reactionOption{Name: name, Glyph: reactionDisplayName(name)})
			}
		}
	}
	return options
}

func sanitizeReactionName(s string) string {
	s = strings.TrimSpace(strings.Trim(s, ":"))
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '+' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

const reactionPickerGridColumns = 8

func (m Model) reactionPickerColumns(options []reactionOption) int {
	if strings.TrimSpace(m.reactionPickerQuery) != "" {
		return min(2, max(1, len(options)))
	}
	return reactionPickerGridColumns
}

type reactionTargetKind int

const (
	reactionTargetTimeline reactionTargetKind = iota
	reactionTargetThread
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
	m.threadReturnFocus = focusTimeline
	m.threadRootID = rootID
	m.threadSelected = -1
	m.loadDraft(threadDraftKey(channelID, rootID))
	m.threadLoading = true
	m.threadPosts = nil
	m.threadFocusComposer = true
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

func (m Model) selectedPostValue() (domain.Post, bool) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return domain.Post{}, false
	}
	post := m.posts[idx]
	if m.hasThreadSignal(post.ChannelID, post.ID) {
		post.ThreadUnread = true
	}
	return post, true
}

func (m Model) openSelectedPostLink() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	post := m.posts[idx]
	link := firstURL(post.Message)
	if link == "" {
		link = m.firstFileURL(post)
	}
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
	text := strings.TrimSpace(postPlainText(m.posts[idx]))
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

func (m Model) firstFileURL(post domain.Post) string {
	if len(post.Files) == 0 || post.Files[0].ID == "" || m.session == nil {
		return ""
	}
	serverURL := strings.TrimRight(strings.TrimSpace(m.session.ServerURL), "/")
	if serverURL == "" {
		return ""
	}
	return serverURL + "/api/v4/files/" + url.PathEscape(post.Files[0].ID)
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

func (m Model) openReactionPicker() (tea.Model, tea.Cmd) {
	idx, ok := m.selectedPostIndex()
	if !ok {
		return m, nil
	}
	m.reactionPickerOpen = true
	m.reactionPickerSelected = 0
	m.reactionPickerQuery = ""
	m.reactionTargetKind = reactionTargetTimeline
	m.reactionTargetPostID = m.posts[idx].ID
	m.status = "pick a reaction"
	return m, nil
}

func (m Model) toggleSelectedReaction() (tea.Model, tea.Cmd) {
	post, ok := m.selectedReactionTarget()
	if !ok {
		return m, nil
	}
	options := m.reactionOptionsForPost(post)
	if len(options) == 0 || m.reactionPickerSelected < 0 || m.reactionPickerSelected >= len(options) {
		return m, nil
	}
	reaction := options[m.reactionPickerSelected]
	m.reactionPickerOpen = false
	m.status = "toggling reaction…"
	return m, toggleReactionCmd(m.ctx, m.backend, post, reaction.Name)
}

func (m Model) openThreadReactionPicker() (tea.Model, tea.Cmd) {
	post, ok := m.selectedThreadPost()
	if !ok {
		return m, nil
	}
	m.reactionPickerOpen = true
	m.reactionPickerSelected = 0
	m.reactionPickerQuery = ""
	m.reactionTargetKind = reactionTargetThread
	m.reactionTargetPostID = post.ID
	m.status = "pick a reaction"
	return m, nil
}

func (m Model) selectedReactionTarget() (domain.Post, bool) {
	switch m.reactionTargetKind {
	case reactionTargetThread:
		for _, post := range m.threadPosts {
			if post.ID == m.reactionTargetPostID {
				return post, true
			}
		}
	default:
		for _, post := range m.posts {
			if post.ID == m.reactionTargetPostID {
				return post, true
			}
		}
	}
	return domain.Post{}, false
}

func postPlainText(post domain.Post) string {
	message := strings.TrimSpace(post.Message)
	if len(post.Files) == 0 {
		return message
	}
	var lines []string
	if message != "" {
		lines = append(lines, message)
	}
	for _, file := range post.Files {
		lines = append(lines, attachmentPlainLabel(file))
	}
	return strings.Join(lines, "\n")
}

func attachmentPlainLabel(file domain.PostFile) string {
	name := strings.TrimSpace(file.Name)
	if name == "" {
		name = "file"
		if file.ID != "" {
			name += " " + shortID(file.ID)
		}
	}
	var details []string
	if file.Size > 0 {
		details = append(details, formatFileSize(file.Size))
	}
	if file.MIMEType != "" {
		details = append(details, file.MIMEType)
	} else if file.Extension != "" {
		details = append(details, file.Extension)
	}
	if file.Width > 0 && file.Height > 0 {
		details = append(details, fmt.Sprintf("%d×%d", file.Width, file.Height))
	}
	if len(details) > 0 {
		return "📎 " + name + " (" + strings.Join(details, " · ") + ")"
	}
	return "📎 " + name
}

func formatQuotedReply(post domain.Post) string {
	message := strings.TrimSpace(postPlainText(post))
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
