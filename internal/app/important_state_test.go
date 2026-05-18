package app

import (
	"testing"

	"band-tui/internal/domain"
)

func TestReconcileChannelImportanceCountsRemainingSignals(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev"}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "reply-unread", ChannelID: "dev", RootID: "root-a", Unread: true},
				{ID: "reply-mentioned", ChannelID: "dev", RootID: "root-b", Mentioned: true},
				{ID: "root-a", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
				{ID: "root-b", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
				{ID: "root-c", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
			},
		},
	}

	m.reconcileChannelImportance("dev")

	if m.channels[0].Unread != 2 || m.channels[0].Mentions != 1 {
		t.Fatalf("reconciled counters = %#v", m.channels[0])
	}
}

func TestApplyThreadReadReconcilesFromRemainingSignals(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Unread: 3, Mentions: 1}},
		postsByChannel: map[string][]domain.Post{
			"dev": {
				{ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 2},
				{ID: "reply-1", ChannelID: "dev", RootID: "root", Unread: true},
				{ID: "reply-2", ChannelID: "dev", RootID: "root", Mentioned: true},
				{ID: "other", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1},
			},
		},
	}

	m.applyThreadRead("dev", "root")

	if m.channels[0].Unread != 1 || m.channels[0].Mentions != 0 {
		t.Fatalf("reconciled counters = %#v", m.channels[0])
	}
}
