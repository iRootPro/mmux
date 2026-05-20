package app

import (
	"strconv"
	"strings"
	"time"

	"band-tui/internal/domain"
)

type pendingSend struct {
	draftKey        string
	text            string
	message         string
	attachmentPaths []string
	files           []domain.PostFile
	channelID       string
	rootID          string
	tempPostID      string
}

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
		m.pendingSends = map[string]pendingSend{}
	}
}

func (m Model) composerReady() bool {
	return m.composer.Placeholder != ""
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
	if !m.composerReady() {
		return
	}
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
	for id, pending := range m.pendingSends {
		if pending.draftKey == key {
			delete(m.pendingSends, id)
		}
	}
	if key == m.activeDraftKey && m.composerReady() {
		m.composer.Reset()
	}
}

func (m *Model) beginPendingSendPayload(key string, payload composerSendPayload) string {
	pendingID := m.beginPendingSend(key, payload.Original)
	pending := m.pendingSends[pendingID]
	pending.message = payload.Message
	pending.attachmentPaths = append([]string(nil), payload.AttachmentPaths...)
	pending.files = append([]domain.PostFile(nil), payload.Files...)
	m.pendingSends[pendingID] = pending
	return pendingID
}

func (m *Model) beginPendingSend(key, text string) string {
	if key == "" {
		return ""
	}
	m.ensureDraftMaps()
	m.nextPendingSendID++
	pendingID := strconv.FormatInt(m.nextPendingSendID, 10)
	m.pendingSends[pendingID] = pendingSend{draftKey: key, text: text, message: text}
	delete(m.drafts, key)
	if key == m.activeDraftKey && m.composerReady() {
		m.composer.Reset()
	}
	return pendingID
}

func (m *Model) attachPendingPost(pendingID, channelID, rootID string) domain.Post {
	m.ensureDraftMaps()
	pending, ok := m.pendingSends[pendingID]
	if !ok {
		return domain.Post{}
	}
	pending.channelID = channelID
	pending.rootID = rootID
	pending.tempPostID = "pending:" + pendingID
	m.pendingSends[pendingID] = pending
	post := domain.Post{
		ID:        pending.tempPostID,
		ChannelID: channelID,
		RootID:    rootID,
		Message:   pending.message,
		CreateAt:  time.Now().UnixMilli(),
		Delivery:  domain.DeliveryPending,
		Files:     append([]domain.PostFile(nil), pending.files...),
	}
	if m.session != nil {
		post.UserID = m.session.User.ID
		post.Username = m.session.User.DisplayName
		if post.Username == "" {
			post.Username = m.session.User.Username
		}
	}
	return m.normalizePost(post)
}

func (m *Model) completePendingSend(pendingID, draftKey string) (pendingSend, bool) {
	m.ensureDraftMaps()
	pending, ok := m.pendingSends[pendingID]
	if ok {
		draftKey = pending.draftKey
		delete(m.pendingSends, pendingID)
	}
	if draftKey == "" {
		return pending, ok
	}
	if strings.TrimSpace(m.drafts[draftKey]) == "" {
		delete(m.drafts, draftKey)
	}
	if draftKey == m.activeDraftKey && m.composerReady() && strings.TrimSpace(m.composer.Value()) == "" {
		m.composer.Reset()
	}
	return pending, ok
}

func (m *Model) restorePendingSend(pendingID, draftKey, text string) (pendingSend, bool) {
	m.ensureDraftMaps()
	pending, ok := m.pendingSends[pendingID]
	if ok {
		if draftKey == "" {
			draftKey = pending.draftKey
		}
		if text == "" {
			text = pending.text
		}
		delete(m.pendingSends, pendingID)
	}
	if draftKey == "" || strings.TrimSpace(text) == "" {
		return pending, ok
	}
	if draftKey == m.activeDraftKey && m.composerReady() {
		if strings.TrimSpace(m.composer.Value()) == "" {
			m.drafts[draftKey] = text
			m.composer.SetValue(text)
		}
		return pending, ok
	}
	if strings.TrimSpace(m.drafts[draftKey]) == "" {
		m.drafts[draftKey] = text
	}
	return pending, ok
}
