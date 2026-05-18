package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"band-tui/internal/config"
	"band-tui/internal/domain"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusPane int

const (
	focusSidebar focusPane = iota
	focusTimeline
	focusComposer
)

const (
	allScopesTeamIndex = -1

	sectionFavorites = "favorites"
	sectionChannels  = "channels"
	sectionDirect    = "direct"
	sectionGroups    = "groups"
)

type Model struct {
	backend domain.Backend
	cfg     config.Config

	ctx    context.Context
	cancel context.CancelFunc
	events chan domain.Event

	session             *domain.Session
	channels            []domain.Channel
	posts               []domain.Post
	postsByChannel      map[string][]domain.Post
	postLineOffsets     []int
	recentEvents        []domain.Post
	threadOpen          bool
	threadRootID        string
	threadPosts         []domain.Post
	threadLoading       bool
	threadViewport      viewport.Model
	threadFocusComposer bool

	selectedTeam         int
	selectedChannel      int
	selectedPost         int
	focus                focusPane
	channelFilter        string
	filtering            bool
	favoriteChannels     map[string]bool
	collapsedSections    map[string]bool
	switcherOpen         bool
	switcherQuery        string
	switcherSelected     int
	activityOpen         bool
	activitySelected     int
	triageOpen           bool
	triageSelected       int
	triageItems          []triageItem
	dismissedTriage      map[string]struct{}
	drafts               map[string]string
	activeDraftKey       string
	pendingSends         map[string]string
	infoOpen             bool
	teamSwitcherOpen     bool
	teamSwitcherSelected int
	pendingJumpChannelID string
	pendingJumpPostID    string
	pendingJumpThreadID  string

	viewport viewport.Model
	composer textarea.Model

	width  int
	height int

	status       string
	err          string
	loading      bool
	loadingOlder bool
	hasOlder     bool
	watching     bool
	showHelp     bool
	mockFallback bool
}

func New(backend domain.Backend, cfg config.Config, mockFallback bool) Model {
	ctx, cancel := context.WithCancel(context.Background())
	composer := textarea.New()
	composer.Placeholder = "Write a message…"
	composer.ShowLineNumbers = false
	composer.Prompt = ""
	composer.CharLimit = 12000
	composer.SetHeight(3)
	composer.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j", "alt+enter"), key.WithHelp("ctrl+j", "newline"))
	composer.FocusedStyle.CursorLine = lipgloss.NewStyle()
	composer.FocusedStyle.Prompt = lipgloss.NewStyle()
	composer.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorMuted)
	composer.BlurredStyle.Prompt = lipgloss.NewStyle()
	composer.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(colorMuted)
	_ = composer.Focus()

	vp := viewport.New(80, 20)
	vp.YPosition = 1
	threadVP := viewport.New(50, 10)

	favorites := make(map[string]bool, len(cfg.FavoriteChannels))
	for _, id := range cfg.FavoriteChannels {
		if id != "" {
			favorites[id] = true
		}
	}

	return Model{
		backend:           backend,
		cfg:               cfg,
		ctx:               ctx,
		cancel:            cancel,
		events:            make(chan domain.Event, 64),
		focus:             focusComposer,
		viewport:          vp,
		threadViewport:    threadVP,
		composer:          composer,
		postsByChannel:    map[string][]domain.Post{},
		selectedPost:      -1,
		favoriteChannels:  favorites,
		collapsedSections: map[string]bool{},
		dismissedTriage:   map[string]struct{}{},
		drafts:            map[string]string{},
		pendingSends:      map[string]string{},
		status:            "connecting…",
		loading:           true,
		hasOlder:          true,
		mockFallback:      mockFallback,
	}
}

