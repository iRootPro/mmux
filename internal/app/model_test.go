package app

import (
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFilteredChannelIndexes(t *testing.T) {
	m := Model{
		favoriteChannels: map[string]bool{},
		channels: []domain.Channel{
			{ID: "1", Name: "alice", DisplayName: "Alice", Type: "D"},
			{ID: "2", Name: "dev", DisplayName: "Development", Type: "O"},
			{ID: "3", Name: "random", DisplayName: "Random", Type: "O"},
			{ID: "4", Name: "group", DisplayName: "Group", Type: "G"},
		},
	}

	m.channelFilter = "dev"
	got := m.filteredChannelIndexes()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("got %#v, want [1]", got)
	}

	m.channelFilter = ""
	got = m.filteredChannelIndexes()
	want := []int{1, 2, 0, 3}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestFilteredChannelIndexesSkipsCollapsedSections(t *testing.T) {
	m := Model{
		favoriteChannels:  map[string]bool{},
		collapsedSections: map[string]bool{sectionDirect: true},
		channels: []domain.Channel{
			{ID: "1", Name: "alice", DisplayName: "Alice", Type: "D"},
			{ID: "2", Name: "dev", DisplayName: "Development", Type: "O"},
			{ID: "3", Name: "group", DisplayName: "Group", Type: "G"},
		},
	}
	got := m.filteredChannelIndexes()
	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestFilteredChannelIndexesShowsFavoritesFirst(t *testing.T) {
	m := Model{
		favoriteChannels: map[string]bool{"1": true},
		channels: []domain.Channel{
			{ID: "1", Name: "alice", DisplayName: "Alice", Type: "D"},
			{ID: "2", Name: "dev", DisplayName: "Development", Type: "O"},
			{ID: "3", Name: "group", DisplayName: "Group", Type: "G"},
		},
	}
	got := m.filteredChannelIndexes()
	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestEnsureSelectedVisible(t *testing.T) {
	m := Model{
		favoriteChannels: map[string]bool{},
		selectedChannel:  0,
		channelFilter:    "dev",
		channels: []domain.Channel{
			{ID: "1", Name: "town-square", DisplayName: "Town Square"},
			{ID: "2", Name: "dev", DisplayName: "Development"},
		},
	}
	if !m.ensureSelectedVisible() {
		t.Fatal("expected selection to change")
	}
	if m.selectedChannel != 1 {
		t.Fatalf("selectedChannel = %d, want 1", m.selectedChannel)
	}
	if m.ensureSelectedVisible() {
		t.Fatal("second call should not change selection")
	}
}

func TestSwitcherIndexesUsesVisualSectionOrder(t *testing.T) {
	m := Model{
		favoriteChannels: map[string]bool{"1": true},
		channels: []domain.Channel{
			{ID: "1", Name: "alice", DisplayName: "Alice", Type: "D"},
			{ID: "2", Name: "dev", DisplayName: "Development", Type: "O"},
			{ID: "3", Name: "group", DisplayName: "Group", Type: "G"},
		},
	}
	got := m.switcherIndexes()
	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestSwitcherIndexesFuzzyMatch(t *testing.T) {
	m := Model{
		favoriteChannels: map[string]bool{},
		switcherQuery:    "dv",
		channels: []domain.Channel{
			{ID: "1", Name: "devops", DisplayName: "DevOps", Type: "O"},
			{ID: "2", Name: "random", DisplayName: "Random", Type: "O"},
		},
	}
	got := m.switcherIndexes()
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("got %#v, want [0]", got)
	}
}

func TestSwitcherIncludesGoToCommandsWhenOpen(t *testing.T) {
	m := Model{
		switcherOpen:     true,
		favoriteChannels: map[string]bool{},
		channels:         []domain.Channel{{ID: "1", Name: "dev", DisplayName: "Development", Type: "O"}},
	}
	got := m.switcherIndexes()
	if len(got) < 2 || got[0] != switcherGoSidebar || got[1] != switcherGoTimeline {
		t.Fatalf("switcher indexes should start with commands, got %#v", got)
	}
}

func TestSwitcherCommandFocusesTimeline(t *testing.T) {
	m := New(nil, config.Config{}, true)
	m.switcherOpen = true
	m.switcherSelected = 0
	m.switcherQuery = "timeline"
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	updated, _ := m.handleSwitcherKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.switcherOpen || got.focus != focusTimeline {
		t.Fatalf("switcher command should close and focus timeline: open=%v focus=%v", got.switcherOpen, got.focus)
	}
}
