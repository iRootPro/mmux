package app

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"

	"band-tui/internal/config"
	"band-tui/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

type sessionLoadedMsg struct {
	session *domain.Session
	err     error
}

type channelsLoadedMsg struct {
	channels []domain.Channel
	err      error
}

type postsLoadedMsg struct {
	channelID string
	posts     []domain.Post
	err       error
}

type olderPostsLoadedMsg struct {
	channelID string
	beforeID  string
	posts     []domain.Post
	err       error
}

type postSentMsg struct {
	channelID string
	draftKey  string
	text      string
	post      domain.Post
	err       error
}

type threadLoadedMsg struct {
	rootID string
	posts  []domain.Post
	err    error
}

type replySentMsg struct {
	channelID string
	rootID    string
	draftKey  string
	text      string
	post      domain.Post
	err       error
}

type backendEventMsg struct {
	event domain.Event
}

type watchStartedMsg struct{}

type preferenceSavedMsg struct {
	err error
}

type actionDoneMsg struct {
	status string
	err    error
}

type channelViewedMsg struct {
	channelID string
	err       error
}

func connectCmd(ctx context.Context, backend domain.Backend) tea.Cmd {
	return func() tea.Msg {
		session, err := backend.Connect(ctx)
		return sessionLoadedMsg{session: session, err: err}
	}
}

func loadChannelsCmd(ctx context.Context, backend domain.Backend, teamID string) tea.Cmd {
	return func() tea.Msg {
		channels, err := backend.LoadChannels(ctx, teamID)
		return channelsLoadedMsg{channels: channels, err: err}
	}
}

func loadAllChannelsCmd(ctx context.Context, backend domain.Backend, teams []domain.Team) tea.Cmd {
	return func() tea.Msg {
		seen := map[string]struct{}{}
		channels := make([]domain.Channel, 0)
		for _, team := range teams {
			teamChannels, err := backend.LoadChannels(ctx, team.ID)
			if err != nil {
				return channelsLoadedMsg{err: err}
			}
			for _, ch := range teamChannels {
				if ch.ID == "" {
					continue
				}
				if ch.TeamID == "" && ch.Type != "D" && ch.Type != "G" {
					ch.TeamID = team.ID
				}
				if _, ok := seen[ch.ID]; ok {
					continue
				}
				seen[ch.ID] = struct{}{}
				channels = append(channels, ch)
			}
		}
		sort.SliceStable(channels, func(i, j int) bool {
			if channels[i].Type != channels[j].Type {
				return channelTypeRank(channels[i].Type) < channelTypeRank(channels[j].Type)
			}
			return strings.ToLower(channels[i].DisplayName) < strings.ToLower(channels[j].DisplayName)
		})
		return channelsLoadedMsg{channels: channels}
	}
}

func channelTypeRank(t string) int {
	switch t {
	case "O", "P":
		return 0
	case "D":
		return 1
	case "G":
		return 2
	default:
		return 3
	}
}

func loadPostsCmd(ctx context.Context, backend domain.Backend, channelID string) tea.Cmd {
	return func() tea.Msg {
		posts, err := backend.LoadPosts(ctx, channelID, 80)
		return postsLoadedMsg{channelID: channelID, posts: posts, err: err}
	}
}

func viewChannelCmd(ctx context.Context, backend domain.Backend, channelID string) tea.Cmd {
	return func() tea.Msg {
		return channelViewedMsg{channelID: channelID, err: backend.ViewChannel(ctx, channelID)}
	}
}

func loadOlderPostsCmd(ctx context.Context, backend domain.Backend, channelID, beforeID string) tea.Cmd {
	return func() tea.Msg {
		posts, err := backend.LoadPostsBefore(ctx, channelID, beforeID, 80)
		return olderPostsLoadedMsg{channelID: channelID, beforeID: beforeID, posts: posts, err: err}
	}
}

func loadThreadCmd(ctx context.Context, backend domain.Backend, rootID string) tea.Cmd {
	return func() tea.Msg {
		posts, err := backend.LoadThread(ctx, rootID)
		return threadLoadedMsg{rootID: rootID, posts: posts, err: err}
	}
}

func sendPostCmd(ctx context.Context, backend domain.Backend, channelID, draftKey, text string) tea.Cmd {
	return func() tea.Msg {
		post, err := backend.SendPost(ctx, channelID, text)
		return postSentMsg{channelID: channelID, draftKey: draftKey, text: text, post: post, err: err}
	}
}

func sendReplyCmd(ctx context.Context, backend domain.Backend, channelID, rootID, draftKey, text string) tea.Cmd {
	return func() tea.Msg {
		post, err := backend.SendReply(ctx, channelID, rootID, text)
		return replySentMsg{channelID: channelID, rootID: rootID, draftKey: draftKey, text: text, post: post, err: err}
	}
}

func startWatchCmd(ctx context.Context, backend domain.Backend, events chan<- domain.Event) tea.Cmd {
	return func() tea.Msg {
		go func() {
			_ = backend.WatchPosts(ctx, events)
		}()
		return watchStartedMsg{}
	}
}

func waitEventCmd(events <-chan domain.Event) tea.Cmd {
	return func() tea.Msg {
		ev := <-events
		return backendEventMsg{event: ev}
	}
}

func savePreferenceCmd(configPath, serverURL, team, channel string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadFile(configPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return preferenceSavedMsg{err: err}
		}
		if cfg.ServerURL == "" {
			cfg.ServerURL = serverURL
		}
		cfg.Team = team
		cfg.Channel = channel
		return preferenceSavedMsg{err: config.SaveFile(configPath, cfg)}
	}
}

func saveFavoritesCmd(configPath string, favorites []string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadFile(configPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return preferenceSavedMsg{err: err}
		}
		cfg.FavoriteChannels = favorites
		return preferenceSavedMsg{err: config.SaveFile(configPath, cfg)}
	}
}
