package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

const settingsItemLanguage = 0

func isGlobalSettingsKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "alt+,", "ctrl+,":
		return true
	default:
		return false
	}
}

func (m Model) toggleSettingsOverlay() Model {
	if m.settingsOpen {
		m.settingsOpen = false
		return m
	}
	return m.openSettingsOverlay()
}

func (m Model) openSettingsOverlay() Model {
	m.settingsOpen = true
	m.settingsSelected = 0
	m.activityOpen = false
	m.switcherOpen = false
	m.switcherQuery = ""
	m.infoOpen = false
	m.teamSwitcherOpen = false
	m.reactionPickerOpen = false
	m.reactionPickerQuery = ""
	m.triageOpen = false
	m.filtering = false
	return m
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "q":
		m.settingsOpen = false
		return m, nil
	case "up", "k", "down", "j", "tab", "shift+tab":
		// Only one setting for now; keep these keys reserved as the modal grows.
		m.settingsSelected = settingsItemLanguage
		return m, nil
	case "left", "h", "right", "l", " ", "enter":
		return m.cycleLanguageSetting()
	}
	if len(msg.Runes) > 0 {
		switch msg.Runes[0] {
		case 'r', 'R':
			return m.setLanguageSetting(languageRussian)
		case 'e', 'E':
			return m.setLanguageSetting(languageEnglish)
		}
	}
	return m, nil
}

func (m Model) cycleLanguageSetting() (tea.Model, tea.Cmd) {
	if m.language() == languageRussian {
		return m.setLanguageSetting(languageEnglish)
	}
	return m.setLanguageSetting(languageRussian)
}

func (m Model) setLanguageSetting(language string) (tea.Model, tea.Cmd) {
	language = normalizeLanguage(language)
	if m.language() == language {
		return m, nil
	}
	m.cfg.Language = language
	m.applyLanguage()
	m.status = fmt.Sprintf("%s: %s", m.tr("language"), m.languageDisplayName())
	m.refreshViewport()
	m.refreshThreadViewport()
	return m, saveLanguageCmd(m.cfg.Config, m.cfg.ServerURL, language)
}

func (m *Model) applyLanguage() {
	m.composer.Placeholder = m.tr("Write a message…")
}

func (m Model) languageDisplayName() string {
	switch m.language() {
	case languageRussian:
		return m.tr("Russian")
	default:
		return m.tr("English")
	}
}
