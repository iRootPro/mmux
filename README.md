# mmux

A keyboard-first Bubble Tea TUI for Mattermost-compatible chat.

Current MVP:

- Mattermost REST auth with token or username/password
- `doctor` command to verify API access
- teams/channels/DM loading
- message history loading with in-memory and disk cache for faster startup/scope switching
- compact Mattermost markdown rendering: links, inline code, code fences, quotes, headings, mentions
- file/image attachments rendered inline with name, type, size and dimensions when available
- sending messages
- websocket live updates with reconnect
- unread and mention counters when provided by Mattermost
- direct messages sorted by most recent activity
- mock mode for UI work without credentials

## Install

### Install script

Linux/macOS one-liner:

```bash
curl -fsSL https://raw.githubusercontent.com/iRootPro/mmux/main/scripts/install.sh | sh
```

Optional overrides:

```bash
# install a specific version
MMUX_VERSION=v0.1.2 curl -fsSL https://raw.githubusercontent.com/iRootPro/mmux/main/scripts/install.sh | sh

# install without sudo into a user-local bin dir
mkdir -p ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/iRootPro/mmux/main/scripts/install.sh | MMUX_INSTALL_DIR=$HOME/.local/bin sh
```

Then run:

```bash
mmux --help
```

### macOS / Linux with Homebrew

```bash
brew install irootpro/tap/mmux
```

### Linux from GitHub Releases

Download the archive for your architecture from the latest release:

```bash
# x86_64 / amd64
curl -L -o mmux.tar.gz \
  https://github.com/iRootPro/mmux/releases/download/v0.1.2/mmux_v0.1.2_linux_amd64.tar.gz

# arm64 / aarch64
# curl -L -o mmux.tar.gz \
#   https://github.com/iRootPro/mmux/releases/download/v0.1.2/mmux_v0.1.2_linux_arm64.tar.gz

tar -xzf mmux.tar.gz
sudo install -m 0755 mmux_v0.1.2_linux_amd64/mmux /usr/local/bin/mmux
```

Then run:

```bash
mmux --help
```

## Run

```bash
mmux
```

On first launch, if no server is configured, mmux opens onboarding settings.
Enter your Mattermost server URL and token, save, then restart.

For offline UI/demo mode:

```bash
mmux --mock
```

With Mattermost credentials:

```bash
export MMUX_URL=https://mattermost.example.com
export MMUX_TOKEN=your_token

mmux doctor
mmux
```

You can also open settings inside the TUI with `alt+,` and save the server URL/token there.
Connection changes are applied after restarting `mmux`.

To save a browser session token:

```bash
mmux auth
```

The helper asks you to paste the `MMAUTHTOKEN` cookie (or the whole Cookie
header), validates it, and saves it to the config file.

Username/password auth is also supported if the server allows it:

```bash
export MMUX_USERNAME=you@example.com
export MMUX_PASSWORD=...
mmux doctor
```

## Config

Default config path:

```text
~/.config/mmux/config.json
```

Example:

```json
{
  "server_url": "https://mattermost.example.com",
  "token": "...",
  "team": "my-team",
  "channel": "town-square",
  "language": "ru"
}
```

Environment variables override config file values; CLI flags override both.

Supported env vars:

- `MMUX_URL`
- `MMUX_TOKEN`
- `MMUX_USERNAME`
- `MMUX_PASSWORD`
- `MMUX_TEAM`
- `MMUX_CHANNEL`
- `MMUX_LANG=ru` / `MMUX_LANGUAGE=ru` for Russian UI (`en` is the default)
- `MMUX_MOCK=1`

## Auth notes

The API is standard Mattermost `/api/v4`.

Recommended CLI flow:

- use a Mattermost personal/session token via `MMUX_TOKEN`; or
- run `mmux auth` to save the browser `MMAUTHTOKEN` session token.

## Keys

- `tab` / `shift+tab` — switch focus
- `alt+1` / `alt+2` / `alt+3` / `alt+4` — jump directly to sidebar / timeline / composer / thread
- `ctrl+p` — quick switcher plus go-to commands
- `ctrl+h` / `ctrl+j` / `ctrl+k` / `ctrl+l` — tmux/vim-style pane navigation: sidebar / composer / timeline / thread
- `ctrl+b` / `alt+s` — focus the left sidebar from anywhere
- `alt+,` — open settings from anywhere; `,` opens settings when not typing
- `/` — filter channels
- `j/k` or arrows — navigate sidebar / timeline
- `a` — open mentions inbox (`@you`, `@all`, `@channel`, `@here`)
- `n` / `N` — next / previous unread or mention
- `ctrl+u` / `alt+u` — open triage inbox from anywhere, including while typing; `u` still opens it when not typing (`enter` open, `d` locally dismiss, `n/N` move inside the overlay, `esc` close)
- `i` — open channel/person info (when focus is not in composer)
- `F2` / `ctrl+g` — switch scope/team/workspace
- `w` / `T` — switch scope when not typing
- unread/mentioned messages are marked with `●`; threads with unread replies show `● new replies`
- consecutive messages from the same author are visually grouped; unread/mentioned messages keep their own marker/header
- sidebar rows reserve the right edge for mention/unread counts, with mentions shown as `@N`
- status bar changes hints based on active focus: sidebar, timeline, composer, or thread
- DM/group presence is shown in sidebar: `●` online, `◐` away, `○` offline
- `f` — toggle favorite in sidebar
- `s/c/d/g` or `0/1/2/3` — jump to favorites/channels/direct/groups in sidebar
- `pgup/pgdown` — jump between sidebar sections
- `left/right` — collapse/expand current sidebar section
- `space` or `x` — toggle current sidebar section
- `enter` — send message in composer
- `alt+enter` — newline in composer
- `[` or `ctrl+o` — load older messages
- timeline focus: `j/k` select message, `y` copy text, `p` copy permalink, `o` open first link, `r` quote into composer, `e` edit own message, `D` delete own message (press twice), `R` open searchable reaction picker (type to filter/custom emoji, arrows move), `t` open thread
- thread layout: `alt+2` timeline, `alt+3` reply composer, `alt+4` thread messages; `esc` from reply composer returns to messages, then closes thread
- `ctrl+r` — reload current channel or retry connection/scope loading when offline; if auth expired, refresh token and restart
- `?` — help
- `q` outside composer / `ctrl+c` — quit

## Development

```bash
go test ./...
go build ./...
```

Use `mmux --mock` for offline UI/design work without credentials.