func (m Model) Init() tea.Cmd {
	return connectCmd(m.ctx, m.backend)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case sessionLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "connection failed"
			return m, nil
		}
		m.session = msg.session
		m.selectedTeam = m.pickTeam()
		m.status = "connected"
		cmds := []tea.Cmd{m.loadCurrentScopeCmd()}
		if !m.watching {
			m.watching = true
			cmds = append(cmds, startWatchCmd(m.ctx, m.backend, m.events), waitEventCmd(m.events))
		}
		return m, tea.Batch(cmds...)

	case channelsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "could not load channels"
			return m, nil
		}
		m.err = ""
		m.channels = msg.channels
		m.selectedChannel = m.pickChannel()
		m.switchDraft(m.currentDraftKey())
		m.rebuildTriageItems()
		if len(m.channels) == 0 {
			m.status = "no channels"
			m.refreshViewport()
			return m, nil
		}
		m.status = "loading messages…"
		m.loading = true
		m.posts = nil
		m.refreshViewport()
		return m, loadPostsCmd(m.ctx, m.backend, m.currentChannelID())

	case postsLoadedMsg:
		if msg.channelID != m.currentChannelID() {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "could not load messages"
			return m, nil
		}
		m.err = ""
		m.hasOlder = len(msg.posts) >= 80
		m.cachePosts(msg.channelID, msg.posts)
		m.posts = append([]domain.Post(nil), msg.posts...)
		m.selectedPost = m.initialSelectedPost(msg.channelID)
		m.status = fmt.Sprintf("%d messages", len(m.posts))
		if m.selectedPost >= 0 && m.selectedPost < len(m.posts) && (m.posts[m.selectedPost].Unread || m.posts[m.selectedPost].ThreadUnread) {
			m.status += " · unread selected"
		}
		pendingThreadID := ""
		if msg.channelID == m.pendingJumpChannelID {
			pendingThreadID = m.pendingJumpThreadID
			m.pendingJumpChannelID = ""
			m.pendingJumpPostID = ""
			m.pendingJumpThreadID = ""
		}
		m.markChannelRead(msg.channelID)
		m.cachePosts(msg.channelID, m.posts)
		m.rebuildTriageItems()
		m.refreshViewport()
		if m.selectedPost >= 0 && m.selectedPost < len(m.posts)-1 {
			m.scrollSelectedPostIntoView()
		} else {
			m.viewport.GotoBottom()
		}
		cmds := []tea.Cmd{viewChannelCmd(m.ctx, m.backend, msg.channelID)}
		if pendingThreadID != "" {
			m.threadOpen = true
			m.threadRootID = pendingThreadID
			m.saveActiveDraft()
			m.loadDraft(threadDraftKey(msg.channelID, pendingThreadID))
			m.threadLoading = true
			m.threadPosts = nil
			m.threadFocusComposer = false
			m.resize()
			m.refreshThreadViewport()
			cmds = append(cmds, loadThreadCmd(m.ctx, m.backend, pendingThreadID))
		}
		return m, tea.Batch(cmds...)

	case olderPostsLoadedMsg:
		if msg.channelID != m.currentChannelID() {
			return m, nil
		}
		m.loadingOlder = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "could not load older messages"
			return m, nil
		}
		if len(msg.posts) == 0 {
			m.hasOlder = false
			m.status = "no older messages"
			return m, nil
		}
		m.err = ""
		oldSelected := m.selectedPost
		m.prependPosts(msg.channelID, msg.posts)
		m.selectedPost = oldSelected + len(msg.posts)
		m.hasOlder = len(msg.posts) >= 80
		m.status = fmt.Sprintf("%d messages", len(m.posts))
		m.refreshViewport()
		m.rebuildTriageItems()
		return m, nil

	case threadLoadedMsg:
		if msg.rootID != m.threadRootID {
			return m, nil
		}
		m.threadLoading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "could not load thread"
			m.refreshThreadViewport()
			return m, nil
		}
		m.err = ""
		m.threadPosts = msg.posts
		m.rebuildTriageItems()
		m.refreshThreadViewport()
		m.threadViewport.GotoBottom()
		return m, nil

	case replySentMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.restorePendingSend(msg.draftKey, msg.text)
			m.status = "reply failed · draft restored"
			return m, nil
		}
		m.completePendingSend(msg.draftKey)
		if msg.rootID != m.threadRootID {
			return m, nil
		}
		m.err = ""
		m.threadPosts = append(m.threadPosts, m.normalizePost(msg.post))
		m.bumpReplyCount(msg.rootID)
		m.refreshViewport()
		m.rebuildTriageItems()
		m.refreshThreadViewport()
		m.threadViewport.GotoBottom()
		m.status = "reply sent"
		return m, nil

	case postSentMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.restorePendingSend(msg.draftKey, msg.text)
			m.status = "send failed · draft restored"
			return m, nil
		}
		m.completePendingSend(msg.draftKey)
		if msg.channelID != m.currentChannelID() {
			return m, nil
		}
		m.err = ""
		m.addPost(msg.post)
		m.status = "sent"
		m.rebuildTriageItems()
		m.refreshViewport()
		m.viewport.GotoBottom()
		return m, nil

	case backendEventMsg:
		cmds := []tea.Cmd{waitEventCmd(m.events)}
		switch msg.event.Kind {
		case domain.EventPost:
			post := msg.event.Post
			mentionActivity := m.isMentionActivity(post)
			if mentionActivity {
				post.Unread = true
				m.status = m.activityStatus(post)
			}
			m.recordActivity(post)
			if post.RootID != "" {
				viewCurrent := post.ChannelID == m.currentChannelID()
				visibleThread := viewCurrent && m.threadOpen && post.RootID == m.threadRootID
				if !visibleThread {
					post.Unread = true
					post.ThreadUnread = true
				}
				m.addPostToCache(post.ChannelID, post)
				m.bumpReplyCount(post.RootID)
				if !visibleThread {
					m.markThreadUnread(post.ChannelID, post.RootID)
				}
				if visibleThread {
					m.threadPosts = append(m.threadPosts, m.normalizePost(post))
					m.refreshThreadViewport()
					m.threadViewport.GotoBottom()
				}
				if !visibleThread {
					m.bumpUnread(post.ChannelID)
					if mentionActivity {
						m.bumpMention(post.ChannelID)
					}
				} else {
					m.markChannelRead(post.ChannelID)
					cmds = append(cmds, viewChannelCmd(m.ctx, m.backend, post.ChannelID))
				}
				m.refreshViewport()
				m.rebuildTriageItems()
				return m, tea.Batch(cmds...)
			}
			viewCurrent := post.ChannelID == m.currentChannelID()
			if !viewCurrent {
				post.Unread = true
			}
			m.addPostToCache(post.ChannelID, post)
			if viewCurrent {
				wasAtEnd := m.selectedPost >= len(m.posts)-1
				m.addPost(post)
				if wasAtEnd {
					m.selectedPost = len(m.posts) - 1
				}
				m.markChannelRead(post.ChannelID)
				m.refreshViewport()
				m.viewport.GotoBottom()
				cmds = append(cmds, viewChannelCmd(m.ctx, m.backend, post.ChannelID))
			} else {
				m.bumpUnread(post.ChannelID)
				if mentionActivity {
					m.bumpMention(post.ChannelID)
				}
			}
			m.rebuildTriageItems()
		case domain.EventStatus:
			m.updateUserStatus(msg.event.UserID, msg.event.Status)
		case domain.EventError:
			if msg.event.Err != nil {
				m.err = msg.event.Err.Error()
				m.status = "reconnecting…"
			}
		}
		return m, tea.Batch(cmds...)

	case watchStartedMsg:
		return m, nil

	case preferenceSavedMsg:
		if msg.err != nil {
			m.err = "save preference: " + msg.err.Error()
		}
		return m, nil

	case channelViewedMsg:
		if msg.err != nil {
			m.err = "mark read: " + msg.err.Error()
		}
		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "action failed"
		} else {
			m.err = ""
			m.status = msg.status
		}
		return m, nil
	}

	if m.focus == focusTimeline {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	if m.focus == focusComposer {
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.infoOpen {
		return m.renderInfo(m.width, m.height)
	}
	if m.teamSwitcherOpen {
		return m.renderTeamSwitcher(m.width, m.height)
	}
	if m.triageOpen {
		return m.renderTriage(m.width, m.height)
	}
	if m.activityOpen {
		return m.renderActivity(m.width, m.height)
	}
	if m.switcherOpen {
		return m.renderSwitcher(m.width, m.height)
	}
	if m.threadOpen {
		return m.renderThreadLayout(m.width, m.height)
	}

	sidebarWidth := m.sidebarWidth()
	mainWidth := max(20, m.width-sidebarWidth-1)

	sidebar := m.renderSidebar(sidebarWidth, m.height-2)
	timeline := m.renderMain(mainWidth, m.height-2)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, timeline)
	return body + "\n" + m.renderStatus(m.width)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.infoOpen {
		return m.handleInfoKey(msg)
	}
	if m.teamSwitcherOpen {
		return m.handleTeamSwitcherKey(msg)
	}
	if m.triageOpen {
		return m.handleTriageKey(msg)
	}
	if msg.String() == "u" && m.threadOpen && !m.threadFocusComposer {
		m = m.openTriageOverlay()
		return m, nil
	}
	if m.activityOpen {
		return m.handleActivityKey(msg)
	}
	if m.threadOpen {
		return m.handleThreadKey(msg)
	}
	if m.switcherOpen {
		return m.handleSwitcherKey(msg)
	}
	if msg.String() == "ctrl+p" || msg.String() == "ctrl+k" {
		m.switcherOpen = true
		m.switcherQuery = ""
		m.switcherSelected = 0
		m.focus = focusSidebar
		m.applyFocus()
		return m, nil
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "q":
		if m.focus != focusComposer {
			m.cancel()
			_ = m.backend.Close()
			return m, tea.Quit
		}
	case "?":
		if m.focus != focusComposer {
			m.showHelp = !m.showHelp
			m.refreshViewport()
			return m, nil
		}
	case "/":
		if m.focus != focusComposer {
			m.focus = focusSidebar
			m.filtering = true
			m.applyFocus()
			m.status = "filter channels"
			return m, nil
		}
	case "tab":
		m.nextFocus()
		return m, nil
	case "shift+tab":
		m.prevFocus()
		return m, nil
	case "ctrl+r":
		if m.currentChannelID() == "" {
			return m, nil
		}
		m.status = "reloading…"
		return m, loadPostsCmd(m.ctx, m.backend, m.currentChannelID())
	case "[", "ctrl+o":
		if m.focus != focusComposer {
			return m.loadOlderPosts()
		}
	case "a":
		if m.focus != focusComposer {
			m.activityOpen = true
			m.activitySelected = 0
			return m, nil
		}
	case "i":
		if m.focus != focusComposer {
			m.infoOpen = true
			return m, nil
		}
	case "u":
		if m.focus != focusComposer {
			m = m.openTriageOverlay()
			return m, nil
		}
	case "f2", "ctrl+g":
		if m.session != nil && len(m.session.Teams) > 1 {
			m.teamSwitcherOpen = true
			m.teamSwitcherSelected = m.switcherItemForTeamIndex(m.selectedTeam)
			return m, nil
		}
	case "w", "W", "T", "alt+t", "ctrl+t", "ctrl+s":
		if m.focus != focusComposer && m.session != nil && len(m.session.Teams) > 1 {
			m.teamSwitcherOpen = true
			m.teamSwitcherSelected = m.switcherItemForTeamIndex(m.selectedTeam)
			return m, nil
		}
	case "n":
		if m.focus != focusComposer {
			return m.selectRelativeImportantPost(1)
		}
	case "N":
		if m.focus != focusComposer {
			return m.selectRelativeImportantPost(-1)
		}
	case "t":
		if m.focus != focusComposer {
			return m.openSelectedThread()
		}
	}

	if m.focus == focusSidebar {
		switch msg.String() {
		case "up", "k":
			return m.selectRelativeChannel(-1)
		case "down", "j":
			return m.selectRelativeChannel(1)
		case "pgup":
			return m.jumpRelativeSection(-1)
		case "pgdown":
			return m.jumpRelativeSection(1)
		case "home":
			return m.selectEdgeChannel(false)
		case "end":
			return m.selectEdgeChannel(true)
		case "f":
			return m.toggleFavorite()
		case "s", "0":
			return m.jumpToSection(sectionFavorites)
		case "c", "1":
			return m.jumpToSection(sectionChannels)
		case "d", "2":
			return m.jumpToSection(sectionDirect)
		case "g", "3":
			return m.jumpToSection(sectionGroups)
		case "left", "h":
			return m.setCurrentSectionCollapsed(true)
		case "right", "l":
			return m.setCurrentSectionCollapsed(false)
		case " ", "x":
			return m.toggleCurrentSection()
		case "enter":
			m.focus = focusComposer
			m.applyFocus()
			return m, nil
		}
	}

	if m.focus == focusTimeline {
		switch msg.String() {
		case "up", "k":
			return m.selectRelativePost(-1)
		case "down", "j":
			return m.selectRelativePost(1)
		case "home":
			return m.selectPost(0)
		case "end":
			return m.selectPost(len(m.posts) - 1)
		case "o", "enter":
			return m.openSelectedPostLink()
		case "y":
			return m.copySelectedPostText()
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	if m.focus == focusComposer {
		switch msg.String() {
		case "enter":
			text := strings.TrimSpace(m.composer.Value())
			if text == "" || m.currentChannelID() == "" {
				return m, nil
			}
			key := m.currentDraftKey()
			m.beginPendingSend(key, text)
			m.status = "sending…"
			m.loading = true
			return m, sendPostCmd(m.ctx, m.backend, m.currentChannelID(), key, text)
		case "ctrl+j":
			m.composer.InsertString("\n")
			return m, nil
		}
		var cmd tea.Cmd
		m.composer, cmd = m.composer.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleTeamSwitcherKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	itemCount := m.teamSwitcherItemCount()
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "f2", "ctrl+g", "w", "W", "T", "alt+t", "ctrl+t", "ctrl+s":
		m.teamSwitcherOpen = false
		return m, nil
	case "up", "k":
		if m.teamSwitcherSelected > 0 {
			m.teamSwitcherSelected--
		}
		return m, nil
	case "down", "j":
		if m.teamSwitcherSelected < itemCount-1 {
			m.teamSwitcherSelected++
		}
		return m, nil
	case "home":
		m.teamSwitcherSelected = 0
		return m, nil
	case "end":
		if itemCount > 0 {
			m.teamSwitcherSelected = itemCount - 1
		}
		return m, nil
	case "enter":
		if m.teamSwitcherSelected >= 0 && m.teamSwitcherSelected < itemCount {
			return m.switchTeam(m.teamIndexForSwitcherItem(m.teamSwitcherSelected))
		}
		return m, nil
	}
	return m, nil
}

func (m Model) switchTeam(index int) (tea.Model, tea.Cmd) {
	if m.session == nil || index < allScopesTeamIndex || index >= len(m.session.Teams) {
		return m, nil
	}
	m.teamSwitcherOpen = false
	if index == m.selectedTeam {
		return m, nil
	}
	m.selectedTeam = index
	m.selectedChannel = 0
	m.selectedPost = -1
	m.channels = nil
	m.posts = nil
	m.postLineOffsets = nil
	m.postsByChannel = map[string][]domain.Post{}
	m.channelFilter = ""
	m.filtering = false
	m.threadOpen = false
	m.threadRootID = ""
	m.threadPosts = nil
	m.activityOpen = false
	m.infoOpen = false
	m.loading = true
	m.loadingOlder = false
	m.hasOlder = true
	m.triageOpen = false
	m.triageSelected = 0
	m.triageItems = nil
	m.status = "loading scope…"
	m.refreshViewport()
	return m, m.loadCurrentScopeCmd()
}

func (m Model) teamSwitcherItemCount() int {
	if m.session == nil {
		return 0
	}
	if len(m.session.Teams) > 1 {
		return len(m.session.Teams) + 1
	}
	return len(m.session.Teams)
}

func (m Model) switcherItemForTeamIndex(teamIndex int) int {
	if m.session != nil && len(m.session.Teams) > 1 {
		if teamIndex == allScopesTeamIndex {
			return 0
		}
		return teamIndex + 1
	}
	return teamIndex
}

func (m Model) teamIndexForSwitcherItem(item int) int {
	if m.session != nil && len(m.session.Teams) > 1 {
		if item == 0 {
			return allScopesTeamIndex
		}
		return item - 1
	}
	return item
}

func (m Model) handleInfoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "i", "q":
		m.infoOpen = false
		return m, nil
	}
	return m, nil
}

