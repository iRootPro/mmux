package app

import (
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func testConfig() config.Config { return config.Config{Mock: true} }

func draftKey(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
func TestDraftKeysAreStable(t *testing.T) {
	if got := channelDraftKey("dev"); got != "channel:dev" {
		t.Fatalf("channel key = %q", got)
	}
	if got := threadDraftKey("dev", "root-1"); got != "thread:dev:root-1" {
		t.Fatalf("thread key = %q", got)
	}
}

func TestSwitchDraftSavesAndLoadsDestinationText(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts"},
	}
	m.selectedChannel = 0
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("dev draft")

	m.switchDraft(channelDraftKey("alerts"))
	if got := m.composer.Value(); got != "" {
		t.Fatalf("new destination composer = %q, want empty", got)
	}

	m.composer.SetValue("alerts draft")
	m.switchDraft(channelDraftKey("dev"))
	if got := m.composer.Value(); got != "dev draft" {
		t.Fatalf("restored dev draft = %q", got)
	}
	if got := m.drafts[channelDraftKey("alerts")]; got != "alerts draft" {
		t.Fatalf("saved alerts draft = %q", got)
	}
}

func TestSaveActiveDraftDropsEmptyDraft(t *testing.T) {
	m := New(nil, testConfig(), false)
	key := channelDraftKey("dev")
	m.drafts[key] = "old"
	m.loadDraft(key)
	m.composer.SetValue("   ")

	m.saveActiveDraft()
	if _, ok := m.drafts[key]; ok {
		t.Fatalf("empty draft should be removed: %#v", m.drafts)
	}
}

func TestChannelDraftSurvivesChannelSwitch(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts"},
	}
	m.selectedChannel = 0
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("dev text")

	updated, _ := m.selectChannel(1)
	m = updated.(Model)
	if got := m.composer.Value(); got != "" {
		t.Fatalf("alerts composer = %q, want empty", got)
	}

	m.composer.SetValue("alerts text")
	updated, _ = m.selectChannel(0)
	m = updated.(Model)
	if got := m.composer.Value(); got != "dev text" {
		t.Fatalf("dev draft restored = %q", got)
	}
	if got := m.drafts[channelDraftKey("alerts")]; got != "alerts text" {
		t.Fatalf("alerts draft saved = %q", got)
	}
}

func TestInitialChannelLoadsChannelDraftKey(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0

	m.switchDraft(m.currentDraftKey())
	if got := m.activeDraftKey; got != channelDraftKey("dev") {
		t.Fatalf("active draft key = %q", got)
	}
}

func TestThreadDraftSurvivesCloseAndReopen(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.posts = []domain.Post{{ID: "root", ChannelID: "dev", Username: "Alice", Message: "root"}}
	m.selectedPost = 0
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("channel text")

	updated, _ := m.openSelectedThread()
	m = updated.(Model)
	if got := m.activeDraftKey; got != threadDraftKey("dev", "root") {
		t.Fatalf("active thread draft key = %q", got)
	}
	if got := m.composer.Value(); got != "" {
		t.Fatalf("new thread composer = %q, want empty", got)
	}

	m.composer.SetValue("reply text")
	updated, _ = m.handleThreadKey(draftKey("esc"))
	m = updated.(Model)
	if got := m.composer.Value(); got != "channel text" {
		t.Fatalf("channel draft restored after closing thread = %q", got)
	}

	updated, _ = m.openSelectedThread()
	m = updated.(Model)
	if got := m.composer.Value(); got != "reply text" {
		t.Fatalf("thread draft restored = %q", got)
	}
}

func TestChannelAndThreadDraftsAreIsolated(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.drafts[channelDraftKey("dev")] = "channel text"
	m.drafts[threadDraftKey("dev", "root")] = "reply text"

	m.loadDraft(channelDraftKey("dev"))
	if got := m.composer.Value(); got != "channel text" {
		t.Fatalf("channel composer = %q", got)
	}
	m.loadDraft(threadDraftKey("dev", "root"))
	if got := m.composer.Value(); got != "reply text" {
		t.Fatalf("thread composer = %q", got)
	}
}

func TestFailedChannelSendRestoresDraft(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("hello")

	updated, _ := m.handleKey(draftKey("enter"))
	m = updated.(Model)
	if got := m.composer.Value(); got != "" {
		t.Fatalf("composer should clear while sending, got %q", got)
	}

	key := channelDraftKey("dev")
	updated, _ = m.Update(postSentMsg{channelID: "dev", draftKey: key, text: "hello", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "hello" {
		t.Fatalf("failed send restored composer = %q", got.composer.Value())
	}
	if got.drafts[key] != "hello" {
		t.Fatalf("failed send restored draft = %q", got.drafts[key])
	}
	if got.status != "send failed · draft restored" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestSuccessfulChannelSendClearsOnlySentDraft(t *testing.T) {
	key := channelDraftKey("dev")
	other := channelDraftKey("alerts")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(key)
	m.drafts[key] = "hello"
	m.drafts[other] = "keep me"
	m.pendingSends[key] = "hello"

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, text: "hello", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "hello"}})
	got := updated.(Model)
	if _, ok := got.drafts[key]; ok {
		t.Fatalf("sent draft should clear: %#v", got.drafts)
	}
	if got.drafts[other] != "keep me" {
		t.Fatalf("other draft lost: %#v", got.drafts)
	}
}
