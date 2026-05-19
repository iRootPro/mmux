package app

import (
	"strings"
	"testing"
	"time"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	"band-tui/internal/mock"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestQuestionMarkGoesIntoComposer(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.focus = focusComposer
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Alt: false})
	got := updated.(Model)
	if got.showHelp {
		t.Fatal("? in composer should not toggle help")
	}
	if got.composer.Value() != "?" {
		t.Fatalf("composer value = %q, want ?", got.composer.Value())
	}
}

func TestQuestionMarkTogglesHelpOutsideComposer(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.focus = focusTimeline
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Alt: false})
	got := updated.(Model)
	if !got.showHelp {
		t.Fatal("? outside composer should toggle help")
	}
}

func TestComposerHeightStableWhenTypingFirstCharacter(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.width = 100
	m.height = 30
	m.resize()
	before := lipgloss.Height(m.renderComposer(70))
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: false})
	afterModel := updated.(Model)
	after := lipgloss.Height(afterModel.renderComposer(70))
	if before != after {
		t.Fatalf("composer height changed after typing: before=%d after=%d", before, after)
	}
}

func TestRenderComposerShowsDestinationWithoutChangingHeight(t *testing.T) {
	baseline := New(nil, config.Config{}, true)
	baseline.focus = focusComposer
	wantHeight := lipgloss.Height(baseline.renderComposer(80))

	m := New(nil, config.Config{}, true)
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	got := m.renderComposer(80)

	if !strings.Contains(got, "to # Town") || !strings.Contains(got, "enter send") || !strings.Contains(got, "ctrl+j newline") {
		t.Fatalf("composer label missing destination/help: %q", got)
	}
	if strings.Contains(got, "tab nav") || strings.Contains(got, "message # Town") {
		t.Fatalf("composer label should stay short without nav/status wording: %q", got)
	}
	if h := lipgloss.Height(got); h != wantHeight {
		t.Fatalf("composer height = %d, want stable height %d", h, wantHeight)
	}
}

func TestRenderComposerShowsInactiveStateOutsideComposerFocus(t *testing.T) {
	m := New(nil, config.Config{}, true)
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town"}}
	m.selectedChannel = 0
	m.focus = focusSidebar

	got := m.renderComposer(80)

	if !strings.Contains(got, "composer inactive") || !strings.Contains(got, "tab focus input") {
		t.Fatalf("inactive composer label missing: %q", got)
	}
	if strings.Contains(got, "to # Town") || strings.Contains(got, "enter send") {
		t.Fatalf("inactive composer should not advertise active send controls: %q", got)
	}
}

func TestTabCycleKeepsFocusHintsInSync(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusSidebar || !strings.Contains(m.renderStatus(120), "sidebar") || !strings.Contains(m.renderComposer(80), "composer inactive") {
		t.Fatalf("sidebar focus/hints out of sync: focus=%v status=%q composer=%q", m.focus, m.renderStatus(120), m.renderComposer(80))
	}

	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusTimeline || !strings.Contains(m.renderStatus(120), "timeline") || !strings.Contains(m.renderComposer(80), "composer inactive") {
		t.Fatalf("timeline focus/hints out of sync: focus=%v status=%q composer=%q", m.focus, m.renderStatus(120), m.renderComposer(80))
	}

	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusComposer || !strings.Contains(m.renderStatus(120), "at latest") || !strings.Contains(m.renderComposer(80), "to # Town") {
		t.Fatalf("composer focus/hints out of sync: focus=%v status=%q composer=%q", m.focus, m.renderStatus(120), m.renderComposer(80))
	}
}

func TestComposerFocusRoutesPrintableShortcutsToText(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.focus = focusComposer
	m.posts = []domain.Post{{ID: "p1", Message: "thread root"}}
	m.selectedPost = 0

	for _, r := range []rune{'/', 'a', 't', '['} {
		updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	if got := m.composer.Value(); got != "/at[" {
		t.Fatalf("composer value = %q, want printable shortcuts inserted", got)
	}
	if m.filtering || m.activityOpen || m.threadOpen {
		t.Fatalf("printable shortcuts changed app mode while composing: filtering=%v activity=%v thread=%v", m.filtering, m.activityOpen, m.threadOpen)
	}
}

func TestCtrlBFocusesSidebarFromComposer(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.focus = focusComposer
	m.composer.SetValue("draft")

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	got := updated.(Model)

	if got.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", got.focus)
	}
	if got.composer.Value() != "draft" {
		t.Fatalf("composer value changed: %q", got.composer.Value())
	}
}

