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

func pendingID(p map[string]pendingSend) string {
	for id := range p {
		return id
	}
	return ""
}
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
	m.composer.SetValue("hello")
	m.drafts[other] = "keep me"
	updated, _ := m.handleKey(draftKey("enter"))
	m = updated.(Model)
	if len(m.pendingSends) != 1 {
		t.Fatalf("expected one pending channel send, got %#v", m.pendingSends)
	}
	var pendingID string
	for id := range m.pendingSends {
		pendingID = id
	}
	updated, _ = m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: pendingID, text: "hello", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "hello"}})
	got := updated.(Model)
	if _, ok := got.drafts[key]; ok {
		t.Fatalf("sent draft should clear: %#v", got.drafts)
	}
	if got.drafts[other] != "keep me" {
		t.Fatalf("other draft lost: %#v", got.drafts)
	}
}

func TestFailedThreadReplyRestoresDraft(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	key := threadDraftKey("dev", "root")
	m.loadDraft(key)
	m.composer.SetValue("reply text")

	updated, _ := m.handleThreadKey(draftKey("enter"))
	m = updated.(Model)
	if got := m.composer.Value(); got != "" {
		t.Fatalf("composer should clear while sending reply, got %q", got)
	}

	updated, _ = m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: key, pendingID: pendingID(m.pendingSends), text: "reply text", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "reply text" {
		t.Fatalf("failed reply restored composer = %q", got.composer.Value())
	}
	if got.status != "reply failed · draft restored" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestSuccessfulThreadReplyClearsOnlyThreadDraft(t *testing.T) {
	threadKey := threadDraftKey("dev", "root")
	channelKey := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.loadDraft(threadKey)
	m.composer.SetValue("reply")
	m.drafts[channelKey] = "channel"
	updated, _ := m.handleThreadKey(draftKey("enter"))
	m = updated.(Model)
	if len(m.pendingSends) != 1 {
		t.Fatalf("expected one pending thread send, got %#v", m.pendingSends)
	}
	var pendingID string
	for id := range m.pendingSends {
		pendingID = id
	}
	updated, _ = m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadKey, pendingID: pendingID, text: "reply", post: domain.Post{ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply"}})
	got := updated.(Model)
	if _, ok := got.drafts[threadKey]; ok {
		t.Fatalf("sent thread draft should clear: %#v", got.drafts)
	}
	if got.drafts[channelKey] != "channel" {
		t.Fatalf("channel draft lost: %#v", got.drafts)
	}
}

func TestFailedSendForInactiveChannelDoesNotOverwriteCurrentComposer(t *testing.T) {
	devKey := channelDraftKey("dev")
	alertsKey := channelDraftKey("alerts")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts"},
	}
	m.selectedChannel = 1
	m.loadDraft(alertsKey)
	m.composer.SetValue("current alerts text")
	m.pendingSends["1"] = pendingSend{draftKey: devKey, text: "failed dev text"}

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: devKey, pendingID: "1", text: "failed dev text", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "current alerts text" {
		t.Fatalf("inactive failure overwrote current composer: %q", got.composer.Value())
	}
	if got.drafts[devKey] != "failed dev text" {
		t.Fatalf("inactive failed draft not stored: %#v", got.drafts)
	}
}

func TestFailedReplyForInactiveThreadDoesNotOverwriteChannelComposer(t *testing.T) {
	threadKey := threadDraftKey("dev", "root")
	channelKey := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.threadOpen = false
	m.loadDraft(channelKey)
	m.composer.SetValue("channel text")
	m.pendingSends["1"] = pendingSend{draftKey: threadKey, text: "failed reply"}

	updated, _ := m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadKey, pendingID: "1", text: "failed reply", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "channel text" {
		t.Fatalf("inactive reply failure overwrote composer: %q", got.composer.Value())
	}
	if got.drafts[threadKey] != "failed reply" {
		t.Fatalf("inactive failed reply draft not stored: %#v", got.drafts)
	}
}

func TestSuccessfulSendDoesNotClearNewSameDestinationText(t *testing.T) {
	key := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(key)
	m.pendingSends["1"] = pendingSend{draftKey: key, text: "first"}
	m.composer.SetValue("second")

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: "1", text: "first", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "first"}})
	got := updated.(Model)
	if got.composer.Value() != "second" {
		t.Fatalf("new same-destination text was cleared: %q", got.composer.Value())
	}
}