func (m Model) handleActivityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.activityItems()
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "a":
		m.activityOpen = false
		return m, nil
	case "up", "k":
		if m.activitySelected > 0 {
			m.activitySelected--
		}
		return m, nil
	case "down", "j":
		if m.activitySelected < len(items)-1 {
			m.activitySelected++
		}
		return m, nil
	case "enter":
		if m.activitySelected >= 0 && m.activitySelected < len(items) {
			item := items[m.activitySelected]
			if idx := m.channelIndexByID(item.ChannelID); idx >= 0 {
				m.selectedChannel = idx
				m.activityOpen = false
				m.pendingJumpChannelID = item.ChannelID
				if item.HasPost {
					if item.Post.RootID != "" {
						m.pendingJumpPostID = item.Post.RootID
						m.pendingJumpThreadID = item.Post.RootID
					} else {
						m.pendingJumpPostID = item.Post.ID
					}
				}
				return m.openCurrentChannel()
			}
		}
		return m, nil
	case "c":
		m.recentEvents = nil
		m.activitySelected = 0
		m.status = "mention activity cleared"
		return m, nil
	}
	return m, nil
}

func (m Model) handleTriageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "u":
		m.triageOpen = false
		return m, nil
	case "up", "k", "N":
		if m.triageSelected > 0 {
			m.triageSelected--
		}
		return m, nil
	case "down", "j", "n":
		if m.triageSelected < len(m.triageItems)-1 {
			m.triageSelected++
		}
		return m, nil
	case "home":
		m.triageSelected = 0
		return m, nil
	case "end":
		if len(m.triageItems) > 0 {
			m.triageSelected = len(m.triageItems) - 1
		}
		return m, nil
	case "enter":
		if m.triageSelected >= 0 && m.triageSelected < len(m.triageItems) {
			return m.openTriageItem(m.triageItems[m.triageSelected])
		}
		return m, nil
	case "d":
		if m.dismissCurrentTriageItem() {
			m.status = "triage item dismissed"
		} else {
			m.status = "nothing to dismiss"
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleThreadKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc":
		m.saveActiveDraft()
		m.threadOpen = false
		m.threadRootID = ""
		m.threadPosts = nil
		m.loadDraft(channelDraftKey(m.currentChannelID()))
		m.resize()
		m.refreshViewport()
		m.scrollSelectedPostIntoView()
		return m, nil
	case "tab", "shift+tab":
		m.threadFocusComposer = !m.threadFocusComposer
		return m, nil
	}

	if !m.threadFocusComposer {
		switch msg.String() {
		case "up", "k":
			m.threadViewport.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.threadViewport.ScrollDown(1)
			return m, nil
		case "pgup":
			m.threadViewport.PageUp()
			return m, nil
		case "pgdown":
			m.threadViewport.PageDown()
			return m, nil
		case "home":
			m.threadViewport.GotoTop()
			return m, nil
		case "end":
			m.threadViewport.GotoBottom()
			return m, nil
		}
		var cmd tea.Cmd
		m.threadViewport, cmd = m.threadViewport.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.composer.Value())
		if text == "" || m.threadRootID == "" || m.currentChannelID() == "" {
			return m, nil
		}
		key := m.currentDraftKey()
		m.beginPendingSend(key, text)
		m.status = "sending reply…"
		return m, sendReplyCmd(m.ctx, m.backend, m.currentChannelID(), m.threadRootID, key, text)
	case "ctrl+j":
		m.composer.InsertString("\n")
		return m, nil
	}
	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	return m, cmd
}

