package app

import (
	"path/filepath"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
)

func TestNewRestoresDiskCacheForImmediateStartup(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	cfg := config.Config{ServerURL: "https://chat.example.com", Token: "token"}
	session := &domain.Session{ServerURL: cfg.ServerURL, User: domain.User{ID: "me", Username: "me"}, Teams: []domain.Team{{ID: "team-1", Name: "team"}}}
	cache := appCache{
		Version:           appCacheVersion,
		ServerURL:         cfg.ServerURL,
		Session:           session,
		SelectedTeam:      0,
		SelectedChannelID: "dev",
		ChannelsByScope: map[string][]domain.Channel{
			"team-1": {{ID: "dev", TeamID: "team-1", DisplayName: "Dev", Type: "O"}},
		},
		PostsByChannel: map[string][]domain.Post{
			"dev": {{ID: "p1", ChannelID: "dev", Username: "Alice", Message: "cached message"}},
		},
	}
	if err := saveAppCache(cachePath(cfg), cache); err != nil {
		t.Fatal(err)
	}

	m := New(nil, cfg, false)
	if m.session == nil || len(m.channels) != 1 || m.currentChannelID() != "dev" || len(m.posts) != 1 || m.posts[0].Message != "cached message" {
		t.Fatalf("cache was not restored: session=%#v channels=%#v posts=%#v", m.session, m.channels, m.posts)
	}
	if m.status != "cached · connecting…" || !m.loading {
		t.Fatalf("unexpected cached startup status/loading: %q %v", m.status, m.loading)
	}
}

func TestSwitchTeamShowsCachedScopeImmediately(t *testing.T) {
	m := New(nil, config.Config{}, true)
	m.session = &domain.Session{Teams: []domain.Team{{ID: "t1", Name: "one"}, {ID: "t2", Name: "two"}}}
	m.selectedTeam = 0
	m.channels = []domain.Channel{{ID: "c1", TeamID: "t1", DisplayName: "One"}}
	m.scopeChannels = map[string][]domain.Channel{
		"t2": {{ID: "c2", TeamID: "t2", DisplayName: "Two"}},
	}
	m.postsByChannel = map[string][]domain.Post{
		"c2": {{ID: "p2", ChannelID: "c2", Message: "cached in t2"}},
	}

	updated, _ := m.switchTeam(1)
	got := updated.(Model)
	if got.selectedTeam != 1 || got.currentChannelID() != "c2" || len(got.posts) != 1 || got.posts[0].Message != "cached in t2" {
		t.Fatalf("cached team was not shown immediately: team=%d channels=%#v posts=%#v", got.selectedTeam, got.channels, got.posts)
	}
	if got.status != "cached · refreshing scope…" {
		t.Fatalf("status = %q", got.status)
	}
}
