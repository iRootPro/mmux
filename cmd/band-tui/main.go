package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"band-tui/internal/app"
	"band-tui/internal/config"
	"band-tui/internal/domain"
	"band-tui/internal/mattermost"
	"band-tui/internal/mock"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--help" || arg == "-h" {
			printHelp()
			return
		}
	}

	opts, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	switch opts.Command {
	case "tui", "":
		if err := runTUI(opts.Config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "doctor":
		if err := runDoctor(opts.Config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "auth":
		if err := runAuth(opts.Config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", opts.Command)
		printHelp()
		os.Exit(2)
	}
}

func runTUI(cfg config.Config) error {
	backend, mockFallback := backendForTUI(cfg)
	defer backend.Close()

	model := app.New(backend, cfg, mockFallback)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := program.Run()
	return err
}

func runDoctor(cfg config.Config) error {
	backend := backendForDoctor(cfg)
	defer backend.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	session, err := backend.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	fmt.Printf("server:    %s\n", session.ServerURL)
	fmt.Printf("user:      @%s (%s)\n", session.User.Username, session.User.ID)
	fmt.Printf("teams:     %d\n", len(session.Teams))
	if len(session.Teams) == 0 {
		return nil
	}
	team := pickTeam(session.Teams, cfg.Team)
	fmt.Printf("team:      %s (%s)\n", team.DisplayName, team.ID)
	channels, err := backend.LoadChannels(ctx, team.ID)
	if err != nil {
		return fmt.Errorf("channels: %w", err)
	}
	fmt.Printf("channels:  %d\n", len(channels))
	if len(channels) > 0 {
		ch := pickChannel(channels, cfg.Channel)
		fmt.Printf("channel:   %s (%s)\n", ch.DisplayName, ch.ID)
		posts, err := backend.LoadPosts(ctx, ch.ID, 5)
		if err != nil {
			return fmt.Errorf("posts: %w", err)
		}
		fmt.Printf("posts:     %d loaded\n", len(posts))
	}
	fmt.Println("status:    ok")
	return nil
}

func backendForTUI(cfg config.Config) (domain.Backend, bool) {
	if cfg.Mock || !config.HasCredentials(cfg) {
		return mock.New(), !cfg.Mock
	}
	return mattermost.New(cfg), false
}

func backendForDoctor(cfg config.Config) domain.Backend {
	if cfg.Mock {
		return mock.New()
	}
	return mattermost.New(cfg)
}

func pickTeam(teams []domain.Team, want string) domain.Team {
	for _, t := range teams {
		if want != "" && (t.ID == want || t.Name == want || t.DisplayName == want) {
			return t
		}
	}
	return teams[0]
}

func pickChannel(channels []domain.Channel, want string) domain.Channel {
	for _, ch := range channels {
		if want != "" && (ch.ID == want || ch.Name == want || ch.DisplayName == want) {
			return ch
		}
	}
	return channels[0]
}

func printHelp() {
	fmt.Print(`band-tui - minimal TUI for band.wb.ru / Mattermost

Usage:
  band-tui [flags]          open the TUI
  band-tui [flags] auth     browser OAuth helper; saves MMAUTHTOKEN
  band-tui [flags] doctor   check API access

Flags:
  --server URL      default https://band.wb.ru
  --token TOKEN     personal/session token
  --username USER   login/password auth user
  --password PASS   login/password auth password
  --team TEAM       preferred team name or ID
  --channel CH      preferred channel name or ID
  --lang LANG       UI language: en or ru
  --config PATH     config JSON path
  --mock            run against built-in mock data

Environment:
  BAND_URL, BAND_TOKEN, BAND_USERNAME, BAND_PASSWORD, BAND_TEAM, BAND_CHANNEL, BAND_LANG

`)
}
