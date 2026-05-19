package app

import (
	"path/filepath"
	"strings"
	"testing"

	"band-tui/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsOpensFromComposerAndPersistsLanguage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(nil, config.Config{Config: path, Language: "en"}, true)
	m.focus = focusComposer

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true})
	got := updated.(Model)
	if !got.settingsOpen {
		t.Fatal("alt+, should open settings even from composer")
	}

	got.settingsSelected = settingsItemLanguage
	updated, cmd := got.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got = updated.(Model)
	if got.language() != languageRussian {
		t.Fatalf("language = %q", got.language())
	}
	if got.composer.Placeholder != "Написать сообщение…" {
		t.Fatalf("placeholder = %q", got.composer.Placeholder)
	}
	if cmd == nil {
		t.Fatal("language change should save config")
	}
	msg := cmd()
	if saved, ok := msg.(preferenceSavedMsg); !ok || saved.err != nil {
		t.Fatalf("save msg = %#v", msg)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != languageRussian {
		t.Fatalf("saved language = %q", cfg.Language)
	}
}

func TestSettingsRenderRussian(t *testing.T) {
	m := New(nil, config.Config{Language: "ru"}, true)
	m.settingsOpen = true
	got := m.renderSettings(100, 30)
	if !strings.Contains(got, "Настройки") || !strings.Contains(got, "Язык") || !strings.Contains(got, "русский") {
		t.Fatalf("settings modal missing russian labels: %q", got)
	}
}

func TestSwitcherCanOpenSettings(t *testing.T) {
	m := New(nil, config.Config{}, true)
	m.switcherOpen = true
	indexes := m.switcherCommandIndexes("settings")
	found := false
	for _, idx := range indexes {
		if idx == switcherOpenSettings {
			found = true
		}
	}
	if !found {
		t.Fatalf("settings command missing from switcher indexes: %#v", indexes)
	}
	updated, _ := m.executeSwitcherCommand(switcherOpenSettings)
	if !updated.(Model).settingsOpen {
		t.Fatal("switcher settings command should open settings")
	}
}

func TestSettingsCanSaveConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := New(nil, config.Config{Config: path, ServerURL: "https://old.example.com"}, true).openSettingsOverlay()
	m.settingsSelected = settingsItemServer

	updated, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = updated.(Model)
	for _, r := range []rune("https://chat.example.com/") {
		updated, _ = m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, cmd := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.cfg.ServerURL != "https://chat.example.com" || !strings.Contains(m.status, "restart") {
		t.Fatalf("server not saved into model: url=%q status=%q", m.cfg.ServerURL, m.status)
	}
	if cmd == nil {
		t.Fatal("expected save command")
	}
	if msg := cmd(); msg.(preferenceSavedMsg).err != nil {
		t.Fatalf("save failed: %#v", msg)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "https://chat.example.com" {
		t.Fatalf("saved server = %q", cfg.ServerURL)
	}
}
