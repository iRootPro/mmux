package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	subtle           = lipgloss.NewStyle().Foreground(colorSubtle)
	accent           = lipgloss.NewStyle().Foreground(colorAccent)
	errorText        = lipgloss.NewStyle().Foreground(colorError)
	pillStyle        = lipgloss.NewStyle().Foreground(colorText).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"}).Padding(0, 1)
	selectedMsgStyle = lipgloss.NewStyle().Foreground(colorText)

	sidebarStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(colorSubtle).Padding(0, 1)
	mainStyle    = lipgloss.NewStyle().PaddingLeft(1)
	headerStyle  = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	boxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorSubtle).Padding(0, 1)
	focusStyle   = lipgloss.NewStyle().BorderForeground(colorAccent)

	statusBarStyle     = lipgloss.NewStyle().Foreground(colorMuted).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "234"}).Width(1)
	statusChipBase     = lipgloss.NewStyle().Foreground(colorText).Background(lipgloss.AdaptiveColor{Light: "255", Dark: "236"}).Padding(0, 1)
	statusKeyChip      = lipgloss.NewStyle().Foreground(colorAccent).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "235"}).Padding(0, 1)
	sidebarBadgeStyle  = lipgloss.NewStyle().Foreground(colorText).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"}).Padding(0, 1)
	sidebarMentionPill = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "230"}).Background(colorAccent).Padding(0, 1)
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

func (m Model) renderSettings(width, height int) string {
	boxWidth := min(max(78, width/2), max(78, width-8))
	boxHeight := min(18, max(12, height-4))
	var b strings.Builder
	b.WriteString(headerStyle.Render(m.tr("Settings")))
	b.WriteString(muted.Render("  ↑/↓ " + m.tr("move") + " · enter " + m.tr("edit/save") + " · esc " + m.tr("close")))
	b.WriteString("\n\n")

	rows := []string{
		m.settingsRow(settingsItemLanguage, "🌍 "+m.tr("Language"), m.languageDisplayName(), boxWidth-4),
		m.settingsRow(settingsItemServer, "🌐 "+m.tr("Server URL"), settingsDisplayValue(m.settingsDraftServer, m.tr("not set")), boxWidth-4),
		m.settingsRow(settingsItemToken, "🔑 "+m.tr("Access token"), maskedToken(m.settingsDraftToken, m.tr("not set")), boxWidth-4),
		m.settingsRow(settingsItemSave, "💾 "+m.tr("Save connection"), m.tr("save to config"), boxWidth-4),
	}
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	for _, line := range m.settingsHelpLines(boxWidth - 4) {
		b.WriteString(line)
		b.WriteString("\n")
	}

	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) settingsHelpLines(width int) []string {
	if m.settingsEditing {
		return []string{muted.Render(truncate(m.tr("editing: type, enter save, esc cancel, ctrl+u clear"), width))}
	}
	lines := make([]string, 0, 4)
	if m.setupRequired {
		lines = append(lines, accent.Render(truncate(m.tr("Enter server URL and token, save, then restart."), width)))
	}
	lines = append(lines,
		muted.Render(truncate(m.tr("What is this token?"), width)),
		muted.Render(truncate("• "+m.tr("Recommended: Mattermost Personal Access Token."), width)),
		muted.Render(truncate("• "+m.tr("Also works: browser MMAUTHTOKEN session cookie."), width)),
		muted.Render(truncate("• "+m.tr("Use mmux auth to save MMAUTHTOKEN interactively."), width)),
	)
	if !m.setupRequired {
		lines = append(lines, muted.Render(truncate(m.tr("Connection changes are used after restart."), width)))
	}
	return lines
}

func (m Model) settingsRow(index int, label, value string, width int) string {
	cursor := ""
	if m.settingsEditing && m.settingsSelected == index {
		cursor = "_"
	}
	line := fmt.Sprintf("%s: %s%s", label, value, cursor)
	if index == settingsItemLanguage {
		line += "  ‹ ›"
	}
	line = truncate(line, max(1, width-2))
	if m.settingsSelected == index {
		return pillStyle.Width(width).Render(line)
	}
	return muted.Render(truncate(line, width))
}

func settingsDisplayValue(value, empty string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return empty
	}
	return sanitizeTerminal(value)
}

