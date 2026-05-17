package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestUpdateUserStatusUpdatesDMChannel(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", UserIDs: []string{"u2"}}}}
	m.updateUserStatus("u2", "online")
	if m.channels[0].Status != "online" {
		t.Fatalf("status = %q", m.channels[0].Status)
	}
}

func TestSidebarPresenceGlyph(t *testing.T) {
	m := Model{channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Alice", Status: "online"}}, selectedChannel: -1}
	line := m.renderSidebarChannelLine(0, 40)
	if !strings.Contains(line, "●") || !strings.Contains(line, "@ Alice") {
		t.Fatalf("line = %q", line)
	}
}
