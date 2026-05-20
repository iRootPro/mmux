package app

import (
	"fmt"
	"strings"

	"band-tui/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	settingsItemLanguage = iota
	settingsItemServer
	settingsItemToken
	settingsItemSave
	settingsItemCount
)

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
	m.settingsEditing = false
	m.settingsDraftServer = m.cfg.ServerURL
	m.settingsDraftToken = m.cfg.Token
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
	if m.settingsEditing {
		return m.handleSettingsEditKey(msg)
	}
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc", "q":
		m.settingsOpen = false
		return m, nil
	case "up", "k", "shift+tab":
		m.settingsSelected = (m.settingsSelected + settingsItemCount - 1) % settingsItemCount
		return m, nil
	case "down", "j", "tab":
		m.settingsSelected = (m.settingsSelected + 1) % settingsItemCount
		return m, nil
	case "left", "h", "right", "l", " ":
		if m.settingsSelected == settingsItemLanguage {
			return m.cycleLanguageSetting()
		}
		return m, nil
	case "enter":
		switch m.settingsSelected {
		case settingsItemServer, settingsItemToken:
			m.settingsEditing = true
			return m, nil
		case settingsItemLanguage:
			return m.cycleLanguageSetting()
		case settingsItemSave:
			return m.saveConnectionSettings()
		}
	}
	if len(msg.Runes) > 0 {
		switch msg.Runes[0] {
		case 'r', 'R':
			if m.settingsSelected == settingsItemLanguage {
				return m.setLanguageSetting(languageRussian)
			}
		case 'e', 'E':
			if m.settingsSelected == settingsItemLanguage {
				return m.setLanguageSetting(languageEnglish)
			}
		case 's', 'S':
			return m.saveConnectionSettings()
		}
	}
	return m, nil
}

func (m Model) handleSettingsEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		_ = m.backend.Close()
		return m, tea.Quit
	case "esc":
		m.settingsEditing = false
		m.settingsDraftServer = m.cfg.ServerURL
		m.settingsDraftToken = m.cfg.Token
		return m, nil
	case "enter":
		m.settingsEditing = false
		return m.saveConnectionSettings()
	case "backspace", "ctrl+h":
		m.backspaceSettingsDraft()
		return m, nil
	case "ctrl+u":
		m.clearSettingsDraft()
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.appendSettingsDraft(string(msg.Runes))
		return m, nil
	}
	return m, nil
}

func (m *Model) appendSettingsDraft(s string) {
	switch m.settingsSelected {
	case settingsItemServer:
		m.settingsDraftServer += s
	case settingsItemToken:
		m.settingsDraftToken += s
	}
}

func (m *Model) backspaceSettingsDraft() {
	switch m.settingsSelected {
	case settingsItemServer:
		m.settingsDraftServer = dropLastRune(m.settingsDraftServer)
	case settingsItemToken:
		m.settingsDraftToken = dropLastRune(m.settingsDraftToken)
	}
}

func (m *Model) clearSettingsDraft() {
	switch m.settingsSelected {
	case settingsItemServer:
		m.settingsDraftServer = ""
	case settingsItemToken:
		m.settingsDraftToken = ""
	}
}

func dropLastRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return string(runes[:len(runes)-1])
}

func (m Model) saveConnectionSettings() (tea.Model, tea.Cmd) {
	serverURL := config.NormalizeServerURL(m.settingsDraftServer)
	token := strings.TrimSpace(m.settingsDraftToken)
	if serverURL == "" {
		m.status = "server URL is required"
		return m, nil
	}
	changedConnection := serverURL != m.cfg.ServerURL || token != m.cfg.Token
	m.cfg.ServerURL = serverURL
	m.cfg.Token = token
	m.status = "connection settings saved"
	if changedConnection || m.setupRequired {
		m.status = "connection settings saved · restart to connect"
		m.setupRequired = false
	}
	return m, saveConnectionSettingsCmd(m.cfg.Config, m.cfg)
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
