package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config contains CLI/config-file/env settings.
type Config struct {
	ServerURL        string   `json:"server_url,omitempty"`
	Token            string   `json:"token,omitempty"`
	Username         string   `json:"username,omitempty"`
	Password         string   `json:"password,omitempty"`
	Team             string   `json:"team,omitempty"`
	Channel          string   `json:"channel,omitempty"`
	FavoriteChannels []string `json:"favorite_channels,omitempty"`
	Config           string   `json:"-"`
	Mock             bool     `json:"-"`
}

// Options are top-level command line options.
type Options struct {
	Command string
	Config  Config
}

func Parse(args []string) (Options, error) {
	fs := flag.NewFlagSet("band-tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cfg Config
	fs.StringVar(&cfg.Config, "config", defaultConfigPath(), "path to config JSON")
	fs.StringVar(&cfg.ServerURL, "server", "", "Mattermost/Band server URL")
	fs.StringVar(&cfg.Token, "token", "", "Mattermost personal access token/session token")
	fs.StringVar(&cfg.Username, "username", "", "username or email for login/password auth")
	fs.StringVar(&cfg.Password, "password", "", "password for login/password auth")
	fs.StringVar(&cfg.Team, "team", "", "preferred team name or ID")
	fs.StringVar(&cfg.Channel, "channel", "", "preferred channel name or ID")
	fs.BoolVar(&cfg.Mock, "mock", false, "run against built-in mock data")

	cmd := "tui"
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		parseArgs = args[1:]
	}

	if err := fs.Parse(parseArgs); err != nil {
		return Options{}, err
	}
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}

	fileCfg, err := LoadFile(cfg.Config)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Options{}, err
	}
	merged := merge(fileCfg, envConfig(), cfg)
	if merged.ServerURL == "" {
		merged.ServerURL = "https://band.wb.ru"
	}
	merged.ServerURL = strings.TrimRight(merged.ServerURL, "/")
	merged.Config = cfg.Config
	merged.Mock = cfg.Mock || strings.EqualFold(os.Getenv("BAND_MOCK"), "1") || strings.EqualFold(os.Getenv("BAND_MOCK"), "true")

	return Options{Command: cmd, Config: merged}, nil
}

func LoadFile(path string) (Config, error) {
	var cfg Config
	if path == "" {
		return cfg, os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func SaveFile(path string, cfg Config) error {
	if path == "" {
		path = defaultConfigPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	cfg.Config = ""
	cfg.Mock = false
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func HasCredentials(cfg Config) bool {
	return cfg.Token != "" || (cfg.Username != "" && cfg.Password != "")
}

func defaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "band-tui", "config.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "band-tui", "config.json")
	}
	return "config.json"
}

func envConfig() Config {
	return Config{
		ServerURL: getenvAny("BAND_URL", "BAND_SERVER", "MATTERMOST_URL"),
		Token:     getenvAny("BAND_TOKEN", "MATTERMOST_TOKEN"),
		Username:  getenvAny("BAND_USERNAME", "MATTERMOST_USERNAME"),
		Password:  getenvAny("BAND_PASSWORD", "MATTERMOST_PASSWORD"),
		Team:      getenvAny("BAND_TEAM", "MATTERMOST_TEAM"),
		Channel:   getenvAny("BAND_CHANNEL", "MATTERMOST_CHANNEL"),
	}
}

func getenvAny(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func merge(configs ...Config) Config {
	var out Config
	for _, cfg := range configs {
		if cfg.ServerURL != "" {
			out.ServerURL = cfg.ServerURL
		}
		if cfg.Token != "" {
			out.Token = cfg.Token
		}
		if cfg.Username != "" {
			out.Username = cfg.Username
		}
		if cfg.Password != "" {
			out.Password = cfg.Password
		}
		if cfg.Team != "" {
			out.Team = cfg.Team
		}
		if cfg.Channel != "" {
			out.Channel = cfg.Channel
		}
		if len(cfg.FavoriteChannels) > 0 {
			out.FavoriteChannels = append([]string(nil), cfg.FavoriteChannels...)
		}
	}
	return out
}