func maskedToken(token, empty string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return empty
	}
	runes := []rune(token)
	if len(runes) <= 8 {
		return strings.Repeat("•", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("•", 8) + string(runes[len(runes)-4:])
}

func (m Model) renderInfo(width, height int) string {
	boxWidth := min(max(72, width*2/3), max(72, width-8))
	boxHeight := min(max(16, height*2/3), max(16, height-4))
	contentWidth := max(20, boxWidth-4)
	var b strings.Builder
	b.WriteString(headerStyle.Render("Info"))
	b.WriteString(muted.Render("  i/esc close · x technical"))
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
	if (ch.Type == "D" || ch.Type == "G") && len(ch.Users) > 0 {
		return m.renderUserInfoBody(ch, width)
	}
	return m.renderChannelInfoBody(ch, width)
}

func (m Model) renderChannelInfoBody(ch domain.Channel, width int) string {
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
	m.writeInfoFacts(&b, []infoFactLine{
		infoFact("members", fmt.Sprintf("%d", ch.MemberCount), ch.MemberCount > 0),
		infoFact("unread", fmt.Sprintf("%d", ch.Unread), ch.Unread > 0),
		infoFact("mentions", fmt.Sprintf("%d", ch.Mentions), ch.Mentions > 0),
		infoFact("last post", formatDate(ch.LastPostAt)+" "+formatTime(ch.LastPostAt), ch.LastPostAt > 0),
	})
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
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderUserInfoBody(ch domain.Channel, width int) string {
	var b strings.Builder
	if ch.Type == "G" && len(ch.Users) > 1 {
		name := ch.DisplayName
		if name == "" {
			name = ch.Name
		}
		b.WriteString(headerStyle.Render("◦ " + sanitizeTerminal(name)))
		b.WriteString("\n")
		b.WriteString(muted.Render(fmt.Sprintf("group message · %d members", len(ch.Users))))
		b.WriteString("\n\n")
		for i, user := range ch.Users {
			if i > 0 {
				b.WriteString("\n")
			}
			m.writeUserCard(&b, user, width, false)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	m.writeUserCard(&b, ch.Users[0], width, m.infoExpanded)
	b.WriteString("\n")
	m.writeInfoFacts(&b, []infoFactLine{
		infoFact("channel", "direct message", true),
		infoFact("unread", fmt.Sprintf("%d", ch.Unread), ch.Unread > 0),
		infoFact("mentions", fmt.Sprintf("%d", ch.Mentions), ch.Mentions > 0),
		infoFact("last post", formatDate(ch.LastPostAt)+" "+formatTime(ch.LastPostAt), ch.LastPostAt > 0),
	})
	return strings.TrimRight(b.String(), "\n")
}

type infoFactLine struct {
	Key   string
	Value string
	Show  bool
}

func infoFact(key, value string, show bool) infoFactLine {
	return infoFactLine{Key: key, Value: value, Show: show}
}

func (m Model) writeInfoFacts(b *strings.Builder, facts []infoFactLine) {
	wrote := false
	for _, fact := range facts {
		value := strings.TrimSpace(fact.Value)
		if !fact.Show || value == "" {
			continue
		}
		b.WriteString(muted.Render("• "))
		b.WriteString(sanitizeTerminal(fact.Key))
		b.WriteString(muted.Render(": "))
		b.WriteString(sanitizeTerminal(value))
		b.WriteString("\n")
		wrote = true
	}
	if wrote {
		b.WriteString("\n")
	}
}

func (m Model) writeUserCard(b *strings.Builder, user domain.User, width int, expanded bool) {
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	if name == "" {
		name = shortID(user.ID)
	}
	b.WriteString(headerStyle.Render(presenceGlyph(user.Status) + sanitizeTerminal(name)))
	b.WriteString("\n")
	subtitle := []string{}
	if user.Username != "" {
		subtitle = append(subtitle, "@"+user.Username)
	}
	if user.Status != "" {
		subtitle = append(subtitle, statusLabel(user.Status))
	}
	if user.Position != "" {
		subtitle = append(subtitle, user.Position)
	}
	if len(subtitle) > 0 {
		b.WriteString(muted.Render(truncate(strings.Join(subtitle, " · "), width)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Profile"))
	b.WriteString("\n")
	m.writeInfoFacts(b, userProfileFacts(user))
	if facts := userActivityFacts(user); len(facts) > 0 {
		b.WriteString(headerStyle.Render("Activity"))
		b.WriteString("\n")
		m.writeInfoFacts(b, facts)
	}
	if facts := userPrettyProps(user.Props); len(facts) > 0 {
		b.WriteString(headerStyle.Render("Status"))
		b.WriteString("\n")
		m.writeInfoFacts(b, facts)
	}
	if expanded {
		if facts := userTechnicalFacts(user); len(facts) > 0 {
			b.WriteString(headerStyle.Render("Technical"))
			b.WriteString("\n")
			m.writeInfoFacts(b, facts)
		}
		if len(user.Timezone) > 0 {
			b.WriteString(headerStyle.Render("Timezone"))
			b.WriteString("\n")
			m.writeInfoFacts(b, mapInfoFacts(user.Timezone))
		}
		if facts := mapInfoFactsExcept(user.Props, map[string]struct{}{"customStatus": {}}); len(facts) > 0 {
			b.WriteString(headerStyle.Render("Props"))
			b.WriteString("\n")
			m.writeInfoFacts(b, facts)
		}
	} else {
		b.WriteString(muted.Render("x show technical details"))
		b.WriteString("\n")
	}
}

func userProfileFacts(user domain.User) []infoFactLine {
	return []infoFactLine{
		infoFact("username", "@"+user.Username, user.Username != ""),
		infoFact("email", user.Email, user.Email != ""),
		infoFact("first name", user.FirstName, user.FirstName != ""),
		infoFact("last name", user.LastName, user.LastName != ""),
		infoFact("nickname", user.Nickname, user.Nickname != ""),
		infoFact("position", user.Position, user.Position != ""),
		infoFact("locale", user.Locale, user.Locale != ""),
		infoFact("timezone", user.Timezone["automaticTimezone"], user.Timezone["automaticTimezone"] != ""),
		infoFact("bot", user.BotDescription, user.BotDescription != ""),
	}
}

func userActivityFacts(user domain.User) []infoFactLine {
	return compactInfoFacts([]infoFactLine{
		infoFact("last active", formatDate(user.LastActivityAt)+" "+formatTime(user.LastActivityAt), user.LastActivityAt > 0),
	})
}

func userTechnicalFacts(user domain.User) []infoFactLine {
	return []infoFactLine{
		infoFact("id", user.ID, user.ID != ""),
		infoFact("roles", user.Roles, user.Roles != ""),
		infoFact("auth", user.AuthService, user.AuthService != ""),
		infoFact("mfa", fmt.Sprintf("%v", user.MFAActive), user.MFAActive),
		infoFact("created", formatDate(user.CreateAt)+" "+formatTime(user.CreateAt), user.CreateAt > 0),
		infoFact("updated", formatDate(user.UpdateAt)+" "+formatTime(user.UpdateAt), user.UpdateAt > 0),
		infoFact("picture updated", formatDate(user.LastPictureUpdate)+" "+formatTime(user.LastPictureUpdate), user.LastPictureUpdate > 0),
		infoFact("password updated", formatDate(user.LastPasswordUpdate)+" "+formatTime(user.LastPasswordUpdate), user.LastPasswordUpdate > 0),
	}
}

func compactInfoFacts(facts []infoFactLine) []infoFactLine {
	out := facts[:0]
	for _, fact := range facts {
		if fact.Show && strings.TrimSpace(fact.Value) != "" {
			out = append(out, fact)
		}
	}
	return out
}

func userPrettyProps(props map[string]string) []infoFactLine {
	custom := strings.TrimSpace(props["customStatus"])
	if custom == "" {
		return nil
	}
	var parsed struct {
		Emoji string `json:"emoji"`
		Text  string `json:"text"`
	}
	if err := json.Unmarshal([]byte(custom), &parsed); err != nil || (parsed.Emoji == "" && parsed.Text == "") {
		return []infoFactLine{infoFact("custom", custom, true)}
	}
	label := strings.TrimSpace(parsed.Text)
	if parsed.Emoji != "" {
		label = strings.TrimSpace(":" + parsed.Emoji + ": " + label)
	}
	return []infoFactLine{infoFact("custom", label, label != "")}
}

func mapInfoFacts(values map[string]string) []infoFactLine {
	return mapInfoFactsExcept(values, nil)
}

func mapInfoFactsExcept(values map[string]string, skip map[string]struct{}) []infoFactLine {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if _, ok := skip[key]; ok {
			continue
		}
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	facts := make([]infoFactLine, 0, len(keys))
	for _, key := range keys {
		facts = append(facts, infoFact(key, values[key], true))
	}
	return facts
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

func (m Model) renderReactionPicker(width, height int) string {
	post, _ := m.selectedReactionTarget()
	options := m.reactionOptionsForPost(post)
	querying := strings.TrimSpace(m.reactionPickerQuery) != ""
	cols := m.reactionPickerColumns(options)
	cellWidth := 8
	if querying {
		cellWidth = 34
	}
	gridRows := max(1, (len(options)+cols-1)/cols)
	visibleRows := min(gridRows, 8)
	boxWidth := min(max(72, cols*cellWidth+4), max(72, width-8))
	boxHeight := min(max(10, visibleRows+7), max(10, height-4))
	visibleRows = max(1, boxHeight-7)
	selectedRow := 0
	if len(options) > 0 {
		m.reactionPickerSelected = min(max(0, m.reactionPickerSelected), len(options)-1)
		selectedRow = m.reactionPickerSelected / cols
	}
	startRow := 0
	if selectedRow >= visibleRows {
		startRow = selectedRow - visibleRows + 1
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(m.tr("Reactions")))
	b.WriteString(muted.Render(m.tr("type search · arrows move · enter toggle · esc close")))
	b.WriteString("\n")
	if querying {
		b.WriteString(accent.Render(m.tr("filter") + ": "))
		b.WriteString(m.reactionPickerQuery)
	} else {
		b.WriteString(muted.Render(m.tr("filter: all available + reactions on this post")))
	}
	b.WriteString("\n\n")
	if startRow > 0 {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}
	for row := startRow; row < gridRows && row < startRow+visibleRows; row++ {
		var cells []string
		for col := 0; col < cols; col++ {
			idx := row*cols + col
			if idx >= len(options) {
				break
			}
			cell := reactionPickerCell(options[idx], cellWidth, idx == m.reactionPickerSelected, querying)
			cells = append(cells, cell)
		}
		b.WriteString(strings.Join(cells, ""))
		b.WriteString("\n")
	}
	if len(options) == 0 {
		b.WriteString(muted.Render(m.tr("No matching reactions.")))
		b.WriteString("\n")
	} else if startRow+visibleRows < gridRows {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}
	if len(options) > 0 {
		selected := options[m.reactionPickerSelected]
		b.WriteString("\n")
		b.WriteString(muted.Render(m.tr("selected") + ": "))
		b.WriteString(accent.Render(reactionDisplayName(selected.Name)))
		b.WriteString(muted.Render(" :" + selected.Name + ":"))
	}
	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func reactionPickerCell(option reactionOption, width int, selected bool, showName bool) string {
	label := reactionDisplayName(option.Name)
	if option.Glyph != "" {
		label = option.Glyph
	}
	if showName {
		label = label + " " + ":" + option.Name + ":"
	}
	label = truncate(label, max(1, width-2))
	style := muted.Width(width).Align(lipgloss.Left).PaddingRight(1)
	if !showName {
		style = style.Align(lipgloss.Center).PaddingRight(0)
	}
	if selected {
		style = pillStyle.Width(width).Align(lipgloss.Left).PaddingRight(1)
		if !showName {
			style = style.Align(lipgloss.Center).PaddingRight(0)
		}
	}
	return style.Render(label)
}

func renderReactionBadges(post domain.Post) string {
	if len(post.Reactions) == 0 {
		return ""
	}
	var parts []string
	for _, reaction := range post.Reactions {
		if reaction.Count <= 0 {
			continue
		}
		label := reactionDisplayName(reaction.Name) + " " + fmt.Sprintf("%d", reaction.Count)
		if reaction.Reacted {
			label = pillStyle.Render(label)
		} else {
			label = muted.Render(label)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "  ")
}

func (m Model) renderTriage(width, height int) string {
	boxWidth := min(max(72, width/2), max(72, width-8))
	visibleItems := min(max(1, len(m.triageItems)), 12)
	boxHeight := min(max(8, visibleItems+5), max(8, height-4))
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s %d", m.tr("Triage"), len(m.triageItems))))
	b.WriteString(muted.Render("  enter " + m.tr("open") + " · d " + m.tr("done") + " · esc " + m.tr("close")))
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
		b.WriteString(muted.Render(m.tr("Nothing to triage.")))
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
	msg := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(postPlainText(post))), "\n", " ")
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
	title := fmt.Sprintf("%s · %d %s", m.tr("Thread"), replies, m.plural(replies, "reply", "replies"))
	if m.threadLoading {
		title = "Thread · loading…"
	}
	help := "tab reply · alt+4 messages · alt+3 reply · alt+2 timeline · esc close · R react"
	if m.threadFocusComposer {
		help = "alt+4 messages · alt+2 timeline · enter reply · alt+enter newline"
	} else if m.focus == focusTimeline {
		help = "alt+4 thread · alt+3 reply · esc close thread"
	}
	rootText := "root: loading…"
	if root, ok := m.threadRootPost(); ok {
		user := root.Username
		if user == "" {
			user = shortID(root.UserID)
		}
		text := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(postPlainText(root))), "\n", " ")
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

func (m Model) plural(n int, one, many string) string {
	return m.tr(plural(n, one, many))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (m Model) replyCountLabel(count int) string {
	return fmt.Sprintf("  ↳ %d %s", count, m.plural(count, "reply", "replies"))
}

func (m Model) renderThreadComposer(width int) string {
	labelText := m.threadComposerLabel(max(10, width-4))
	placeholder := m.tr("Write a reply…")
	if !m.threadFocusComposer {
		labelText = truncate(m.tr("reply composer inactive")+" · "+m.tr("tab reply"), max(10, width-4))
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
	prefix := m.tr("reply in thread")
	if root, ok := m.threadRootPost(); ok {
		user := root.Username
		if user == "" {
			user = shortID(root.UserID)
		}
		text := strings.ReplaceAll(strings.TrimSpace(sanitizeMessageText(postPlainText(root))), "\n", " ")
		if text != "" {
			prefix = m.tr("reply to") + ": " + user + " · " + text
		} else {
			prefix = m.tr("reply to") + ": " + user
		}
	}
	return truncate(prefix+" · "+m.tr("enter send")+" · "+m.tr("alt+enter newline"), width)
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

func threadRenderedCount(posts []domain.Post, rootID string) int {
	count := 0
	for _, post := range posts {
		if post.ID == rootID || post.RootID == rootID || (rootID == "" && post.RootID == "") {
			count++
		}
	}
	return count
}

func (m Model) renderThreadPosts(width int) string {
	content, _ := m.renderThreadPostsWithOffsets(width)
	return content
}

func (m Model) renderThreadPostsWithOffsets(width int) (string, []int) {
	var b strings.Builder
	offsets := make([]int, len(m.threadPosts))
	lineNo := 0
	writeLine := func(s string) {
		b.WriteString(s)
		b.WriteString("\n")
		lineNo++
	}
	writeBlank := func() { writeLine("") }
	repliesWritten := 0
	replyCount := threadRenderedCount(m.threadPosts, m.threadRootID)
	var prevRendered domain.Post
	hasPrevRendered := false
	for idx, post := range m.threadPosts {
		offsets[idx] = lineNo
		grouped := hasPrevRendered && shouldGroupTimelinePost(prevRendered, post)
		selected := idx == m.threadSelected && !m.threadFocusComposer
		if !grouped {
			writeLine(m.renderPostLine(m.renderThreadPostHeader(post, grouped, selected), selected))
		}
		body := renderMarkdownMessage(post.Message, max(20, width-6))
		for _, line := range renderMessageBodyLines(post, body, grouped, selected) {
			writeLine(m.renderPostLine(line, selected))
		}
		for _, line := range m.renderAttachmentLines(post, max(20, width-6), grouped && body == "", selected) {
			writeLine(m.renderPostLine(line, selected))
		}
		if badges := renderReactionBadges(post); badges != "" {
			writeLine(m.renderPostLine(renderMessageBodyLine(badges, post, selected), selected))
		}
		repliesWritten++
		prevRendered = post
		hasPrevRendered = true
		if repliesWritten < replyCount && !nextThreadReplyGroups(m.threadPosts, idx+1, m.threadRootID, post) {
			writeBlank()
		}
	}
	return strings.TrimRight(b.String(), "\n"), offsets
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
		query = m.tr("type command or channel…")
	}
	b.WriteString(headerStyle.Render(m.tr("Go to")))
	b.WriteString("\n")
	b.WriteString(muted.Render("ctrl+p · enter " + m.tr("open") + " · esc " + m.tr("close")))
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
		b.WriteString(muted.Render(m.tr("No matches")))
		b.WriteString("\n")
	} else if start+limit < len(indexes) {
		b.WriteString(muted.Render("…"))
		b.WriteString("\n")
	}

	box := boxStyle.Width(boxWidth).Height(boxHeight).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) switcherCommandLine(index int) string {
	switch index {
	case switcherGoSidebar:
		return m.tr("Go: Sidebar") + "  · alt+1"
	case switcherGoTimeline:
		return m.tr("Go: Timeline") + " · alt+2"
	case switcherGoComposer:
		return m.tr("Go: Composer") + " · alt+3"
	case switcherGoThread:
		return m.tr("Go: Thread messages") + " · alt+4"
	case switcherOpenTriage:
		return m.tr("Open: Triage inbox") + " · ctrl+u"
	case switcherOpenMentions:
		return m.tr("Open: Mentions inbox") + " · a"
	case switcherOpenSettings:
		return m.tr("Open: Settings") + " · alt+,"
	default:
		return m.tr("Go: unknown")
	}
}

func (m Model) renderSwitcherLine(index, width int) string {
	if index < 0 {
		return truncate(m.switcherCommandLine(index), width)
	}
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
	section := m.tr("channels")
	if m.favoriteChannels[ch.ID] {
		section = m.tr("favorite")
	} else if ch.Type == "D" {
		section = m.tr("direct")
	} else if ch.Type == "G" {
		section = m.tr("group")
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

	headerLines := []string{statusChipBase.Render("◆ " + m.sidebarTitle())}
	if m.session != nil {
		name := m.session.User.DisplayName
		if name == "" {
			name = "@" + m.session.User.Username
		}
		headerLines = append(headerLines, muted.Render("👤 "+sanitizeTerminal(name)))
	} else {
		headerLines = append(headerLines, muted.Render("◌ "+m.tr("connecting…")))
	}
	if m.filtering || m.channelFilter != "" {
		cursor := ""
		if m.filtering {
			cursor = "_"
		}
		headerLines = append(headerLines, muted.Render("/ "+sanitizeTerminal(m.channelFilter)+cursor))
	} else {
		headerLines = append(headerLines, muted.Render("⌘ / "+m.tr("filter")+" · F2 scopes"))
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
		{ID: sectionFavorites, Title: "★ Избранное"},
		{ID: sectionChannels, Title: "# Каналы", Types: []string{"O", "P"}},
		{ID: sectionDirect, Title: "@ Личные", Types: []string{"D"}},
		{ID: sectionGroups, Title: "◦ Группы", Types: []string{"G"}},
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
		sectionLabel := fmt.Sprintf("%s %s", chevron, section.Title)
		count := muted.Render(fmt.Sprintf("%d", len(sectionLines)))
		lines = append(lines, sidebarLine{Text: muted.Render(joinLabelAndBadge(sectionLabel, count, width)), Section: section.Title, Header: true})
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
		marker = "▌ "
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
		return sidebarMentionPill.Render(fmt.Sprintf("@%d", ch.Mentions))
	}
	if ch.Unread > 0 {
		return sidebarBadgeStyle.Render(fmt.Sprintf("%d", ch.Unread))
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
	if m.focus == focusTimeline {
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
		parts = append(parts, fmt.Sprintf("%d %s", len(m.posts), m.plural(len(m.posts), "message", "messages")))
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
		labelText = truncate(m.tr("composer inactive")+" · "+m.tr("tab focus input"), max(10, width-4))
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
	target := m.tr("current channel")
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
	return truncate(m.tr("to")+" "+target+" · "+m.tr("enter send")+" · "+m.tr("alt+enter newline"), width)
}

func (m Model) renderStatus(width int) string {
	if width <= 0 {
		return ""
	}
	segments := make([]string, 0, 6)
	if scope := m.sidebarTitle(); scope != "" {
		segments = append(segments, m.statusChip("◆ "+m.tr("scope")+": "+scope, statusChipBase))
	}
	if net := m.connectionStatusText(); net != "" {
		segments = append(segments, m.statusChip(m.connectionStatusIcon()+" "+m.tr("net")+": "+net, statusChipBase))
	}
	mainStatus := m.statusDisplayText(m.status)
	if m.threadOpen {
		mainStatus = m.threadStatusLine()
	} else if mainStatus == "" {
		mainStatus = m.tr("ready")
	}
	if mainStatus != "" {
		icon := m.statusMainIcon(mainStatus)
		segments = append(segments, m.statusChip(icon+" "+mainStatus, statusChipBase))
	}
	if !m.threadOpen {
		if m.focus == focusComposer {
			segments = append(segments, m.statusChip("↓ "+m.timelinePositionLabel(), statusChipBase))
		}
		if hint := m.focusStatusHint(); hint != "" {
			segments = append(segments, m.statusChip(m.tr("keys")+": "+hint, statusKeyChip))
		}
	}
	if m.err != "" {
		segments = append(segments, errorText.Render("⚠ "+m.tr(m.err)))
	}
	if badge := m.notificationBadge(); badge != "" {
		segments = append(segments, accent.Render("@ "+badge))
	}
	status := strings.Join(segments, " ")
	return statusBarStyle.Width(width).Padding(0, 1).Render(truncate(status, width-2))
}

func (m Model) statusChip(text string, style lipgloss.Style) string {
	return style.Render(text)
}

func (m Model) statusMainIcon(status string) string {
	if m.loading {
		return "⟳"
	}
	lower := strings.ToLower(status)
	if strings.Contains(lower, "connected") || strings.Contains(lower, "sent") || strings.Contains(lower, "подключено") || strings.Contains(lower, "отправ") {
		return "✓"
	}
	if strings.Contains(lower, "failed") || strings.Contains(lower, "error") || strings.Contains(lower, "не удалось") {
		return "⚠"
	}
	if strings.Contains(lower, "message") || strings.Contains(lower, "сообщ") {
		return "💬"
	}
	return "✦"
}

func (m Model) connectionStatusIcon() string {
	switch m.connectionState {
	case domain.ConnectionConnected:
		return "●"
	case domain.ConnectionConnecting, domain.ConnectionReconnecting:
		return "◌"
	case domain.ConnectionAuthExpired:
		return "⚠"
	case domain.ConnectionOffline:
		return "○"
	default:
		return "◇"
	}
}

func (m Model) statusDisplayText(status string) string {
	if status == "" {
		return ""
	}
	if head, tail, ok := strings.Cut(status, " · "); ok {
		return m.statusDisplayText(head) + " · " + m.tr(tail)
	}
	fields := strings.Fields(status)
	if len(fields) == 2 && fields[1] == "messages" {
		if n, err := strconv.Atoi(fields[0]); err == nil {
			return fmt.Sprintf("%d %s", n, m.plural(n, "message", "messages"))
		}
	}
	return m.tr(status)
}

func (m Model) focusStatusHint() string {
	if m.isRussian() {
		switch m.focus {
		case focusSidebar:
			return "сайдбар · enter открыть · / фильтр · ctrl+k лента · ctrl+j ввод"
		case focusTimeline:
			return "лента · j/k выбрать · t тред · y копия · n непрочит. · ctrl+h сайдбар · ctrl+j ввод"
		case focusComposer:
			return "ctrl+h сайдбар · ctrl+k лента"
		default:
			return "? помощь"
		}
	}
	switch m.focus {
	case focusSidebar:
		return "sidebar · enter open · / filter · ctrl+k timeline · ctrl+j compose"
	case focusTimeline:
		return "timeline · j/k select · t thread · y copy · n unread · ctrl+h sidebar · ctrl+j compose"
	case focusComposer:
		return "ctrl+h sidebar · ctrl+k timeline"
	default:
		return "? help"
	}
}

func (m Model) threadStatusLine() string {
	count := m.tr("loading…")
	if !m.threadLoading {
		count = fmt.Sprintf("%d messages", len(m.threadPosts))
	}
	if m.isRussian() {
		if m.focus == focusTimeline {
			return "лента · " + count + " в треде · alt+3 ответ · alt+4 тред · esc закрыть тред"
		}
		if m.threadFocusComposer {
			return "ответ в тред · " + count + " · tab сообщения · alt+4 сообщения · alt+2 лента · esc закрыть/к сообщениям"
		}
		return "сообщения треда · " + count + " · tab ответ · alt+3 ответ · alt+2 лента · esc закрыть"
	}
	if m.focus == focusTimeline {
		return "timeline · " + count + " in thread · alt+3 reply · alt+4 thread · esc close thread"
	}
	if m.threadFocusComposer {
		return "thread reply · " + count + " · tab messages · alt+4 messages · alt+2 timeline · esc close/messages"
	}
	return "thread messages · " + count + " · tab reply · alt+3 reply · alt+2 timeline · esc close"
}

func (m Model) timelinePositionLabel() string {
	if m.viewport.TotalLineCount() == 0 || m.viewport.AtBottom() {
		return m.tr("at latest")
	}
	return m.tr("scrolled")
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
			return muted.Render(m.tr("Loading messages…")), nil
		}
		return muted.Render(m.tr("No messages yet.")), nil
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
		writeLine(muted.Render(m.tr("Loading older messages…")))
		writeBlank()
	} else if !m.hasOlder {
		writeLine(muted.Render(m.tr("Beginning of history")))
		writeBlank()
	} else {
		writeLine(muted.Render(m.tr("Press [ to load older messages")))
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
			writeLine(renderTimelineSeparator(m.tr(date), m.viewport.Width))
			writeBlank()
			lastDate = date
		}
		if i == importantIndex {
			writeLine(accent.Render("──── " + m.tr("new messages") + " " + strings.Repeat("─", max(0, m.viewport.Width-20))))
			writeBlank()
		}
		grouped := i > 0 && shouldGroupTimelinePost(m.posts[i-1], p)
		offsets[i] = lineNo
		selected := i == m.selectedPost && m.focus == focusTimeline
		if !grouped {
			writeLine(m.renderPostLine(m.renderTimelinePostHeader(p, grouped, selected), selected))
		}
		body := renderMarkdownMessage(p.Message, max(20, m.viewport.Width-8))
		for _, line := range renderMessageBodyLines(p, body, grouped, selected) {
			writeLine(m.renderPostLine(line, selected))
		}
		for _, line := range m.renderAttachmentLines(p, max(20, m.viewport.Width-8), grouped && body == "", selected) {
			writeLine(m.renderPostLine(line, selected))
		}
		if badges := renderReactionBadges(p); badges != "" {
			writeLine(m.renderPostLine(renderMessageBodyLine(badges, p, selected), selected))
		}
		if i < len(m.posts)-1 && !shouldGroupTimelinePost(p, m.posts[i+1]) {
			writeBlank()
		}
	}
	return strings.TrimRight(b.String(), "\n"), offsets
}

func renderTimelineSeparator(label string, width int) string {
	label = " " + label + " "
	lineWidth := max(12, width-6)
	left := max(4, (lineWidth-len([]rune(label)))/2)
	right := max(4, lineWidth-left-len([]rune(label)))
	return muted.Render(strings.Repeat("─", left) + label + strings.Repeat("─", right))
}

func (m Model) renderThreadPostHeader(post domain.Post, grouped bool, selected bool) string {
	if grouped {
		return renderMessageGutter(post, selected, false) + muted.Render(formatTime(post.CreateAt))
	}
	user := sanitizeTerminal(post.Username)
	if user == "" {
		user = sanitizeTerminal(shortID(post.UserID))
	}
	header := renderMessageGutter(post, selected, true) + accent.Render(user) + muted.Render("  "+formatTime(post.CreateAt))
	if post.ThreadUnread {
		header += accent.Render("  ● new replies")
	}
	return header
}

func (m Model) renderTimelinePostHeader(post domain.Post, grouped bool, selected bool) string {
	if grouped {
		return renderMessageGutter(post, selected, false) + muted.Render(formatTime(post.CreateAt))
	}
	user := sanitizeTerminal(post.Username)
	if user == "" {
		user = sanitizeTerminal(shortID(post.UserID))
	}
	header := renderMessageGutter(post, selected, true) + accent.Render(user) + muted.Render("  "+formatTime(post.CreateAt))
	if post.ReplyCount > 0 {
		header += accent.Render(m.replyCountLabel(post.ReplyCount))
		if post.ThreadUnread {
			header += accent.Render("  ● new replies")
		}
	}
	return header
}

func renderMessageGutter(post domain.Post, selected bool, includeImportantDot bool) string {
	important := post.Unread || post.Mentioned || post.ThreadUnread
	if selected || important {
		if includeImportantDot && important {
			return accent.Render("┃ ● ")
		}
		return accent.Render("┃ ")
	}
	return subtle.Render("▏ ")
}

func renderMessageBodyLines(post domain.Post, body string, grouped bool, selected bool) []string {
	if body == "" {
		return nil
	}
	bodyLines := strings.Split(body, "\n")
	out := make([]string, 0, len(bodyLines))
	if !grouped {
		for _, line := range bodyLines {
			out = append(out, renderMessageBodyLine(line, post, selected))
		}
		return out
	}
	timestamp := formatTime(post.CreateAt)
	indent := strings.Repeat(" ", len([]rune(timestamp))+2)
	for i, line := range bodyLines {
		if i == 0 {
			out = append(out, renderMessageGutter(post, selected, false)+subtle.Render(timestamp+"  ")+line)
			continue
		}
		out = append(out, renderMessageGutter(post, selected, false)+subtle.Render(indent)+line)
	}
	return out
}

func renderMessageBodyLine(line string, post domain.Post, selected bool) string {
	return renderMessageGutter(post, selected, false) + line
}

func (m Model) renderAttachmentLines(post domain.Post, width int, groupedNoBody bool, selected bool) []string {
	if len(post.Files) == 0 {
		return nil
	}
	out := make([]string, 0, len(post.Files))
	timestamp := formatTime(post.CreateAt)
	indent := ""
	if groupedNoBody {
		indent = timestamp + "  "
	}
	for i, file := range post.Files {
		prefix := indent
		if groupedNoBody && i > 0 {
			prefix = strings.Repeat(" ", len([]rune(timestamp))+2)
		}
		line := prefix + m.attachmentLabel(file)
		if width > 0 {
			line = truncate(line, width)
		}
		out = append(out, renderMessageGutter(post, selected, false)+line)
	}
	return out
}

func (m Model) attachmentLabel(file domain.PostFile) string {
	name := sanitizeTerminal(strings.TrimSpace(file.Name))
	if name == "" {
		name = m.tr("file")
		if file.ID != "" {
			name += " " + shortID(file.ID)
		}
	}
	parts := []string{name}
	var details []string
	if file.Size > 0 {
		details = append(details, formatFileSize(file.Size))
	}
	if file.MIMEType != "" {
		details = append(details, sanitizeTerminal(file.MIMEType))
	} else if file.Extension != "" {
		details = append(details, sanitizeTerminal(file.Extension))
	}
	if file.Width > 0 && file.Height > 0 {
		details = append(details, fmt.Sprintf("%d×%d", file.Width, file.Height))
	}
	if len(details) > 0 {
		parts = append(parts, "("+strings.Join(details, " · ")+")")
	}
	suffix := ""
	if len(parts) > 1 {
		suffix = " " + strings.Join(parts[1:], " ")
	}
	return muted.Render("📎 ") + accent.Render(parts[0]) + muted.Render(suffix)
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			if value >= 10 {
				return fmt.Sprintf("%.0f %s", value, unit)
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.0f PB", value/1024)
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
	// Selection is shown by the block gutter itself (┃) to avoid competing left markers.
	return selectedMsgStyle.Render(line)
}

func (m Model) helpText() string {
	if m.isRussian() {
		return strings.TrimSpace(`Клавиши

  ctrl+p            быстрый переход / команды
  alt+1/2/3/4       сайдбар / лента / ввод / тред
  ctrl+h/j/k/l      сайдбар / ввод / лента / тред как в tmux/vim
  ctrl+b / alt+s    перейти в левый сайдбар из любого места
  alt+,             настройки из любого места
  ,                 настройки, когда вы не печатаете
  /                 фильтр каналов
  tab / shift+tab   переключить фокус
  j/k или стрелки   навигация по сайдбару / ленте
  n / N             следующее / предыдущее непрочитанное или упоминание
  a                 упоминания (@you/@all/@channel/@here)
  ctrl+u / alt+u    triage из любого места, включая ввод
  u                 triage когда вы не печатаете
                    triage: enter открыть · d скрыть · n/N двигаться · esc закрыть
  i                 информация о канале/человеке
  F2 / ctrl+g       переключить область/team/workspace
  w / T             переключить область, когда вы не печатаете
  f                 добавить/убрать избранное в сайдбаре
  s/c/d/g или 0/1/2/3 перейти к избранным/каналам/личным/группам
  pgup/pgdown       прыгать между секциями
  left/right        свернуть/развернуть секцию
  space или x       переключить секцию
  enter             отправить сообщение или открыть канал
  alt+enter         новая строка
  [ или ctrl+o      загрузить старые сообщения
  y                 скопировать текст сообщения
  p                 скопировать permalink
  o / enter         открыть первую ссылку
  r                 процитировать сообщение
  e                 редактировать своё сообщение
  D                 удалить своё сообщение (нажать дважды)
  t                 открыть тред
  R                 поиск/выбор реакции
  ctrl+r            перезагрузить канал или переподключиться
  ?                 помощь
  q / ctrl+c        выйти

Конфиг

  MMUX_URL=https://mattermost.example.com
  MMUX_TOKEN=...
  MMUX_LANG=ru

или ~/.config/band-tui/config.json с server_url, token и language.`)
	}
	return strings.TrimSpace(`Keys

  ctrl+p            quick switcher / go to
  alt+1/2/3/4       sidebar / timeline / composer / thread
  ctrl+h/j/k/l      sidebar / composer / timeline / thread like tmux/vim
  ctrl+b / alt+s    focus left sidebar from anywhere
  alt+,             open settings from anywhere
  ,                 open settings when not typing
  /                 filter channels
  tab / shift+tab   switch focus
  j/k or arrows     move in sidebar / timeline
  n / N             next / previous unread or mention
  a                 open mentions inbox (@you/@all/@channel/@here)
  ctrl+u / alt+u  open triage inbox from anywhere (incl. typing)
  u                 open triage inbox when not typing
                    triage: mentions/unread/thread replies · enter open · d done · n/N move · esc close
  i                 open channel/person info (when not typing)
  F2 / ctrl+g       switch scope/team/workspace
  w / T             switch scope when not typing
  f                 toggle favorite in sidebar
  s/c/d/g or 0/1/2/3 jump to favorites/channels/direct/groups
  pgup/pgdown       jump between sections
  left/right        collapse/expand current section
  space or x        toggle current section
  enter             send message or open selected channel
  alt+enter         insert newline in composer
  [ or ctrl+o       load older messages
  y                 copy selected message text
  p                 copy selected message permalink
  o / enter         open first link in selected message
  r                 quote selected message into composer
  e                 edit selected own message
  D                 delete selected own message (press twice)
  t                 open thread for selected message
  R                 open reaction picker/search for selected message
  ctrl+r            reload current channel or reconnect when offline
  ?                 toggle help
  q / ctrl+c        quit

Config

  MMUX_URL=https://mattermost.example.com
  MMUX_TOKEN=...
  MMUX_LANG=ru

or ~/.config/band-tui/config.json with server_url, token and language.`)
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
