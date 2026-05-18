# band-tui

A light, minimal Bubble Tea TUI for `band.wb.ru` / Mattermost-compatible chat.

Current MVP:

- Mattermost REST auth with token or username/password
- `doctor` command to verify API access
- teams/channels/DM loading
- message history loading with per-channel cache
- compact Mattermost markdown rendering: links, inline code, code fences, quotes, headings, mentions
- sending messages
- websocket live updates with reconnect
- unread and mention counters when provided by Mattermost
- direct messages sorted by most recent activity
- mock mode for UI work without credentials

## Run

```bash
go run ./cmd/band-tui --mock
```

With Band/Mattermost credentials:

```bash
export BAND_URL=https://band.wb.ru
export BAND_TOKEN=your_token

go run ./cmd/band-tui doctor
go run ./cmd/band-tui
```

Band currently uses browser OAuth/SSO. To save a browser session token:

```bash
go run ./cmd/band-tui auth
```

The helper opens Employee/Guest login, asks you to paste the `MMAUTHTOKEN` cookie
(or the whole Cookie header), validates it, and saves it to the config file.

Username/password auth is also supported if the server allows it:

```bash
export BAND_USERNAME=you@example.com
export BAND_PASSWORD=...
go run ./cmd/band-tui doctor
```

## Config

Default config path:

```text
~/.config/band-tui/config.json
```

Example:

```json
{
  "server_url": "https://band.wb.ru",
  "token": "...",
  "team": "my-team",
  "channel": "town-square"
}
```

Environment variables override config file values; CLI flags override both.

Supported env vars:

- `BAND_URL`
- `BAND_TOKEN`
- `BAND_USERNAME`
- `BAND_PASSWORD`
- `BAND_TEAM`
- `BAND_CHANNEL`
- `BAND_MOCK=1`

## Auth notes

From the public Band config:

- email/username sign-in is disabled
- Employee Login uses `/oauth/gitlab/login` → Wildberries Keycloak
- Guest Login uses `/oauth/wb/login` → wildberries.ru OAuth
- the API itself is standard Mattermost `/api/v4`

So the recommended CLI flow is either a Mattermost personal/session token via
`BAND_TOKEN`, or `band-tui auth` to save the browser `MMAUTHTOKEN` session token.

## Keys

- `tab` / `shift+tab` — switch focus
- `ctrl+p` / `ctrl+k` — quick switcher
- `/` — filter channels
- `j/k` or arrows — navigate sidebar / timeline
- `a` — open mentions inbox (`@you`, `@all`, `@channel`, `@here`)
- `n` / `N` — next / previous unread or mention
- `u` — open triage inbox for mentions, unread channels, and thread replies (`enter` open, `d` locally dismiss, `n/N` move inside the overlay, `esc` close)
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
- `ctrl+j` — newline in composer
- `[` or `ctrl+o` — load older messages
- timeline focus: `j/k` select message, `y` copy text, `o` open first link, `t` open thread
- `ctrl+r` — reload current channel
- `?` — help
- `q` outside composer / `ctrl+c` — quit

## Development

```bash
go test ./...
go build ./cmd/band-tui
```

If no credentials are provided, the TUI falls back to mock mode so design and interaction work can continue offline.