func TestFailedSendDoesNotOverwriteNewSameDestinationText(t *testing.T) {
	key := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(key)
	m.pendingSends["1"] = pendingSend{draftKey: key, text: "first"}
	m.composer.SetValue("second")

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: "1", text: "first", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "second" {
		t.Fatalf("failed old send overwrote new same-destination text: %q", got.composer.Value())
	}
	if got.drafts[key] == "first" {
		t.Fatalf("failed old send overwrote newer draft state: %#v", got.drafts)
	}
}

func TestSendResponseDoesNotReplaceSavedNewerInactiveDraft(t *testing.T) {
	key := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts"},
	}
	m.selectedChannel = 1
	m.loadDraft(channelDraftKey("alerts"))
	m.pendingSends["1"] = pendingSend{draftKey: key, text: "first"}
	m.drafts[key] = "second"

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: "1", text: "first", err: assertErr{}})
	got := updated.(Model)
	if got.drafts[key] != "second" {
		t.Fatalf("inactive newer draft overwritten after failure: %#v", got.drafts)
	}

	got.pendingSends["1"] = pendingSend{draftKey: key, text: "first"}
	updated, _ = got.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: "1", text: "first", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "first"}})
	got = updated.(Model)
	if got.drafts[key] != "second" {
		t.Fatalf("inactive newer draft deleted after success: %#v", got.drafts)
	}
}

func TestOpenCurrentChannelSavesOldDraftAfterDirectChannelMutation(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts"},
	}
	m.selectedChannel = 0
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("dev direct text")
	m.drafts[channelDraftKey("alerts")] = "alerts saved text"

	m.selectedChannel = 1
	updated, _ := m.openCurrentChannel()
	got := updated.(Model)
	if got.drafts[channelDraftKey("dev")] != "dev direct text" {
		t.Fatalf("old draft not saved after direct selectedChannel mutation: %#v", got.drafts)
	}
	if got.composer.Value() != "alerts saved text" {
		t.Fatalf("new channel draft not loaded after direct selectedChannel mutation: %q", got.composer.Value())
	}
}

func TestSwitchTeamSavesDraftAndClearsActiveComposer(t *testing.T) {
	m := New(nil, testConfig(), false)
	m.session = &domain.Session{Teams: []domain.Team{{ID: "one", Name: "one"}, {ID: "two", Name: "two"}}}
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedTeam = 0
	m.selectedChannel = 0
	key := channelDraftKey("dev")
	m.loadDraft(key)
	m.composer.SetValue("team scoped draft")

	updated, _ := m.switchTeam(1)
	got := updated.(Model)
	if got.drafts[key] != "team scoped draft" {
		t.Fatalf("team switch should save active draft before clearing scope: %#v", got.drafts)
	}
	if got.activeDraftKey != "" {
		t.Fatalf("active draft key should clear on team switch, got %q", got.activeDraftKey)
	}
	if got.composer.Value() != "" {
		t.Fatalf("composer should clear on team switch, got %q", got.composer.Value())
	}
}

func TestTriageThreadOpenSwitchesFromChannelDraftToThreadDraft(t *testing.T) {
	channelKey := channelDraftKey("dev")
	threadKey := threadDraftKey("dev", "root")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.triageOpen = true
	m.triageItems = []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root", PostID: "reply"}}
	m.drafts[threadKey] = "saved reply draft"
	m.loadDraft(channelKey)
	m.composer.SetValue("channel draft before triage")

	updated, _ := m.handleTriageKey(draftKey("enter"))
	got := updated.(Model)
	if got.drafts[channelKey] != "channel draft before triage" {
		t.Fatalf("triage thread open did not save channel draft: %#v", got.drafts)
	}
	if got.activeDraftKey != threadKey {
		t.Fatalf("active draft key = %q, want %q", got.activeDraftKey, threadKey)
	}
	if got.composer.Value() != "saved reply draft" {
		t.Fatalf("thread draft not restored after triage open: %q", got.composer.Value())
	}
}

