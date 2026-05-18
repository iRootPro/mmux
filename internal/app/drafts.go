package app

import "strings"

func channelDraftKey(channelID string) string {
	if channelID == "" {
		return ""
	}
	return "channel:" + channelID
}

func threadDraftKey(channelID, rootID string) string {
	if channelID == "" || rootID == "" {
		return ""
	}
	return "thread:" + channelID + ":" + rootID
}

func (m Model) currentDraftKey() string {
	if m.threadOpen && m.threadRootID != "" {
		return threadDraftKey(m.currentChannelID(), m.threadRootID)
	}
	return channelDraftKey(m.currentChannelID())
}

func (m *Model) ensureDraftMaps() {
	if m.drafts == nil {
		m.drafts = map[string]string{}
	}
	if m.pendingSends == nil {
		m.pendingSends = map[string]string{}
	}
}

func (m *Model) saveActiveDraft() {
	if m.activeDraftKey == "" {
		return
	}
	m.ensureDraftMaps()
	if strings.TrimSpace(m.composer.Value()) == "" {
		delete(m.drafts, m.activeDraftKey)
		return
	}
	m.drafts[m.activeDraftKey] = m.composer.Value()
}

func (m *Model) loadDraft(key string) {
	m.ensureDraftMaps()
	m.activeDraftKey = key
	m.composer.SetValue(m.drafts[key])
}

func (m *Model) switchDraft(key string) {
	if key == m.activeDraftKey {
		return
	}
	m.saveActiveDraft()
	m.loadDraft(key)
}

func (m *Model) clearDraft(key string) {
	if key == "" {
		return
	}
	m.ensureDraftMaps()
	delete(m.drafts, key)
	delete(m.pendingSends, key)
	if key == m.activeDraftKey {
		m.composer.Reset()
	}
}
