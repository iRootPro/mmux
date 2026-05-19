package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMergesFileEnvAndCLI(t *testing.T) {
	for _, key := range []string{"BAND_URL", "BAND_TOKEN", "BAND_USERNAME", "BAND_PASSWORD", "BAND_TEAM", "BAND_CHANNEL", "BAND_LANG", "BAND_MOCK", "MATTERMOST_URL", "MATTERMOST_TOKEN"} {
		t.Setenv(key, "")
	}
	t.Setenv("BAND_TOKEN", "env-token")
	t.Setenv("BAND_TEAM", "env-team")
	t.Setenv("BAND_LANG", "ru")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"server_url":"https://file.example","token":"file-token","channel":"file-channel","favorite_channels":["c1","d2"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := Parse([]string{"doctor", "--config", path, "--server", "https://cli.example/", "--channel", "cli-channel"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Command != "doctor" {
		t.Fatalf("command = %q", opts.Command)
	}
	if opts.Config.ServerURL != "https://cli.example" {
		t.Fatalf("server = %q", opts.Config.ServerURL)
	}
	if opts.Config.Token != "env-token" {
		t.Fatalf("token = %q", opts.Config.Token)
	}
	if opts.Config.Team != "env-team" {
		t.Fatalf("team = %q", opts.Config.Team)
	}
	if opts.Config.Channel != "cli-channel" {
		t.Fatalf("channel = %q", opts.Config.Channel)
	}
	if opts.Config.Language != "ru" {
		t.Fatalf("language = %q", opts.Config.Language)
	}
	if len(opts.Config.FavoriteChannels) != 2 || opts.Config.FavoriteChannels[0] != "c1" || opts.Config.FavoriteChannels[1] != "d2" {
		t.Fatalf("favorites = %#v", opts.Config.FavoriteChannels)
	}
}

func TestHasCredentials(t *testing.T) {
	if !HasCredentials(Config{Token: "token"}) {
		t.Fatal("token should be credentials")
	}
	if !HasCredentials(Config{Username: "u", Password: "p"}) {
		t.Fatal("username/password should be credentials")
	}
	if HasCredentials(Config{Username: "u"}) {
		t.Fatal("partial credentials should not count")
	}
}
