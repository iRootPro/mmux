package app

import (
	"testing"

	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTeamSwitcherOpensOutsideComposer(t *testing.T) {
	m := Model{focus: focusSidebar, session: &domain.Session{Teams: []domain.Team{{ID: "t1"}, {ID: "t2"}}}}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if !updated.(Model).teamSwitcherOpen {
		t.Fatal("team switcher did not open")
	}
}

func TestTeamSwitcherOpensWithW(t *testing.T) {
	m := Model{focus: focusSidebar, session: &domain.Session{Teams: []domain.Team{{ID: "t1"}, {ID: "t2"}}}}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if !updated.(Model).teamSwitcherOpen {
		t.Fatal("team switcher did not open with w")
	}
}

func TestTeamSwitcherOpensWithCtrlGWhileTyping(t *testing.T) {
	m := Model{focus: focusComposer, session: &domain.Session{Teams: []domain.Team{{ID: "t1"}, {ID: "t2"}}}}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlG})
	if !updated.(Model).teamSwitcherOpen {
		t.Fatal("team switcher did not open with ctrl+g")
	}
}

func TestTeamSwitcherDoesNotOpenWithWWhileTyping(t *testing.T) {
	m := Model{focus: focusComposer, session: &domain.Session{Teams: []domain.Team{{ID: "t1"}, {ID: "t2"}}}}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if updated.(Model).teamSwitcherOpen {
		t.Fatal("team switcher opened while composer focused")
	}
}

func TestSwitchTeamClearsCurrentChannelState(t *testing.T) {
	m := Model{
		session:        &domain.Session{Teams: []domain.Team{{ID: "t1"}, {ID: "t2"}}},
		selectedTeam:   0,
		channels:       []domain.Channel{{ID: "c1"}},
		posts:          []domain.Post{{ID: "p1"}},
		postsByChannel: map[string][]domain.Post{"c1": {{ID: "p1"}}},
	}
	updated, _ := m.switchTeam(1)
	got := updated.(Model)
	if got.selectedTeam != 1 || len(got.channels) != 0 || len(got.posts) != 0 || !got.loading || got.status != "loading scope…" {
		t.Fatalf("bad switched model: %#v", got)
	}
}
