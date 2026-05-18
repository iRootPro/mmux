package app

import (
	"fmt"
	"strings"

	"band-tui/internal/domain"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	colorText    = lipgloss.AdaptiveColor{Light: "235", Dark: "255"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "243", Dark: "246"}
	colorSubtle  = lipgloss.AdaptiveColor{Light: "250", Dark: "241"}
	colorAccent  = lipgloss.AdaptiveColor{Light: "62", Dark: "147"}
	colorError   = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "35", Dark: "114"}

	baseStyle        = lipgloss.NewStyle().Foreground(colorText)
	muted            = lipgloss.NewStyle().Foreground(colorMuted)
	accent           = lipgloss.NewStyle().Foreground(colorAccent)
	errorText        = lipgloss.NewStyle().Foreground(colorError)
	pillStyle        = lipgloss.NewStyle().Foreground(colorText).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"}).Padding(0, 1)
	selectedMsgStyle = lipgloss.NewStyle().Foreground(colorText)

	sidebarStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(colorSubtle).Padding(0, 1)
	mainStyle    = lipgloss.NewStyle().PaddingLeft(1)
	headerStyle  = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	boxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorSubtle).Padding(0, 1)
	focusStyle   = lipgloss.NewStyle().BorderForeground(colorAccent)
)

func (m Model) sidebarWidth() int {
	if m.width <= 70 {
		return 22
	}
	if m.width >= 140 {
		return 34
	}
	return 28
}