func TestSidebarHotkeyClosesOverlays(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.focus = focusComposer
	m.infoOpen = true
	m.activityOpen = true
	m.switcherOpen = true
	m.triageOpen = true

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	got := updated.(Model)

	if got.focus != focusSidebar || got.infoOpen || got.activityOpen || got.switcherOpen || got.triageOpen {
		t.Fatalf("sidebar hotkey should focus sidebar and close overlays: %#v", got)
	}
}

func TestSidebarHotkeyClosesThreadAndRestoresChannelDraft(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.loadDraft(threadDraftKey("dev", "root"))
	m.composer.SetValue("thread draft")

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	got := updated.(Model)

	if got.focus != focusSidebar || got.threadOpen {
		t.Fatalf("sidebar hotkey should focus sidebar and close thread: focus=%v threadOpen=%v", got.focus, got.threadOpen)
	}
	if got.drafts[threadDraftKey("dev", "root")] != "thread draft" {
		t.Fatalf("thread draft not saved: %#v", got.drafts)
	}
	if got.activeDraftKey != channelDraftKey("dev") {
		t.Fatalf("active draft key = %q, want channel draft", got.activeDraftKey)
	}
}

func TestAltNumberFocusesPanesDirectly(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	got := updated.(Model)
	if got.focus != focusTimeline {
		t.Fatalf("alt+2 focus = %v, want timeline", got.focus)
	}

	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	got = updated.(Model)
	if got.focus != focusComposer {
		t.Fatalf("alt+3 focus = %v, want composer", got.focus)
	}
}

func TestAltNumberNavigatesThreadPanesWithoutClosingThread(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadPosts = []domain.Post{{ID: "root", ChannelID: "dev"}}
	m.threadFocusComposer = true

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}, Alt: true})
	got := updated.(Model)
	if !got.threadOpen || got.threadFocusComposer {
		t.Fatalf("alt+4 should focus thread messages without closing: threadOpen=%v composer=%v", got.threadOpen, got.threadFocusComposer)
	}

	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	got = updated.(Model)
	if !got.threadOpen || got.focus != focusTimeline {
		t.Fatalf("alt+2 should focus timeline with thread open: threadOpen=%v focus=%v", got.threadOpen, got.focus)
	}

	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	got = updated.(Model)
	if !got.threadOpen || !got.threadFocusComposer {
		t.Fatalf("alt+3 should focus thread reply composer: threadOpen=%v composer=%v", got.threadOpen, got.threadFocusComposer)
	}
}

func TestEscInThreadComposerReturnsToThreadMessagesFirst(t *testing.T) {
	m := New(mock.New(), config.Config{Mock: true}, false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if !got.threadOpen || got.threadFocusComposer {
		t.Fatalf("first esc should keep thread open and focus messages: threadOpen=%v composer=%v", got.threadOpen, got.threadFocusComposer)
	}

	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(Model)
	if got.threadOpen {
		t.Fatal("second esc should close thread")
	}
}

func TestRussianHelpAndComposerLabels(t *testing.T) {
	m := New(nil, config.Config{Language: "ru"}, true)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	help := m.helpText()
	if !strings.Contains(help, "Клавиши") || !strings.Contains(help, "сайдбар") {
		t.Fatalf("russian help missing translations: %q", help)
	}
	composer := m.renderComposer(100)
	if !strings.Contains(composer, "enter отправить") || !strings.Contains(composer, "Написать сообщение") {
		t.Fatalf("russian composer missing translations: %q", composer)
	}
}

func TestRussianStatusTranslatesConnectionAndHints(t *testing.T) {
	m := New(nil, config.Config{Language: "ru"}, true)
	m.connectionState = domain.ConnectionReconnecting
	m.connectionRetryIn = 5 * time.Second
	m.status = "ready"
	m.focus = focusTimeline

	got := m.renderStatus(160)
	if !strings.Contains(got, "переподключение") || !strings.Contains(got, "готово") || !strings.Contains(got, "лента") {
		t.Fatalf("russian status missing translations: %q", got)
	}
}