func (m Model) handleSwitcherKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc":
		m.switcherOpen = false
		m.switcherQuery = ""
		return m, nil
	case "enter":
		indexes := m.switcherIndexes()
		if len(indexes) == 0 {
			return m, nil
		}
		if m.switcherSelected < 0 {
			m.switcherSelected = 0
		}
		if m.switcherSelected >= len(indexes) {
			m.switcherSelected = len(indexes) - 1
		}
		idx := indexes[m.switcherSelected]
		m.switcherOpen = false
		m.switcherQuery = ""
		m.selectedChannel = idx
		m.focus = focusComposer
		m.applyFocus()
		return m.openCurrentChannel()
	case "backspace", "ctrl+h":
		if m.switcherQuery != "" {
			r := []rune(m.switcherQuery)
			m.switcherQuery = string(r[:len(r)-1])
			m.switcherSelected = 0
		}
		return m, nil
	case "ctrl+u":
		m.switcherQuery = ""
		m.switcherSelected = 0
		return m, nil
	case "up", "ctrl+p":
		if m.switcherSelected > 0 {
			m.switcherSelected--
		}
		return m, nil
	case "down", "ctrl+n":
		if m.switcherSelected < len(m.switcherIndexes())-1 {
			m.switcherSelected++
		}
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.switcherQuery += string(msg.Runes)
		m.switcherSelected = 0
		return m, nil
	}
	return m, nil
}