func TestTriageUnreadChannelOpenSwitchesChannelDrafts(t *testing.T) {
	devKey := channelDraftKey("dev")
	alertsKey := channelDraftKey("alerts")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 1},
	}
	m.selectedChannel = 0
	m.triageOpen = true
	m.triageItems = []triageItem{{Kind: triageUnreadChannel, ChannelID: "alerts", Title: "#alerts", UnreadCount: 1}}
	m.drafts[alertsKey] = "alerts draft"
	m.loadDraft(devKey)
	m.composer.SetValue("dev draft before triage")

	updated, _ := m.handleTriageKey(draftKey("enter"))
	got := updated.(Model)
	if got.drafts[devKey] != "dev draft before triage" {
		t.Fatalf("triage channel open did not save old channel draft: %#v", got.drafts)
	}
	if got.activeDraftKey != alertsKey || got.composer.Value() != "alerts draft" {
		t.Fatalf("triage channel open did not load target draft: key=%q composer=%q", got.activeDraftKey, got.composer.Value())
	}
}

func TestSuccessfulSendDoesNotClearNewIdenticalSameDestinationText(t *testing.T) {
	key := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(key)
	pendingID := m.beginPendingSend(key, "ok")
	m.composer.SetValue("ok")

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: pendingID, text: "ok", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "ok"}})
	got := updated.(Model)
	if got.composer.Value() != "ok" {
		t.Fatalf("new identical same-destination text was cleared: %q", got.composer.Value())
	}
}

func TestMultiplePendingSendsForSameDestinationRestoreIndependently(t *testing.T) {
	key := channelDraftKey("dev")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(key)
	firstID := m.beginPendingSend(key, "first")
	secondID := m.beginPendingSend(key, "second")

	updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: firstID, text: "first", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "first"}})
	m = updated.(Model)
	if _, ok := m.pendingSends[secondID]; !ok {
		t.Fatalf("first response cleared second pending send: %#v", m.pendingSends)
	}

	updated, _ = m.Update(postSentMsg{channelID: "dev", draftKey: key, pendingID: secondID, text: "second", err: assertErr{}})
	got := updated.(Model)
	if got.composer.Value() != "second" {
		t.Fatalf("second failed send was not restored independently: %q", got.composer.Value())
	}
}

func TestTriageChannelOpenFromThreadClosesThreadAndLoadsChannelDraft(t *testing.T) {
	threadKey := threadDraftKey("dev", "root")
	alertsKey := channelDraftKey("alerts")
	m := New(nil, testConfig(), false)
	m.channels = []domain.Channel{
		{ID: "dev", Type: "O", DisplayName: "dev"},
		{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 1},
	}
	m.selectedChannel = 0
	m.threadOpen = true
	m.threadRootID = "root"
	m.threadFocusComposer = true
	m.triageOpen = true
	m.triageItems = []triageItem{{Kind: triageUnreadChannel, ChannelID: "alerts", Title: "#alerts", UnreadCount: 1}}
	m.loadDraft(threadKey)
	m.composer.SetValue("thread reply draft")
	m.drafts[alertsKey] = "alerts channel draft"

	updated, _ := m.handleTriageKey(draftKey("enter"))
	got := updated.(Model)
	if got.threadOpen || got.threadRootID != "" {
		t.Fatalf("triage channel open should close stale thread, threadOpen=%v root=%q", got.threadOpen, got.threadRootID)
	}
	if got.drafts[threadKey] != "thread reply draft" {
		t.Fatalf("thread draft not saved before triage channel open: %#v", got.drafts)
	}
	if got.activeDraftKey != alertsKey || got.composer.Value() != "alerts channel draft" {
		t.Fatalf("target channel draft not loaded: key=%q composer=%q", got.activeDraftKey, got.composer.Value())
	}
}