func (m Model) renderTeamSwitcher(width, height int) string {
	boxWidth := min(max(54, width/3), max(54, width-8))
	itemCount := m.teamSwitcherItemCount()
	boxHeight := min(max(8, itemCount+5), max(8, height-4))
	var b strings.Builder
	b.WriteString(headerStyle.Render("Scopes"))
	b.WriteString(muted.Render("  enter switch · esc close"))
	b.WriteString("\n\n")
	if itemCount == 0 {
		b.WriteString(muted.Render("No scopes available."))
		b.WriteString("\n")
	} else {
		limit := max(1, boxHeight-5)
		start := 0
		if m.teamSwitcherSelected >= limit {
			start = m.teamSwitcherSelected - limit + 1
		}
		if start > 0 {
			b.WriteString(muted.Render("…"))
			b.WriteString("\n")
		}
		for i := start; i < itemCount && i < start+limit; i++ {
			teamIndex := m.teamIndexForSwitcherItem(i)
			name := "All scopes"
			if teamIndex >= 0 {
				team := m.session.Teams[teamIndex]
				name = team.DisplayName
				if name == "" {
					name = team.Name
				}
				if name == "" {
					name = team.ID
				}
			}
			line := sanitizeTerminal(name)
			if teamIndex == m.selectedTeam {
				line += "  ✓"
			}
			if i == m.teamSwitcherSelected {
				line = pillStyle.Width(boxWidth - 4).Render(truncate(line, boxWidth-6))
			} else {
				line = muted.Render(truncate(line, boxWidth-4))
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		if start+limit < itemCount {
			b.WriteString(muted.Render("…"))
			b.WriteString("\n")
		}
	}
	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderInfo(width, height int) string {
	boxWidth := min(max(72, width*2/3), max(72, width-8))
	boxHeight := min(max(16, height*2/3), max(16, height-4))
	contentWidth := max(20, boxWidth-4)
	var b strings.Builder
	b.WriteString(headerStyle.Render("Info"))
	b.WriteString(muted.Render("  i/esc close"))
	b.WriteString("\n\n")
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		b.WriteString(muted.Render("No channel selected."))
	} else {
		ch := m.channels[m.selectedChannel]
		b.WriteString(m.renderInfoBody(ch, contentWidth))
	}
	content := fitHeight(b.String(), max(1, boxHeight-2))
	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderInfoBody(ch domain.Channel, width int) string {
	var b strings.Builder
	name := ch.DisplayName
	if name == "" {
		name = ch.Name
	}
	prefix := "#"
	typeLabel := "channel"
	switch ch.Type {
	case "D":
		prefix = "@"
		typeLabel = "direct message"
	case "G":
		prefix = "◦"
		typeLabel = "group message"
	case "P":
		typeLabel = "private channel"
	}
	b.WriteString(headerStyle.Render(prefix + " " + sanitizeTerminal(name)))
	b.WriteString("\n")
	b.WriteString(muted.Render(typeLabel))
	if ch.Status != "" {
		b.WriteString(muted.Render(" · "))
		b.WriteString(presenceGlyph(ch.Status))
		b.WriteString(muted.Render(statusLabel(ch.Status)))
	}
	b.WriteString("\n\n")
	facts := []string{}
	if ch.MemberCount > 0 {
		facts = append(facts, fmt.Sprintf("members: %d", ch.MemberCount))
	}
	if ch.Unread > 0 {
		facts = append(facts, fmt.Sprintf("unread: %d", ch.Unread))
	}
	if ch.Mentions > 0 {
		facts = append(facts, fmt.Sprintf("mentions: %d", ch.Mentions))
	}
	if ch.LastPostAt > 0 {
		facts = append(facts, "last post: "+formatDate(ch.LastPostAt)+" "+formatTime(ch.LastPostAt))
	}
	for _, fact := range facts {
		b.WriteString(muted.Render("• "))
		b.WriteString(sanitizeTerminal(fact))
		b.WriteString("\n")
	}
	if len(facts) > 0 {
		b.WriteString("\n")
	}
	if strings.TrimSpace(ch.Header) != "" {
		b.WriteString(headerStyle.Render("Header"))
		b.WriteString("\n")
		b.WriteString(renderMarkdownMessage(ch.Header, width))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(ch.Purpose) != "" && strings.TrimSpace(ch.Purpose) != strings.TrimSpace(ch.Header) {
		b.WriteString(headerStyle.Render("Purpose"))
		b.WriteString("\n")
		b.WriteString(renderMarkdownMessage(ch.Purpose, width))
		b.WriteString("\n\n")
	}
	b.WriteString(muted.Render("id: " + ch.ID))
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderActivity(width, height int) string {
	items := m.activityItems()
	boxWidth := min(max(72, width/2), max(72, width-8))
	visibleItems := min(max(1, len(items)), 12)
	boxHeight := min(max(8, visibleItems+5), max(8, height-4))
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("Mentions %d", len(items))))
	b.WriteString(muted.Render("  enter open · c clear · esc close"))
	b.WriteString("\n")
	if len(m.recentEvents) == 0 && len(items) > 0 {
		b.WriteString(muted.Render("No live mention text yet; showing channels with mentions."))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	limit := max(1, boxHeight-5)
	for i, item := range items {
		if i >= limit {
			b.WriteString(muted.Render("…"))
			b.WriteString("\n")
			break
		}
		line := m.renderActivityItem(item, boxWidth-4)
		if i == m.activitySelected {
			line = pillStyle.Width(boxWidth - 4).Render(truncate(line, boxWidth-6))
		} else {
			line = muted.Render(truncate(line, boxWidth-4))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(items) == 0 {
		b.WriteString(muted.Render("No personal/@all mentions."))
		b.WriteString("\n")
	}
	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, box)
}

func (m Model) renderTriage(width, height int) string {
	boxWidth := min(max(72, width/2), max(72, width-8))
	visibleItems := min(max(1, len(m.triageItems)), 12)
	boxHeight := min(max(8, visibleItems+5), max(8, height-4))
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("Triage %d", len(m.triageItems))))
	b.WriteString(muted.Render("  enter open · d done · esc close"))
	b.WriteString("\n\n")

	limit := max(1, boxHeight-5)
	start := 0
	if m.triageSelected >= limit {
		start = m.triageSelected - limit + 1
	}
	if start > 0 {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}
	for i := start; i < len(m.triageItems) && i < start+limit; i++ {
		line := m.renderTriageItem(m.triageItems[i], boxWidth-4)
		if i == m.triageSelected {
			line = pillStyle.Width(boxWidth - 4).Render(truncate(line, boxWidth-6))
		} else {
			line = muted.Render(truncate(line, boxWidth-4))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(m.triageItems) == 0 {
		b.WriteString(muted.Render("Nothing to triage."))
		b.WriteString("\n")
	} else if start+limit < len(m.triageItems) {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}

	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderTriageItem(item triageItem, width int) string {
	title := item.Title
	if title == "" {
		title = m.channelLabel(item.ChannelID)
	}
	freshness := ""
	if item.CreateAt > 0 {
		freshness = formatTime(item.CreateAt)
	}
	switch item.Kind {
	case triageMention:
		return truncate(triageLine("@", title, item.Actor, item.Preview, "", freshness), width)
	case triageThreadReply:
		return truncate(triageLine("↳", title, item.Actor, item.Preview, "", freshness), width)
	case triageUnreadChannel:
		count := ""
		if item.UnreadCount > 0 {
			count = fmt.Sprintf("%d unread", item.UnreadCount)
		}
		return truncate(triageLine("•", title, item.Actor, item.Preview, count, freshness), width)
	default:
		return truncate(triageLine("•", title, item.Actor, item.Preview, "", freshness), width)
	}
}

func triageLine(prefix, title, actor, preview, fallback, freshness string) string {
	var parts []string
	if title != "" {
		parts = append(parts, title)
	}
	if actor != "" {
		parts = append(parts, actor)
	}
	if preview != "" {
		parts = append(parts, preview)
	} else if fallback != "" {
		parts = append(parts, fallback)
	}
	if freshness != "" {
		parts = append(parts, freshness)
	}
	if len(parts) == 0 {
		return prefix
	}
	return prefix + " " + strings.Join(parts, " · ")
}

func (m Model) renderActivityItem(item activityItem, width int) string {
	if item.HasPost {
		return m.renderActivityLine(item.Post, width)
	}
	idx := m.channelIndexByID(item.ChannelID)
	if idx < 0 {
		return truncate("unknown unread channel", width)
	}
	ch := m.channels[idx]
	label := m.channelLabel(ch.ID)
	return truncate(fmt.Sprintf("%s · @%d", label, ch.Mentions), width)
}

func (m Model) renderActivityLine(post domain.Post, width int) string {
	ch := "unknown"
	if idx := m.channelIndexByID(post.ChannelID); idx >= 0 {
		ch = m.channels[idx].DisplayName
		if ch == "" {
			ch = m.channels[idx].Name
		}
		if m.channels[idx].Type == "D" {
			ch = "@" + ch
		} else {
			ch = "#" + ch
		}
	}
	user := post.Username
	if user == "" {
		user = shortID(post.UserID)
	}
	prefix := ""
	if post.RootID != "" {
		prefix = "↳ "
	}
	msg := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(post.Message)), "\n", " ")
	line := fmt.Sprintf("%s%s · %s · %s", prefix, ch, user, msg)
	return truncate(line, width)
}

func (m Model) renderThreadLayout(width, height int) string {
	if width < 120 {
		return m.renderThreadOverlay(width, height)
	}
	bodyHeight := height - 2
	sidebarWidth := m.sidebarWidth()
	threadWidth := min(max(46, width/3), 72)
	mainWidth := max(30, width-sidebarWidth-threadWidth-2)
	if mainWidth < 30 {
		return m.renderThreadOverlay(width, height)
	}

	dividerWidth := 1
	rightWidth := mainWidth + dividerWidth + threadWidth
	composer := m.renderThreadComposer(rightWidth)
	composerHeight := lipgloss.Height(composer)
	topHeight := max(8, bodyHeight-composerHeight)

	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	timeline := m.renderMain(mainWidth, topHeight)
	divider := verticalDivider(topHeight)
	thread := m.renderThreadPanel(threadWidth, topHeight)
	top := lipgloss.JoinHorizontal(lipgloss.Top, timeline, divider, thread)
	right := lipgloss.JoinVertical(lipgloss.Left, top, composer)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	return body + "\n" + m.renderStatus(width)
}

func (m Model) renderThreadPanel(width, height int) string {
	header := m.renderThreadHeader(max(20, width-4))
	contentHeight := max(3, height-lipgloss.Height(header)-1)
	vp := m.threadViewport
	vp.Width = max(20, width-4)
	vp.Height = contentHeight
	contentBorder := colorSubtle
	if !m.threadFocusComposer {
		contentBorder = colorAccent
	}
	content := lipgloss.NewStyle().Height(contentHeight).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(contentBorder).PaddingLeft(1).Render(vp.View())
	panel := fitHeight(lipgloss.JoinVertical(lipgloss.Left, header, "", content), height)
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).Render(panel)
}

func verticalDivider(height int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = muted.Render("│")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderThreadOverlay(width, height int) string {
	boxWidth := min(max(70, width*3/4), max(70, width-6))
	boxHeight := min(max(16, height*3/4), max(16, height-4))
	composer := m.renderThreadComposer(boxWidth - 2)
	header := m.renderThreadHeader(max(20, boxWidth-4))
	contentHeight := max(3, boxHeight-lipgloss.Height(header)-lipgloss.Height(composer)-4)
	vp := m.threadViewport
	vp.Width = max(20, boxWidth-4)
	vp.Height = contentHeight
	content := lipgloss.NewStyle().Height(contentHeight).Render(vp.View())
	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(lipgloss.JoinVertical(lipgloss.Left, header, "", content, composer))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderThreadHeader(width int) string {
	replies := m.threadReplyCount()
	title := fmt.Sprintf("Thread · %d %s", replies, plural(replies, "reply", "replies"))
	if m.threadLoading {
		title = "Thread · loading…"
	}
	help := "tab reply · esc close · j/k scroll"
	if m.threadFocusComposer {
		help = "tab messages · enter reply · ctrl+j newline"
	}
	rootText := "root: loading…"
	if root, ok := m.threadRootPost(); ok {
		user := root.Username
		if user == "" {
			user = shortID(root.UserID)
		}
		text := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(root.Message)), "\n", " ")
		if text == "" {
			text = "(empty)"
		}
		rootText = "root: " + user + " · " + text
	} else if !m.threadLoading {
		rootText = "root: unavailable"
	}
	// Keep the first physical row empty, like the main header workaround. Some
	// terminal/ANSI combinations in this layout consistently drop the first row;
	// putting the important content on the second row makes it reliable.
	info := headerStyle.Render(title) + muted.Render(" · ") + muted.Render(rootText)
	info = ansi.Truncate(info, width, "…")
	helpLine := muted.Render(truncate(help, width))
	return strings.Join([]string{"", info, helpLine}, "\n")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func replyCountLabel(count int) string {
	return fmt.Sprintf("  ↳ %d %s", count, plural(count, "reply", "replies"))
}

func (m Model) renderThreadComposer(width int) string {
	labelText := m.threadComposerLabel(max(10, width-4))
	placeholder := "Write a reply…"
	if !m.threadFocusComposer {
		labelText = truncate("reply composer inactive · tab reply", max(10, width-4))
		placeholder = ""
	}
	label := muted.Render(labelText)
	composer := m.composer
	composer.SetWidth(max(1, width-4))
	composer.SetHeight(3)
	composer.Placeholder = placeholder
	style := lipgloss.NewStyle().Width(width-1).Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(colorSubtle).Padding(0, 1)
	if m.threadFocusComposer {
		style = style.BorderForeground(colorAccent)
	}
	return style.Render(label + "\n" + composer.View())
}

func (m Model) threadComposerLabel(width int) string {
	prefix := "reply in thread"
	if root, ok := m.threadRootPost(); ok {
		user := root.Username
		if user == "" {
			user = shortID(root.UserID)
		}
		text := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(root.Message)), "\n", " ")
		if text != "" {
			prefix = "reply to: " + user + " · " + text
		} else {
			prefix = "reply to: " + user
		}
	}
	return truncate(prefix+" · enter send · ctrl+j newline", width)
}

func (m Model) threadRootPost() (domain.Post, bool) {
	for _, post := range m.threadPosts {
		if post.ID == m.threadRootID || post.RootID == "" {
			return post, true
		}
	}
	return domain.Post{}, false
}

func (m Model) threadReplyCount() int {
	count := 0
	for _, post := range m.threadPosts {
		if post.ID != m.threadRootID && post.RootID != "" {
			count++
		}
	}
	return count
}

func (m Model) threadHasReplies() bool {
	return m.threadReplyCount() > 0
}

func (m Model) renderThreadPosts(width int) string {
	var b strings.Builder
	repliesWritten := 0
	replyCount := m.threadReplyCount()
	var prevReply domain.Post
	hasPrevReply := false
	for idx, post := range m.threadPosts {
		if post.ID == m.threadRootID || post.RootID == "" {
			continue
		}
		grouped := hasPrevReply && shouldGroupTimelinePost(prevReply, post)
		if !grouped {
			user := sanitizeTerminal(post.Username)
			if user == "" {
				user = sanitizeTerminal(shortID(post.UserID))
			}
			if post.Unread || post.Mentioned || post.ThreadUnread {
				b.WriteString(accent.Render("● "))
			}
			b.WriteString(accent.Render(user))
			b.WriteString(muted.Render("  " + formatTime(post.CreateAt)))
			if post.ThreadUnread {
				b.WriteString(accent.Render("  ● new replies"))
			}
			b.WriteString("\n")
		}
		body := renderMarkdownMessage(post.Message, max(20, width-4))
		for _, line := range strings.Split(body, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		repliesWritten++
		prevReply = post
		hasPrevReply = true
		if repliesWritten < replyCount && !nextThreadReplyGroups(m.threadPosts, idx+1, m.threadRootID, post) {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func nextThreadReplyGroups(posts []domain.Post, start int, rootID string, current domain.Post) bool {
	for i := start; i < len(posts); i++ {
		next := posts[i]
		if next.ID == rootID || next.RootID == "" {
			continue
		}
		return shouldGroupTimelinePost(current, next)
	}
	return false
}

func (m Model) renderSwitcher(width, height int) string {
	boxWidth := min(max(50, width*2/3), max(50, width-8))
	boxHeight := min(max(12, height*2/3), max(12, height-4))
	indexes := m.switcherIndexes()

	var b strings.Builder
	query := m.switcherQuery
	if query == "" {
		query = "type to search…"
	}
	b.WriteString(headerStyle.Render("Jump to channel"))
	b.WriteString("\n")
	b.WriteString(muted.Render("ctrl+p/ctrl+k · enter open · esc close"))
	b.WriteString("\n\n")
	b.WriteString(accent.Render("› "))
	b.WriteString(sanitizeTerminal(query))
	b.WriteString("\n\n")

	limit := max(1, boxHeight-7)
	start := 0
	if m.switcherSelected >= limit {
		start = m.switcherSelected - limit + 1
	}
	if start > 0 {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}
	for pos := start; pos < len(indexes) && pos < start+limit; pos++ {
		idx := indexes[pos]
		line := m.renderSwitcherLine(idx, boxWidth-4)
		if pos == m.switcherSelected {
			line = pillStyle.Width(boxWidth - 4).Render(truncate(line, boxWidth-6))
		} else {
			line = muted.Render(truncate(line, boxWidth-4))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(indexes) == 0 {
		b.WriteString(muted.Render("No matches"))
		b.WriteString("\n")
	} else if start+limit < len(indexes) {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}

	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderSwitcherLine(index, width int) string {
	ch := m.channels[index]
	prefix := "#"
	if ch.Type == "D" {
		prefix = "@"
	} else if ch.Type == "G" {
		prefix = "◦"
	}
	name := sanitizeTerminal(ch.DisplayName)
	if name == "" {
		name = sanitizeTerminal(ch.Name)
	}
	section := "channels"
	if m.favoriteChannels[ch.ID] {
		section = "favorite"
	} else if ch.Type == "D" {
		section = "direct"
	} else if ch.Type == "G" {
		section = "group"
	}
	line := fmt.Sprintf("%s%s %s", presenceGlyphPlain(ch.Status), prefix, name)
	if m.favoriteChannels[ch.ID] {
		line += " ★"
	}
	if scope := m.scopeSuffix(ch); scope != "" {
		line += " · " + scope
	}
	if ch.Mentions > 0 {
		line += fmt.Sprintf("  @%d", ch.Mentions)
	} else if ch.Unread > 0 {
		line += "  •"
	}
	line += muted.Render("  · " + section)
	return truncate(line, width)
}

func (m Model) scopeSuffix(ch domain.Channel) string {
	if m.selectedTeam != allScopesTeamIndex || (ch.Type == "D" || ch.Type == "G") {
		return ""
	}
	return m.teamDisplayName(ch.TeamID)
}

func (m Model) teamDisplayName(teamID string) string {
	if m.session == nil || teamID == "" {
		return ""
	}
	for _, team := range m.session.Teams {
		if team.ID == teamID {
			if team.DisplayName != "" {
				return team.DisplayName
			}
			if team.Name != "" {
				return team.Name
			}
			return team.ID
		}
	}
	return ""
}

func presenceGlyph(status string) string {
	switch status {
	case "online":
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("● ")
	case "away":
		return lipgloss.NewStyle().Foreground(colorMuted).Render("◐ ")
	case "dnd":
		return errorText.Render("● ")
	case "offline":
		return muted.Render("○ ")
	default:
		return ""
	}
}

func presenceGlyphPlain(status string) string {
	switch status {
	case "online":
		return "● "
	case "away":
		return "◐ "
	case "dnd":
		return "● "
	case "offline":
		return "○ "
	default:
		return ""
	}
}

func (m Model) renderSidebar(width, height int) string {
	innerWidth := max(10, width-3)

	headerLines := []string{accent.Bold(true).Render("scope: " + m.sidebarTitle())}
	if m.session != nil {
		name := m.session.User.DisplayName
		if name == "" {
			name = "@" + m.session.User.Username
		}
		headerLines = append(headerLines, muted.Render(name))
	} else {
		headerLines = append(headerLines, muted.Render("connecting"))
	}
	if m.filtering || m.channelFilter != "" {
		cursor := ""
		if m.filtering {
			cursor = "_"
		}
		headerLines = append(headerLines, muted.Render("/ "+sanitizeTerminal(m.channelFilter)+cursor))
	} else {
		headerLines = append(headerLines, muted.Render("/ filter · F2 scopes"))
	}
	headerLines = append(headerLines, "")

	listItems, selectedLine := m.sidebarListLines(innerWidth)
	available := max(0, height-len(headerLines))
	listLines := cropSidebarLines(listItems, selectedLine, available)

	lines := append(headerLines, listLines...)
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	style := sidebarStyle.Width(width - 1).Height(height)
	if m.focus == focusSidebar {
		style = style.BorderForeground(colorAccent)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) sidebarTitle() string {
	if m.selectedTeam == allScopesTeamIndex {
		return "All scopes"
	}
	userName := ""
	if m.session != nil {
		userName = strings.TrimSpace(m.session.User.DisplayName)
		if userName == "" {
			userName = strings.TrimSpace(m.session.User.Username)
		}
	}
	if m.session != nil && len(m.session.Teams) > 0 && m.selectedTeam >= 0 && m.selectedTeam < len(m.session.Teams) {
		title := strings.TrimSpace(m.session.Teams[m.selectedTeam].DisplayName)
		if title == "" {
			title = strings.TrimSpace(m.session.Teams[m.selectedTeam].Name)
		}
		// Some Band deployments can return a user-like team title. Keep the
		// workspace line stable and distinct from the user line to avoid the header
		// appearing to lose a row after channels load.
		if title != "" && title != userName {
			return title
		}
	}
	if strings.Contains(strings.ToLower(m.cfg.ServerURL), "wb.ru") {
		return "WB"
	}
	return "band"
}

type sidebarSection struct {
	ID    string
	Title string
	Types []string
}

type sidebarLine struct {
	Text    string
	Section string
	Header  bool
}

func (m Model) sidebarListLines(width int) ([]sidebarLine, int) {
	indexes := m.matchingChannelIndexes()
	if len(m.channels) == 0 {
		return []sidebarLine{{Text: muted.Render("no channels yet")}}, -1
	}
	if len(indexes) == 0 {
		return []sidebarLine{{Text: muted.Render("no matches")}}, -1
	}

	sections := []sidebarSection{
		{ID: sectionFavorites, Title: "ИЗБРАННОЕ"},
		{ID: sectionChannels, Title: "КАНАЛЫ", Types: []string{"O", "P"}},
		{ID: sectionDirect, Title: "ЛИЧНЫЕ", Types: []string{"D"}},
		{ID: sectionGroups, Title: "ГРУППЫ", Types: []string{"G"}},
	}

	var lines []sidebarLine
	selectedLine := -1
	for _, section := range sections {
		collapsed := !m.filtering && m.isSectionCollapsed(section.ID)
		sectionLines := make([]sidebarLine, 0)
		for _, idx := range indexes {
			if !m.channelInSection(idx, section.ID) {
				continue
			}
			line := m.renderSidebarChannelLine(idx, width)
			if idx == m.selectedChannel && !collapsed {
				selectedLine = len(lines) + 1 + len(sectionLines)
			}
			sectionLines = append(sectionLines, sidebarLine{Text: line, Section: section.Title})
		}
		if len(sectionLines) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, sidebarLine{})
		}
		chevron := "▾"
		if collapsed {
			chevron = "▸"
		}
		lines = append(lines, sidebarLine{Text: muted.Render(fmt.Sprintf("%s %s %d", chevron, section.Title, len(sectionLines))), Section: section.Title, Header: true})
		if !collapsed {
			lines = append(lines, sectionLines...)
		}
	}
	return lines, selectedLine
}

func (m Model) renderSidebarChannelLine(index, width int) string {
	ch := m.channels[index]
	marker := "  "
	style := muted
	if index == m.selectedChannel {
		marker = "› "
		style = accent.Bold(true)
	}
	prefix := "#"
	if ch.Type == "D" {
		prefix = "@"
	} else if ch.Type == "G" {
		prefix = "◦"
	}
	name := sanitizeTerminal(ch.DisplayName)
	if name == "" {
		name = sanitizeTerminal(ch.Name)
	}
	label := marker + presenceGlyphPlain(ch.Status) + prefix + " " + name
	if m.favoriteChannels[ch.ID] {
		label += " ★"
	}
	if scope := m.scopeSuffix(ch); scope != "" {
		label += " · " + scope
	}
	badge := channelBadge(ch)
	if index == m.selectedChannel {
		contentWidth := max(0, width-2)
		line := joinLabelAndBadge(label, badge, contentWidth)
		return pillStyle.Width(contentWidth).Render(line)
	}
	line := joinLabelAndBadge(label, badge, width)
	return style.Render(line)
}

func channelBadge(ch domain.Channel) string {
	if ch.Mentions > 0 {
		return fmt.Sprintf("@%d", ch.Mentions)
	}
	if ch.Unread > 0 {
		return fmt.Sprintf("%d", ch.Unread)
	}
	return ""
}

func joinLabelAndBadge(label, badge string, width int) string {
	if width <= 0 {
		return ""
	}
	if badge == "" {
		return truncate(label, width)
	}
	badgeWidth := lipgloss.Width(badge)
	if badgeWidth >= width {
		return truncate(badge, width)
	}
	labelWidth := max(0, width-badgeWidth-1)
	left := truncate(label, labelWidth)
	spaces := max(1, width-lipgloss.Width(left)-badgeWidth)
	return left + strings.Repeat(" ", spaces) + badge
}

func cropSidebarLines(items []sidebarLine, selected, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(items) <= height {
		return sidebarLineTexts(items)
	}
	start := 0
	if selected >= 0 {
		start = selected - height/2
	}
	if start < 0 {
		start = 0
	}
	if start+height > len(items) {
		start = len(items) - height
	}
	cropped := append([]sidebarLine(nil), items[start:start+height]...)
	if start > 0 && len(cropped) > 0 {
		section := cropped[0].Section
		if section == "" && start > 0 {
			section = items[start-1].Section
		}
		if height >= 4 && section != "" && !cropped[0].Header {
			cropped[0] = sidebarLine{Text: muted.Render("▾ " + section), Section: section, Header: true}
			if len(cropped) > 1 && selected != start+1 {
				hiddenAbove := hiddenSidebarItemCount(items, 0, start+2)
				cropped[1] = sidebarLine{Text: muted.Render(hiddenItemsLabel("↑", hiddenAbove, section)), Section: section}
			}
		} else if selected != start {
			hiddenAbove := hiddenSidebarItemCount(items, 0, start+1)
			cropped[0] = sidebarLine{Text: muted.Render(hiddenItemsLabel("↑", hiddenAbove, section)), Section: section}
		}
	}
	if start+height < len(items) && len(cropped) > 0 {
		labelIndex := len(cropped) - 1
		labelItemIndex := start + labelIndex
		if selected != labelItemIndex {
			section := cropped[labelIndex].Section
			if section == "" && start+height < len(items) {
				section = items[start+height].Section
			}
			hiddenBelow := hiddenSidebarItemCount(items, labelItemIndex, len(items))
			cropped[labelIndex] = sidebarLine{Text: muted.Render(hiddenItemsLabel("↓", hiddenBelow, section)), Section: section}
		}
	}
	return sidebarLineTexts(cropped)
}

func sidebarLineTexts(items []sidebarLine) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Text)
	}
	return out
}

func hiddenItemsLabel(direction string, count int, section string) string {
	if count <= 0 {
		return ""
	}
	label := direction + " ещё " + fmt.Sprint(count)
	if section != "" {
		label += " · " + strings.ToLower(section)
	}
	return label
}

func hiddenSidebarItemCount(items []sidebarLine, start, end int) int {
	if start < 0 {
		start = 0
	}
	if end > len(items) {
		end = len(items)
	}
	count := 0
	for i := start; i < end; i++ {
		if items[i].Header || items[i].Text == "" {
			continue
		}
		count++
	}
	return count
}

func (m Model) renderMain(width, height int) string {
	header := m.renderHeader(width)
	composer := ""
	composerHeight := 0
	if !m.threadOpen {
		composer = m.renderComposer(width)
		composerHeight = lipgloss.Height(composer)
	}
	separator := muted.Render(strings.Repeat("─", max(0, width-2)))
	viewportHeight := max(3, height-lipgloss.Height(header)-composerHeight-lipgloss.Height(separator))
	viewportStyle := lipgloss.NewStyle().Width(width-1).Height(viewportHeight).Padding(0, 1)
	if m.focus == focusTimeline && !m.threadOpen {
		viewportStyle = viewportStyle.Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(colorAccent).PaddingLeft(1)
	}
	viewportBox := viewportStyle.Render(m.viewport.View())
	parts := []string{header, separator, viewportBox}
	if composer != "" {
		parts = append(parts, composer)
	}
	return mainStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m Model) renderHeader(width int) string {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return lipgloss.NewStyle().Width(width-1).Padding(0, 1).Render(headerStyle.Render("no channel"))
	}
	ch := m.channels[m.selectedChannel]
	name := sanitizeTerminal(ch.DisplayName)
	if name == "" {
		name = sanitizeTerminal(ch.Name)
	}
	prefix := "#"
	switch ch.Type {
	case "D":
		prefix = "@"
	case "G":
		prefix = "◦"
	}
	contentWidth := max(1, width-4)
	titleLine := renderHeaderLeft(prefix, name, presenceGlyph(ch.Status), m.channelMeta(ch), contentWidth)
	if titleLine == "" {
		titleLine = headerStyle.Render(prefix + " " + name)
	}
	topic := strings.TrimSpace(ch.Header)
	if topic == "" {
		topic = strings.TrimSpace(ch.Purpose)
	}
	visibleLine := titleLine
	if lipgloss.Width(titleLine)+3 < contentWidth {
		if topicLine := m.renderHeaderTopic(topic, max(1, contentWidth-lipgloss.Width(titleLine)-3)); topicLine != "" {
			visibleLine += muted.Render(" — ") + topicLine
		} else if m.mockFallback {
			visibleLine += muted.Render(" — mock backend")
		}
	}
	// Keep header height stable at two rows, but put all important information on
	// the second row. Some terminal/ANSI combinations were consistently losing
	// the first row when the topic contained styled links; this makes the visible
	// row robust while preserving the same vertical footprint.
	visibleLine = ansi.Truncate(visibleLine, contentWidth, "…")
	spacerRow := lipgloss.NewStyle().Width(width-1).Padding(0, 1).Render("")
	infoRow := lipgloss.NewStyle().Width(width-1).Padding(0, 1).Render(visibleLine)
	return lipgloss.JoinVertical(lipgloss.Left, spacerRow, infoRow)
}

func renderHeaderLeft(prefix, name, presence, meta string, width int) string {
	if width <= 0 {
		return ""
	}
	prefixText := prefix + " "
	metaText := ""
	if meta != "" {
		metaText = "  " + meta
	}
	nameBudget := width - lipgloss.Width(presence) - lipgloss.Width(prefixText) - lipgloss.Width(metaText)
	if nameBudget < 8 && metaText != "" {
		metaText = ""
		nameBudget = width - lipgloss.Width(presence) - lipgloss.Width(prefixText)
	}
	name = truncate(name, max(1, nameBudget))
	left := presence + headerStyle.Render(prefixText+name)
	if metaText != "" {
		left += muted.Render(metaText)
	}
	return left
}

func (m Model) renderHeaderTopic(topic string, width int) string {
	if strings.TrimSpace(topic) == "" || width <= 0 {
		return ""
	}
	// Header topic must be exactly one physical row. Channel descriptions can be
	// huge markdown/code blocks; rendering them with wrapping makes the whole TUI
	// jump when browsing the sidebar. Render inline markdown, collapse newlines,
	// then ANSI-safely truncate to the available budget.
	rendered := renderMarkdownMessage(topic, 0)
	rendered = strings.Join(strings.FieldsFunc(rendered, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t'
	}), " ")
	return ansi.Truncate(rendered, width, "…")
}

func (m Model) channelMeta(ch domain.Channel) string {
	parts := make([]string, 0, 4)
	if ch.MemberCount > 0 {
		if ch.Type == "D" {
			parts = append(parts, statusLabel(ch.Status))
		} else {
			parts = append(parts, fmt.Sprintf("%d members", ch.MemberCount))
		}
	} else if ch.Status != "" {
		parts = append(parts, statusLabel(ch.Status))
	}
	if len(m.posts) > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", len(m.posts), plural(len(m.posts), "message", "messages")))
	}
	if ch.Mentions > 0 {
		parts = append(parts, fmt.Sprintf("@%d", ch.Mentions))
	} else if ch.Unread > 0 {
		parts = append(parts, "unread")
	}
	return strings.Join(nonEmpty(parts), " · ")
}

func statusLabel(status string) string {
	switch status {
	case "online":
		return "online"
	case "away":
		return "away"
	case "dnd":
		return "dnd"
	case "offline":
		return "offline"
	default:
		return ""
	}
}

func nonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (m Model) renderComposer(width int) string {
	labelText := m.composerLabel(max(10, width-4))
	if m.focus != focusComposer {
		labelText = truncate("composer inactive · tab focus input", max(10, width-4))
	}
	label := muted.Render(labelText)
	// Always use textarea.View(), including for the placeholder. Render a local
	// copy with the target width so the same composer can be drawn in the main
	// pane, thread side pane, or thread overlay without overflowing or changing
	// layout metrics between frames.
	composer := m.composer
	composer.SetWidth(max(1, width-4))
	composer.SetHeight(3)
	content := composer.View()
	style := lipgloss.NewStyle().Width(width-1).Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(colorSubtle).Padding(0, 1)
	if m.focus == focusComposer {
		style = style.BorderForeground(colorAccent)
	}
	return style.Render(label + "\n" + content)
}

func (m Model) composerLabel(width int) string {
	target := "current channel"
	if len(m.channels) > 0 && m.selectedChannel >= 0 && m.selectedChannel < len(m.channels) {
		ch := m.channels[m.selectedChannel]
		name := sanitizeTerminal(ch.DisplayName)
		if name == "" {
			name = sanitizeTerminal(ch.Name)
		}
		prefix := "#"
		switch ch.Type {
		case "D":
			prefix = "@"
		case "G":
			prefix = "◦"
		}
		if name != "" {
			target = prefix + " " + name
		}
	}
	return truncate("to "+target+" · enter send · ctrl+j newline", width)
}

func (m Model) renderStatus(width int) string {
	status := m.status
	if m.threadOpen {
		status = m.threadStatusLine()
	} else {
		if status == "" {
			status = "ready"
		}
		if m.loading {
			status = "• " + status
		}
		if hint := m.focusStatusHint(); hint != "" {
			status += "  " + muted.Render(hint)
		}
	}
	if net := m.connectionStatusText(); net != "" {
		status = "net: " + net + " · " + status
	}
	if scope := m.sidebarTitle(); scope != "" {
		status = "scope: " + scope + " · " + status
	}
	if strings.Contains(status, "connected") || strings.Contains(status, "sent") {
		status = lipgloss.NewStyle().Foreground(colorSuccess).Render(status)
	}
	if m.err != "" {
		status += "  " + errorText.Render(m.err)
	}
	if badge := m.notificationBadge(); badge != "" {
		status += "  " + accent.Render(badge)
	}
	return lipgloss.NewStyle().Foreground(colorMuted).Width(width).Padding(0, 1).Render(truncate(status, width-2))
}

func (m Model) focusStatusHint() string {
	switch m.focus {
	case focusSidebar:
		return "sidebar · enter open · / filter · tab timeline"
	case focusTimeline:
		return "timeline · j/k select · t thread · y copy · n unread"
	case focusComposer:
		return m.timelinePositionLabel()
	default:
		return "? help"
	}
}

func (m Model) threadStatusLine() string {
	count := "loading…"
	if !m.threadLoading {
		count = fmt.Sprintf("%d messages", len(m.threadPosts))
	}
	if m.threadFocusComposer {
		return "thread reply · " + count + " · tab messages · esc close"
	}
	return "thread messages · " + count + " · tab reply · esc close"
}

func (m Model) timelinePositionLabel() string {
	if m.viewport.TotalLineCount() == 0 || m.viewport.AtBottom() {
		return "at latest"
	}
	return "scrolled"
}

func (m Model) notificationBadge() string {
	mentions := m.mentionTotal()
	parts := make([]string, 0, 2)
	if len(m.recentEvents) > 0 {
		parts = append(parts, fmt.Sprintf("mentions %d", len(m.recentEvents)))
	}
	if mentions > 0 {
		parts = append(parts, fmt.Sprintf("@%d", mentions))
	}
	return strings.Join(parts, " · ")
}

func (m Model) mentionTotal() (mentions int) {
	for _, ch := range m.channels {
		mentions += ch.Mentions
	}
	return mentions
}

const messageGroupWindowMillis int64 = 5 * 60 * 1000

func shouldGroupTimelinePost(prev, current domain.Post) bool {
	if isImportantPost(current) || current.ReplyCount > 0 {
		return false
	}
	if prev.CreateAt <= 0 || current.CreateAt <= 0 {
		return false
	}
	if formatDate(prev.CreateAt) != formatDate(current.CreateAt) {
		return false
	}
	delta := current.CreateAt - prev.CreateAt
	if delta < 0 {
		delta = -delta
	}
	if delta > messageGroupWindowMillis {
		return false
	}
	return samePostAuthor(prev, current)
}

func samePostAuthor(a, b domain.Post) bool {
	if a.UserID != "" || b.UserID != "" {
		return a.UserID != "" && a.UserID == b.UserID
	}
	return a.Username != "" && a.Username == b.Username
}

func (m Model) renderPosts() (string, []int) {
	if len(m.posts) == 0 {
		if m.loading || strings.Contains(m.status, "loading") || strings.Contains(m.status, "refreshing") {
			return muted.Render("Loading messages…"), nil
		}
		return muted.Render("No messages yet."), nil
	}
	var b strings.Builder
	lineNo := 0
	writeLine := func(s string) {
		b.WriteString(s)
		b.WriteString("\n")
		lineNo++
	}
	writeBlank := func() { writeLine("") }

	if m.loadingOlder {
		writeLine(muted.Render("Loading older messages…"))
		writeBlank()
	} else if !m.hasOlder {
		writeLine(muted.Render("Beginning of history"))
		writeBlank()
	} else {
		writeLine(muted.Render("Press [ to load older messages"))
		writeBlank()
	}

	offsets := make([]int, len(m.posts))
	lastDate := ""
	importantIndex := firstImportantPostIndex(m.posts)
	for i, p := range m.posts {
		date := formatDate(p.CreateAt)
		if date != "" && date != lastDate {
			if lastDate != "" {
				writeBlank()
			}
			writeLine(muted.Render("──── " + date + " "))
			writeBlank()
			lastDate = date
		}
		if i == importantIndex {
			writeLine(accent.Render("──── new messages " + strings.Repeat("─", max(0, m.viewport.Width-20))))
			writeBlank()
		}
		grouped := i > 0 && shouldGroupTimelinePost(m.posts[i-1], p)
		offsets[i] = lineNo
		selected := i == m.selectedPost && m.focus == focusTimeline
		if !grouped {
			user := sanitizeTerminal(p.Username)
			if user == "" {
				user = sanitizeTerminal(shortID(p.UserID))
			}
			header := accent.Render(user) + muted.Render("  "+formatTime(p.CreateAt))
			if p.Unread || p.Mentioned || p.ThreadUnread {
				header = accent.Render("● ") + header
			}
			if p.ReplyCount > 0 {
				header += accent.Render(replyCountLabel(p.ReplyCount))
				if p.ThreadUnread {
					header += accent.Render("  ● new replies")
				}
			}
			writeLine(m.renderPostLine(header, selected))
		}
		body := renderMarkdownMessage(p.Message, max(20, m.viewport.Width-6))
		for _, line := range strings.Split(body, "\n") {
			writeLine(m.renderPostLine(baseStyle.Render(line), selected))
		}
		if i < len(m.posts)-1 && !shouldGroupTimelinePost(p, m.posts[i+1]) {
			writeBlank()
		}
	}
	return strings.TrimRight(b.String(), "\n"), offsets
}

func firstImportantPostIndex(posts []domain.Post) int {
	for i, post := range posts {
		if isImportantPost(post) {
			return i
		}
	}
	return -1
}

func (m Model) renderPostLine(line string, selected bool) string {
	if !selected {
		return "  " + line
	}
	// Keep selection height/width stable: a full-width background looked noisy and
	// wrapped oddly with ANSI/markdown content in narrow split-thread layouts.
	return accent.Render("▌ ") + selectedMsgStyle.Render(line)
}

func (m Model) helpText() string {
	return strings.TrimSpace(`Keys

  ctrl+p / ctrl+k   quick switcher
  /                 filter channels
  tab / shift+tab   switch focus
  j/k or arrows     move in sidebar / timeline
  n / N             next / previous unread or mention
  a                 open mentions inbox (@you/@all/@channel/@here)
  u                 open triage inbox (mentions/unread/thread replies)
                    triage: enter open · d done · n/N move · esc close
  i                 open channel/person info (when not typing)
  F2 / ctrl+g       switch scope/team/workspace
  w / T             switch scope when not typing
  f                 toggle favorite in sidebar
  s/c/d/g or 0/1/2/3 jump to favorites/channels/direct/groups
  pgup/pgdown       jump between sections
  left/right        collapse/expand current section
  space or x        toggle current section
  enter             send message or open selected channel
  ctrl+j            insert newline in composer
  [ or ctrl+o       load older messages
  y                 copy selected message text
  p                 copy selected message permalink
  o / enter         open first link in selected message
  r                 quote selected message into composer
  e                 edit selected own message
  D                 delete selected own message (press twice)
  t                 open thread for selected message
  ctrl+r            reload current channel or reconnect when offline
  ?                 toggle help
  q / ctrl+c        quit

Config

  BAND_URL=https://band.wb.ru
  BAND_TOKEN=...

or ~/.config/band-tui/config.json with server_url and token.`)
}

func channelGroup(channelType string) string {
	switch channelType {
	case "D":
		return "direct"
	case "G":
		return "groups"
	default:
		return "channels"
	}
}

func sanitizeTerminal(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\x1b' || r == '\u009b' {
			return -1
		}
		if r < 32 && r != '\t' {
			return -1
		}
		if r >= 0x7f && r <= 0x9f {
			return -1
		}
		return r
	}, s)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

func fitHeight(s string, height int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