func (m Model) switcherIndexes() []int {
	query := strings.ToLower(strings.TrimSpace(m.switcherQuery))
	base := m.matchingChannelIndexesFor(query)
	sections := []string{sectionFavorites, sectionChannels, sectionDirect, sectionGroups}
	indexes := make([]int, 0, len(base))
	for _, section := range sections {
		for _, idx := range base {
			if m.channelInSection(idx, section) {
				indexes = append(indexes, idx)
			}
		}
	}
	return indexes
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc":
		if m.channelFilter == "" {
			m.filtering = false
			m.status = "filter closed"
		} else {
			m.channelFilter = ""
			m.status = "filter cleared"
		}
		return m, nil
	case "enter":
		m.filtering = false
		return m.openCurrentChannel()
	case "backspace", "ctrl+h":
		if m.channelFilter != "" {
			r := []rune(m.channelFilter)
			m.channelFilter = string(r[:len(r)-1])
			if m.ensureSelectedVisible() {
				return m.openCurrentChannel()
			}
		}
		return m, nil
	case "ctrl+u":
		m.channelFilter = ""
		return m, nil
	case "up", "ctrl+p":
		return m.selectRelativeChannel(-1)
	case "down", "ctrl+n":
		return m.selectRelativeChannel(1)
	}
	if len(msg.Runes) > 0 {
		m.channelFilter += string(msg.Runes)
		if m.ensureSelectedVisible() {
			return m.openCurrentChannel()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) selectRelativeChannel(delta int) (tea.Model, tea.Cmd) {
	indexes := m.filteredChannelIndexes()
	if len(indexes) == 0 {
		return m, nil
	}
	pos := -1
	for i, idx := range indexes {
		if idx == m.selectedChannel {
			pos = i
			break
		}
	}
	if pos == -1 {
		return m.selectChannel(indexes[0])
	}
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if pos >= len(indexes) {
		pos = len(indexes) - 1
	}
	return m.selectChannel(indexes[pos])
}

func (m Model) selectEdgeChannel(last bool) (tea.Model, tea.Cmd) {
	indexes := m.filteredChannelIndexes()
	if len(indexes) == 0 {
		return m, nil
	}
	idx := indexes[0]
	if last {
		idx = indexes[len(indexes)-1]
	}
	return m.selectChannel(idx)
}

func (m Model) selectChannel(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.channels) || index == m.selectedChannel {
		return m, nil
	}
	m.saveActiveDraft()
	m.selectedChannel = index
	m.loadDraft(m.currentDraftKey())
	return m.openCurrentChannel()
}

func (m Model) jumpToSection(section string) (tea.Model, tea.Cmd) {
	m.setSectionCollapsed(section, false)
	for _, idx := range m.matchingChannelIndexes() {
		if m.channelInSection(idx, section) {
			return m.selectChannel(idx)
		}
	}
	m.status = "no channels in section"
	return m, nil
}

func (m Model) jumpRelativeSection(delta int) (tea.Model, tea.Cmd) {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return m, nil
	}
	order := []string{sectionFavorites, sectionChannels, sectionDirect, sectionGroups}
	current := m.primarySectionForChannel(m.selectedChannel)
	pos := 0
	for i, section := range order {
		if section == current {
			pos = i
			break
		}
	}
	for step := 1; step <= len(order); step++ {
		next := pos + delta*step
		if next < 0 || next >= len(order) {
			break
		}
		for _, idx := range m.matchingChannelIndexes() {
			if m.channelInSection(idx, order[next]) {
				m.setSectionCollapsed(order[next], false)
				return m.selectChannel(idx)
			}
		}
	}
	return m, nil
}

func (m Model) toggleCurrentSection() (tea.Model, tea.Cmd) {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return m, nil
	}
	section := m.primarySectionForChannel(m.selectedChannel)
	return m.setSectionCollapsedAndRepair(section, !m.isSectionCollapsed(section))
}

func (m Model) setCurrentSectionCollapsed(collapsed bool) (tea.Model, tea.Cmd) {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return m, nil
	}
	section := m.primarySectionForChannel(m.selectedChannel)
	return m.setSectionCollapsedAndRepair(section, collapsed)
}

func (m Model) setSectionCollapsedAndRepair(section string, collapsed bool) (tea.Model, tea.Cmd) {
	if m.isSectionCollapsed(section) == collapsed {
		return m, nil
	}
	m.setSectionCollapsed(section, collapsed)
	m.status = "section expanded"
	if collapsed {
		m.status = "section collapsed"
		if m.channelInSection(m.selectedChannel, section) {
			indexes := m.filteredChannelIndexes()
			if len(indexes) == 0 {
				m.setSectionCollapsed(section, false)
				m.status = "cannot collapse last visible section"
				return m, nil
			}
			m.selectedChannel = indexes[0]
			return m.openCurrentChannel()
		}
	}
	return m, nil
}

func (m Model) loadOlderPosts() (tea.Model, tea.Cmd) {
	if m.loadingOlder || m.currentChannelID() == "" || len(m.posts) == 0 {
		return m, nil
	}
	if !m.hasOlder {
		m.status = "no older messages"
		return m, nil
	}
	m.loadingOlder = true
	m.status = "loading older messages…"
	return m, loadOlderPostsCmd(m.ctx, m.backend, m.currentChannelID(), m.posts[0].ID)
}

func (m Model) toggleFavorite() (tea.Model, tea.Cmd) {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return m, nil
	}
	id := m.channels[m.selectedChannel].ID
	if m.favoriteChannels == nil {
		m.favoriteChannels = map[string]bool{}
	}
	if m.favoriteChannels[id] {
		delete(m.favoriteChannels, id)
		m.status = "removed from favorites"
	} else {
		m.favoriteChannels[id] = true
		m.status = "added to favorites"
	}
	return m, saveFavoritesCmd(m.cfg.Config, m.favoriteChannelIDs())
}

func (m Model) favoriteChannelIDs() []string {
	ids := make([]string, 0, len(m.favoriteChannels))
	for _, ch := range m.channels {
		if m.favoriteChannels[ch.ID] {
			ids = append(ids, ch.ID)
		}
	}
	return ids
}

