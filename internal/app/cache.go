package app

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"band-tui/internal/config"
	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

const appCacheVersion = 1

type appCache struct {
	Version           int                         `json:"version"`
	ServerURL         string                      `json:"server_url"`
	UpdatedAt         int64                       `json:"updated_at"`
	Session           *domain.Session             `json:"session,omitempty"`
	SelectedTeam      int                         `json:"selected_team"`
	SelectedChannelID string                      `json:"selected_channel_id,omitempty"`
	ChannelsByScope   map[string][]domain.Channel `json:"channels_by_scope,omitempty"`
	PostsByChannel    map[string][]domain.Post    `json:"posts_by_channel,omitempty"`
}

type cacheSavedMsg struct{ err error }

func cachePath(cfg config.Config) string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	server := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	if server == "" {
		server = "default"
	}
	sum := sha1.Sum([]byte(server))
	return filepath.Join(base, "mmux", "cache-"+hex.EncodeToString(sum[:8])+".json")
}

func loadAppCache(cfg config.Config) appCache {
	var cache appCache
	b, err := os.ReadFile(cachePath(cfg))
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(b, &cache); err != nil || cache.Version != appCacheVersion {
		return appCache{}
	}
	return cache
}

func saveAppCache(path string, cache appCache) error {
	if path == "" {
		return nil
	}
	if cache.Version == 0 {
		cache.Version = appCacheVersion
	}
	cache.UpdatedAt = time.Now().UnixMilli()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func saveAppCacheCmd(cfg config.Config, cache appCache) tea.Cmd {
	if cfg.Mock {
		return nil
	}
	path := cachePath(cfg)
	return func() tea.Msg {
		err := saveAppCache(path, cache)
		if errors.Is(err, os.ErrPermission) {
			return cacheSavedMsg{err: err}
		}
		return cacheSavedMsg{err: err}
	}
}

func (m *Model) applyCachedState(cache appCache) {
	if cache.Version != appCacheVersion || cache.Session == nil {
		return
	}
	if m.scopeChannels == nil {
		m.scopeChannels = map[string][]domain.Channel{}
	}
	for scope, channels := range cache.ChannelsByScope {
		m.scopeChannels[scope] = append([]domain.Channel(nil), channels...)
	}
	if m.postsByChannel == nil {
		m.postsByChannel = map[string][]domain.Post{}
	}
	for channelID, posts := range cache.PostsByChannel {
		m.postsByChannel[channelID] = append([]domain.Post(nil), posts...)
	}
	m.session = cache.Session
	m.selectedTeam = cache.SelectedTeam
	if m.selectedTeam < allScopesTeamIndex || (m.selectedTeam >= len(m.session.Teams) && len(m.session.Teams) > 0) {
		m.selectedTeam = m.pickTeam()
	}
	m.channels = append([]domain.Channel(nil), m.scopeChannels[m.scopeKey()]...)
	if len(m.channels) == 0 {
		return
	}
	m.selectedChannel = m.channelIndexByID(cache.SelectedChannelID)
	if m.selectedChannel < 0 {
		m.selectedChannel = m.pickChannel()
	}
	if posts, ok := m.postsByChannel[m.currentChannelID()]; ok {
		m.posts = append([]domain.Post(nil), posts...)
		m.selectedPost = m.initialSelectedPost(m.currentChannelID())
	}
	m.loading = true
	m.status = "cached · connecting…"
	m.rebuildTriageItems()
	m.refreshViewport()
}

func (m Model) appCacheSnapshot() appCache {
	channelsByScope := make(map[string][]domain.Channel, len(m.scopeChannels)+1)
	for scope, channels := range m.scopeChannels {
		channelsByScope[scope] = append([]domain.Channel(nil), channels...)
	}
	if len(m.channels) > 0 {
		channelsByScope[m.scopeKey()] = append([]domain.Channel(nil), m.channels...)
	}
	postsByChannel := make(map[string][]domain.Post, len(m.postsByChannel))
	for channelID, posts := range m.postsByChannel {
		postsByChannel[channelID] = append([]domain.Post(nil), posts...)
	}
	return appCache{
		Version:           appCacheVersion,
		ServerURL:         strings.TrimRight(strings.TrimSpace(m.cfg.ServerURL), "/"),
		Session:           m.session,
		SelectedTeam:      m.selectedTeam,
		SelectedChannelID: m.currentChannelID(),
		ChannelsByScope:   channelsByScope,
		PostsByChannel:    postsByChannel,
	}
}

func (m Model) scopeKey() string {
	if m.selectedTeam == allScopesTeamIndex {
		return "all"
	}
	if m.session == nil || m.selectedTeam < 0 || m.selectedTeam >= len(m.session.Teams) {
		return "default"
	}
	if id := strings.TrimSpace(m.session.Teams[m.selectedTeam].ID); id != "" {
		return id
	}
	if name := strings.TrimSpace(m.session.Teams[m.selectedTeam].Name); name != "" {
		return name
	}
	return "team"
}

func (m Model) saveCacheCmd() tea.Cmd {
	if m.mockFallback || m.cfg.Mock || m.session == nil {
		return nil
	}
	return saveAppCacheCmd(m.cfg, m.appCacheSnapshot())
}
