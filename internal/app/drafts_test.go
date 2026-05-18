package app

import (
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
)

func testConfig() config.Config { return config.Config{Mock: true} }

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