func (m Model) openCurrentChannel() (tea.Model, tea.Cmd) {
	channelID := m.currentChannelID()
	if channelID == "" {
		return m, nil
	}
	m.switchDraft(channelDraftKey(channelID))
	m.clearUnread(channelID)
	m.rebuildTriageItems()
	m.hasOlder = true
	m.loadingOlder = false
	if m.showCachedPosts(channelID) {
		m.status = "refreshing…"
	} else {
		m.status = "loading messages…"
		m.loading = true
		m.posts = nil
		m.refreshViewport()
	}
	cmds := []tea.Cmd{loadPostsCmd(m.ctx, m.backend, channelID)}
	if !m.mockFallback && !m.cfg.Mock {
		cmds = append(cmds, savePreferenceCmd(m.cfg.Config, m.cfg.ServerURL, m.currentTeamConfigName(), m.currentChannelConfigName()))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) ensureSelectedVisible() bool {
	indexes := m.filteredChannelIndexes()
	if len(indexes) == 0 {
		return false
	}
	for _, idx := range indexes {
		if idx == m.selectedChannel {
			return false
		}
	}
	m.selectedChannel = indexes[0]
	return true
}

func (m Model) filteredChannelIndexes() []int {
	// Return channels in the same order as the sidebar renders them. The backend
	// gives us a single alphabetically sorted list where channel and DM types are
	// interleaved; if navigation used that raw order, pressing Down would appear
	// to jump between sections. Keep keyboard navigation and visual order aligned.
	filtering := strings.TrimSpace(m.channelFilter) != ""
	matches := m.matchingChannelIndexes()
	sections := []string{sectionFavorites, sectionChannels, sectionDirect, sectionGroups}
	indexes := make([]int, 0, len(matches))
	for _, section := range sections {
		if !filtering && m.isSectionCollapsed(section) {
			continue
		}
		for _, idx := range matches {
			if m.channelInSection(idx, section) {
				indexes = append(indexes, idx)
			}
		}
	}
	return indexes
}

func (m Model) matchingChannelIndexes() []int {
	return m.matchingChannelIndexesFor(strings.ToLower(strings.TrimSpace(m.channelFilter)))
}

func (m Model) matchingChannelIndexesFor(filter string) []int {
	filter = strings.ToLower(strings.TrimSpace(filter))
	indexes := make([]int, 0, len(m.channels))
	for i, ch := range m.channels {
		if filter == "" || channelMatches(ch, filter) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func channelMatches(ch domain.Channel, filter string) bool {
	name := strings.ToLower(ch.DisplayName)
	raw := strings.ToLower(ch.Name)
	return strings.Contains(name, filter) || strings.Contains(raw, filter) || fuzzyMatch(name, filter) || fuzzyMatch(raw, filter)
}

func fuzzyMatch(s, query string) bool {
	q := []rune(query)
	if len(q) == 0 {
		return true
	}
	j := 0
	for _, r := range s {
		if r == q[j] {
			j++
			if j == len(q) {
				return true
			}
		}
	}
	return false
}

func (m Model) channelInSection(index int, section string) bool {
	if index < 0 || index >= len(m.channels) {
		return false
	}
	if section == sectionFavorites {
		return m.favoriteChannels[m.channels[index].ID]
	}
	if m.favoriteChannels[m.channels[index].ID] {
		return false
	}
	return channelSectionID(m.channels[index].Type) == section
}

func (m Model) primarySectionForChannel(index int) string {
	if index >= 0 && index < len(m.channels) && m.favoriteChannels[m.channels[index].ID] {
		return sectionFavorites
	}
	if index >= 0 && index < len(m.channels) {
		return channelSectionID(m.channels[index].Type)
	}
	return sectionChannels
}

func (m Model) isSectionCollapsed(section string) bool {
	return m.collapsedSections != nil && m.collapsedSections[section]
}

func (m *Model) setSectionCollapsed(section string, collapsed bool) {
	if m.collapsedSections == nil {
		m.collapsedSections = map[string]bool{}
	}
	if collapsed {
		m.collapsedSections[section] = true
	} else {
		delete(m.collapsedSections, section)
	}
}

func channelSectionID(channelType string) string {
	switch channelType {
	case "D":
		return sectionDirect
	case "G":
		return sectionGroups
	default:
		return sectionChannels
	}
}

func (m *Model) resize() {
	sidebarWidth := m.sidebarWidth()
	mainWidth := max(20, m.width-sidebarWidth-1)
	if m.threadOpen && m.width >= 120 {
		threadWidth := min(max(46, m.width/3), 72)
		mainWidth = max(30, m.width-sidebarWidth-threadWidth-2)
	}
	composerHeight := 5
	statusHeight := 1
	headerHeight := 2
	viewportHeight := max(3, m.height-statusHeight-headerHeight-composerHeight)

	m.viewport.Width = mainWidth - 4
	m.viewport.Height = viewportHeight
	m.composer.SetWidth(mainWidth - 4)
	m.composer.SetHeight(3)
	if m.threadOpen {
		threadWidth := min(max(46, m.width/3), 72)
		if m.width < 120 {
			threadWidth = min(max(70, m.width*3/4), max(70, m.width-6))
		}
		m.threadViewport.Width = max(20, threadWidth-4)
		m.threadViewport.Height = max(3, m.height-12)
	}
}

func (m *Model) refreshThreadViewport() {
	var content string
	switch {
	case m.threadLoading:
		content = muted.Render("Loading thread…")
	case !m.threadHasReplies():
		content = muted.Render("No replies yet.")
	default:
		content = m.renderThreadPosts(max(20, m.threadViewport.Width))
	}
	m.threadViewport.SetContent(content)
}

func (m *Model) refreshViewport() {
	if m.showHelp {
		m.postLineOffsets = nil
		m.viewport.SetContent(m.helpText())
		return
	}
	content, offsets := m.renderPosts()
	m.postLineOffsets = offsets
	m.viewport.SetContent(content)
}

func (m *Model) nextFocus() {
	m.focus = (m.focus + 1) % 3
	m.applyFocus()
}

func (m *Model) prevFocus() {
	m.focus = (m.focus + 2) % 3
	m.applyFocus()
}

func (m *Model) applyFocus() {
	// Keep the textarea focused even when the logical app focus is elsewhere.
	// Blur/Focus changes cursor rendering and line metrics in bubbles/textarea,
	// which caused visible layout jitter when tabbing back to the composer.
	_ = m.composer.Focus()
}

func (m Model) currentTeamID() string {
	if m.session == nil || len(m.session.Teams) == 0 || m.selectedTeam < 0 || m.selectedTeam >= len(m.session.Teams) {
		return ""
	}
	return m.session.Teams[m.selectedTeam].ID
}

func (m Model) loadCurrentScopeCmd() tea.Cmd {
	if m.session != nil && m.selectedTeam == allScopesTeamIndex {
		return loadAllChannelsCmd(m.ctx, m.backend, m.session.Teams)
	}
	return loadChannelsCmd(m.ctx, m.backend, m.currentTeamID())
}

func (m Model) currentChannelID() string {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return ""
	}
	return m.channels[m.selectedChannel].ID
}

func (m Model) currentChannelName() string {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return "no channel"
	}
	return m.channels[m.selectedChannel].DisplayName
}

func (m Model) currentTeamConfigName() string {
	if m.selectedTeam == allScopesTeamIndex {
		return "all"
	}
	if m.session == nil || len(m.session.Teams) == 0 || m.selectedTeam < 0 || m.selectedTeam >= len(m.session.Teams) {
		return ""
	}
	if m.session.Teams[m.selectedTeam].Name != "" {
		return m.session.Teams[m.selectedTeam].Name
	}
	return m.session.Teams[m.selectedTeam].ID
}

func (m Model) currentChannelConfigName() string {
	if len(m.channels) == 0 || m.selectedChannel < 0 || m.selectedChannel >= len(m.channels) {
		return ""
	}
	if m.channels[m.selectedChannel].Name != "" {
		return m.channels[m.selectedChannel].Name
	}
	return m.channels[m.selectedChannel].ID
}

func (m Model) pickTeam() int {
	if m.session == nil || len(m.session.Teams) == 0 {
		return 0
	}
	want := strings.ToLower(strings.TrimSpace(m.cfg.Team))
	if want == "all" || want == "all scopes" || want == "*" {
		return allScopesTeamIndex
	}
	for i, t := range m.session.Teams {
		if want != "" && (strings.ToLower(t.ID) == want || strings.ToLower(t.Name) == want || strings.ToLower(t.DisplayName) == want) {
			return i
		}
	}
	if want == "" && len(m.session.Teams) > 1 {
		return allScopesTeamIndex
	}
	return 0
}

func (m Model) pickChannel() int {
	if len(m.channels) == 0 {
		return 0
	}
	want := strings.ToLower(m.cfg.Channel)
	for i, ch := range m.channels {
		if want != "" && (strings.ToLower(ch.ID) == want || strings.ToLower(ch.Name) == want || strings.ToLower(ch.DisplayName) == want) {
			return i
		}
	}
	return 0
}

func (m *Model) cachePosts(channelID string, posts []domain.Post) {
	if m.postsByChannel == nil {
		m.postsByChannel = map[string][]domain.Post{}
	}
	m.postsByChannel[channelID] = append([]domain.Post(nil), posts...)
}

func (m *Model) prependPosts(channelID string, posts []domain.Post) {
	seen := make(map[string]struct{}, len(m.posts)+len(posts))
	merged := make([]domain.Post, 0, len(m.posts)+len(posts))
	for _, post := range posts {
		post = m.normalizePost(post)
		if post.ID != "" {
			if _, ok := seen[post.ID]; ok {
				continue
			}
			seen[post.ID] = struct{}{}
		}
		merged = append(merged, post)
	}
	for _, post := range m.posts {
		if post.ID != "" {
			if _, ok := seen[post.ID]; ok {
				continue
			}
			seen[post.ID] = struct{}{}
		}
		merged = append(merged, post)
	}
	m.posts = merged
	m.cachePosts(channelID, merged)
}

func (m Model) initialSelectedPost(channelID string) int {
	if len(m.posts) == 0 {
		return -1
	}
	if channelID == m.pendingJumpChannelID && m.pendingJumpPostID != "" {
		for i, post := range m.posts {
			if post.ID == m.pendingJumpPostID || post.RootID == m.pendingJumpPostID {
				return i
			}
		}
	}
	for i, post := range m.posts {
		if post.Mentioned {
			return i
		}
	}
	for i, post := range m.posts {
		if post.Unread {
			return i
		}
	}
	for i, post := range m.posts {
		if post.ThreadUnread {
			return i
		}
	}
	return len(m.posts) - 1
}

func isImportantPost(post domain.Post) bool {
	return post.Mentioned || post.Unread || post.ThreadUnread
}

func (m *Model) showCachedPosts(channelID string) bool {
	posts, ok := m.postsByChannel[channelID]
	if !ok {
		m.posts = nil
		m.refreshViewport()
		return false
	}
	m.posts = append([]domain.Post(nil), posts...)
	m.refreshViewport()
	m.viewport.GotoBottom()
	return true
}

func (m *Model) addPostToCache(channelID string, post domain.Post) {
	post = m.normalizePost(post)
	if m.postsByChannel == nil {
		m.postsByChannel = map[string][]domain.Post{}
	}
	for _, existing := range m.postsByChannel[channelID] {
		if existing.ID != "" && existing.ID == post.ID {
			return
		}
	}
	m.postsByChannel[channelID] = append(m.postsByChannel[channelID], post)
}

func (m *Model) addPost(post domain.Post) {
	post = m.normalizePost(post)
	for _, existing := range m.posts {
		if existing.ID != "" && existing.ID == post.ID {
			return
		}
	}
	m.posts = append(m.posts, post)
	m.addPostToCache(post.ChannelID, post)
}

func (m *Model) normalizePost(post domain.Post) domain.Post {
	if post.Username == "" {
		if m.session != nil && post.UserID == m.session.User.ID {
			post.Username = m.session.User.DisplayName
			if post.Username == "" {
				post.Username = m.session.User.Username
			}
		} else if post.UserID != "" {
			post.Username = shortID(post.UserID)
		} else {
			post.Username = "unknown"
		}
	}
	return post
}

type activityItem struct {
	ChannelID string
	Post      domain.Post
	HasPost   bool
}

func (m Model) activityItems() []activityItem {
	items := make([]activityItem, 0, len(m.recentEvents)+len(m.channels))
	seenMention := map[string]struct{}{}
	for _, post := range m.recentEvents {
		items = append(items, activityItem{ChannelID: post.ChannelID, Post: post, HasPost: true})
		seenMention[post.ChannelID] = struct{}{}
	}
	for _, ch := range m.channels {
		if ch.Mentions <= 0 {
			continue
		}
		if _, ok := seenMention[ch.ID]; ok {
			continue
		}
		items = append(items, activityItem{ChannelID: ch.ID})
	}
	return items
}

func (m Model) isOwnPost(post domain.Post) bool {
	return m.session != nil && post.UserID != "" && post.UserID == m.session.User.ID
}

func (m Model) activityStatus(post domain.Post) string {
	kind := "new message"
	if post.RootID != "" {
		kind = "new reply"
	}
	user := post.Username
	if user == "" {
		user = shortID(post.UserID)
	}
	return fmt.Sprintf("%s: %s · %s", kind, m.channelLabel(post.ChannelID), user)
}

func (m Model) channelLabel(channelID string) string {
	idx := m.channelIndexByID(channelID)
	if idx < 0 {
		return "unknown"
	}
	name := m.channels[idx].DisplayName
	if name == "" {
		name = m.channels[idx].Name
	}
	if m.channels[idx].Type == "D" {
		return "@" + name
	}
	return "#" + name
}

func (m Model) isMentionActivity(post domain.Post) bool {
	if m.isOwnPost(post) {
		return false
	}
	if post.Mentioned {
		return true
	}
	text := strings.ToLower(post.Message)
	for _, token := range []string{"@all", "@channel", "@here"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	if m.session != nil && m.session.User.Username != "" {
		return strings.Contains(text, "@"+strings.ToLower(m.session.User.Username))
	}
	return false
}

func (m *Model) recordActivity(post domain.Post) {
	if !m.isMentionActivity(post) {
		return
	}
	post = m.normalizePost(post)
	m.recentEvents = append([]domain.Post{post}, m.recentEvents...)
	if len(m.recentEvents) > 50 {
		m.recentEvents = m.recentEvents[:50]
	}
}

func (m Model) channelIndexByID(channelID string) int {
	for i := range m.channels {
		if m.channels[i].ID == channelID {
			return i
		}
	}
	return -1
}

func (m *Model) markChannelRead(channelID string) {
	for i := range m.channels {
		if m.channels[i].ID == channelID {
			m.channels[i].Unread = 0
			m.channels[i].Mentions = 0
			break
		}
	}
	m.clearPostReadFlags(channelID)
}

func (m *Model) clearPostReadFlags(channelID string) {
	for i := range m.posts {
		if m.posts[i].ChannelID == channelID {
			m.posts[i].Unread = false
			m.posts[i].Mentioned = false
			m.posts[i].ThreadUnread = false
		}
	}
	if m.postsByChannel == nil {
		return
	}
	posts := m.postsByChannel[channelID]
	for i := range posts {
		posts[i].Unread = false
		posts[i].Mentioned = false
		posts[i].ThreadUnread = false
	}
	m.postsByChannel[channelID] = posts
}

func (m *Model) clearThreadReadSignal(channelID, rootID string) {
	if channelID == "" || rootID == "" {
		return
	}
	unreadCleared := 0
	mentionsCleared := 0
	threadSignalCleared := false
	countFromCurrentPosts := true
	if m.postsByChannel != nil {
		if posts, ok := m.postsByChannel[channelID]; ok {
			countFromCurrentPosts = false
			for i := range posts {
				if postThreadRootID(posts[i]) != rootID {
					continue
				}
				if posts[i].Unread {
					unreadCleared++
				}
				if posts[i].Mentioned {
					mentionsCleared++
				}
				if posts[i].ThreadUnread {
					threadSignalCleared = true
				}
				posts[i].Unread = false
				posts[i].Mentioned = false
				posts[i].ThreadUnread = false
			}
			m.postsByChannel[channelID] = posts
		}
	}
	for i := range m.posts {
		if m.posts[i].ChannelID != channelID || postThreadRootID(m.posts[i]) != rootID {
			continue
		}
		if countFromCurrentPosts {
			if m.posts[i].Unread {
				unreadCleared++
			}
			if m.posts[i].Mentioned {
				mentionsCleared++
			}
			if m.posts[i].ThreadUnread {
				threadSignalCleared = true
			}
		}
		m.posts[i].Unread = false
		m.posts[i].Mentioned = false
		m.posts[i].ThreadUnread = false
	}
	if unreadCleared == 0 && threadSignalCleared {
		unreadCleared = 1
	}
	for i := range m.channels {
		if m.channels[i].ID != channelID {
			continue
		}
		m.channels[i].Unread = max(0, m.channels[i].Unread-unreadCleared)
		m.channels[i].Mentions = max(0, m.channels[i].Mentions-mentionsCleared)
		return
	}
}

func postThreadRootID(post domain.Post) string {
	if post.RootID != "" {
		return post.RootID
	}
	return post.ID
}

func (m *Model) markThreadUnread(channelID, rootID string) {
	for i := range m.posts {
		if m.posts[i].ID == rootID {
			m.posts[i].ThreadUnread = true
			break
		}
	}
	if m.postsByChannel != nil {
		channelPosts := m.postsByChannel[channelID]
		for i := range channelPosts {
			if channelPosts[i].ID == rootID {
				channelPosts[i].ThreadUnread = true
				break
			}
		}
		m.postsByChannel[channelID] = channelPosts
	}
}

func (m *Model) bumpReplyCount(rootID string) {
	for i := range m.posts {
		if m.posts[i].ID == rootID {
			m.posts[i].ReplyCount++
			if m.postsByChannel != nil {
				channelPosts := m.postsByChannel[m.posts[i].ChannelID]
				for j := range channelPosts {
					if channelPosts[j].ID == rootID {
						channelPosts[j].ReplyCount = m.posts[i].ReplyCount
						break
					}
				}
				m.postsByChannel[m.posts[i].ChannelID] = channelPosts
			}
			return
		}
	}
}

func (m *Model) updateUserStatus(userID, status string) {
	if userID == "" || status == "" {
		return
	}
	for i := range m.channels {
		for _, id := range m.channels[i].UserIDs {
			if id == userID {
				m.channels[i].Status = combinedChannelStatus(m.channels[i].Status, status)
				break
			}
		}
	}
}

func combinedChannelStatus(current, next string) string {
	if next == "online" || current == "" {
		return next
	}
	if current == "online" {
		return current
	}
	if next == "away" || current == "offline" {
		return next
	}
	return current
}

func (m *Model) bumpUnread(channelID string) {
	for i := range m.channels {
		if m.channels[i].ID == channelID {
			m.channels[i].Unread++
			return
		}
	}
}

func (m *Model) bumpMention(channelID string) {
	for i := range m.channels {
		if m.channels[i].ID == channelID {
			m.channels[i].Mentions++
			return
		}
	}
}

func (m *Model) clearUnread(channelID string) {
	m.markChannelRead(channelID)
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func formatTime(ms int64) string {
	if ms <= 0 {
		return "--:--"
	}
	return time.UnixMilli(ms).Format("15:04")
}

func formatDate(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	today := time.Now()
	yesterday := today.AddDate(0, 0, -1)
	switch {
	case sameDay(t, today):
		return "today"
	case sameDay(t, yesterday):
		return "yesterday"
	default:
		return t.Format("02 Jan 2006")
	}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
