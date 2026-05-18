# Connection & Session Reliability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `band-tui` surface connection/session state honestly and recover predictably from network loss, reconnects, and expired auth.

**Architecture:** Keep websocket retry policy inside the backend, but emit typed connection-state events and typed backend errors so the app can render reliable product state instead of parsing strings. The app stores explicit connection state, preserves current cached content during outages, and makes `ctrl+r` reconnect-aware.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` model/view architecture, `internal/domain` backend interface types, Mattermost REST/websocket client, mock backend, VHS smoke tapes.

---

## Prerequisites

Before implementation:

- Ensure the current working tree is clean; this plan should start from the committed draft-safety state.
- Read design: `docs/plans/2026-05-17-connection-session-reliability-design.md`.
- Relevant existing code:
  - `internal/domain/domain.go:64-93` (`Event`, `Backend`)
  - `internal/mattermost/client.go:48-92,520-592`
  - `internal/mattermost/websocket.go:17-37,39-111`
  - `internal/mock/backend.go:14-19,50-56,152-173`
  - `internal/app/model.go:152-183,342-413,657-672,1237-1248`
  - `internal/app/view.go:1184-1212`
  - `README.md:94-123`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Do not add token refresh or browser auth into the TUI.
- Keep existing backend interface method list unchanged unless a domain type change is enough.
- Preserve existing cached content on reconnect/offline whenever possible.
- Keep `ctrl+r` as the manual recovery key; do not invent a second reconnect shortcut in this cycle.

---

### Task 1: Add typed backend error and connection state primitives

**Files:**
- Modify: `internal/domain/domain.go:64-93`
- Modify: `internal/mattermost/client.go:520-592`
- Test: modify `internal/mattermost/client_test.go`
- Optional create: `internal/mattermost/errors_test.go`

**Step 1: Write the failing tests**

Add focused tests in `internal/mattermost/client_test.go` or a new `errors_test.go`:

```go
func TestBackendErrorKindFromHTTPStatus(t *testing.T) {
    err := wrapHTTPError("get current user", 401, "unauthorized")
    var be *domain.BackendError
    if !errors.As(err, &be) {
        t.Fatalf("expected BackendError, got %T", err)
    }
    if be.Kind != domain.BackendErrorAuth || !be.Retryable {
        t.Fatalf("unexpected backend error: %#v", be)
    }
}

func TestBackendErrorKindFromServerFailure(t *testing.T) {
    err := wrapHTTPError("get posts", 503, "unavailable")
    var be *domain.BackendError
    if !errors.As(err, &be) {
        t.Fatalf("expected BackendError, got %T", err)
    }
    if be.Kind != domain.BackendErrorServer || !be.Retryable {
        t.Fatalf("unexpected backend error: %#v", be)
    }
}
```

If you prefer not to expose `wrapHTTPError`, write tests against `client.do()` with `httptest` handlers returning 401 and 503, then assert `errors.As` against `*domain.BackendError`.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/mattermost -run 'TestBackendErrorKind' -count=1
```

Expected: FAIL because typed backend errors do not exist.

**Step 3: Implement minimal domain types**

In `internal/domain/domain.go`, add:

```go
type ConnectionState string

const (
    ConnectionConnecting   ConnectionState = "connecting"
    ConnectionConnected    ConnectionState = "connected"
    ConnectionReconnecting ConnectionState = "reconnecting"
    ConnectionOffline      ConnectionState = "offline"
    ConnectionAuthExpired  ConnectionState = "auth_expired"
)

type BackendErrorKind string

const (
    BackendErrorAuth    BackendErrorKind = "auth"
    BackendErrorNetwork BackendErrorKind = "network"
    BackendErrorServer  BackendErrorKind = "server"
    BackendErrorUnknown BackendErrorKind = "unknown"
)

type BackendError struct {
    Op         string
    Kind       BackendErrorKind
    StatusCode int
    Retryable  bool
    Err        error
}

func (e *BackendError) Error() string
func (e *BackendError) Unwrap() error
```

Extend `Event` with:

```go
State   ConnectionState
Attempt int
RetryIn time.Duration
Message string
```

Add `time` import.

In `internal/mattermost/client.go`, implement small helpers:

```go
func wrapHTTPError(op string, status int, message string) error
func wrapRequestError(op string, err error) error
```

Rules:

- 401/403 => `BackendErrorAuth`
- 5xx => `BackendErrorServer`
- request transport failures / context deadline / dial failures => `BackendErrorNetwork`
- everything else => `BackendErrorUnknown`

Use these helpers in `login()` and `do()` instead of plain `fmt.Errorf` for classified failures.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/mattermost -run 'TestBackendErrorKind' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/domain.go internal/mattermost/client.go internal/mattermost/client_test.go internal/mattermost/errors_test.go
git commit -m "feat: add typed backend reliability errors"
```

---

### Task 2: Emit typed watch state events from Mattermost and mock backends

**Files:**
- Modify: `internal/mattermost/websocket.go:17-37,39-111`
- Modify: `internal/mock/backend.go:14-19,152-173`
- Test: modify `internal/mattermost/websocket_test.go`
- Test: modify `internal/mock/backend_test.go`

**Step 1: Write the failing tests**

In `internal/mock/backend_test.go`, add deterministic scenario tests:

```go
func TestMockControlMessageEmitsOfflineState(t *testing.T) {
    b := New()
    events := make(chan domain.Event, 4)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go b.WatchPosts(ctx, events)

    _, err := b.SendPost(context.Background(), "dev", "mock:offline")
    if err != nil {
        t.Fatal(err)
    }
    ev := <-events
    if ev.Kind != domain.EventState || ev.State != domain.ConnectionOffline {
        t.Fatalf("unexpected event: %#v", ev)
    }
}
```

In `internal/mattermost/websocket_test.go`, add a pure helper test if needed:

```go
func TestWatchFailureStateFromBackendError(t *testing.T) {
    state, retryable := watchFailureState(&domain.BackendError{Kind: domain.BackendErrorAuth, Retryable: false})
    if state != domain.ConnectionAuthExpired || retryable {
        t.Fatalf("unexpected auth state: %q retryable=%v", state, retryable)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/mattermost -run TestWatchFailureState -count=1
go test ./internal/mock -run TestMockControlMessageEmitsOfflineState -count=1
```

Expected: FAIL because watch state events are not emitted and mock control messages do not exist.

**Step 3: Implement watch state emission**

In `internal/mattermost/websocket.go`:

- emit `EventState{State: domain.ConnectionConnecting}` before the first dial;
- emit `EventState{State: domain.ConnectionConnected}` after websocket auth is established;
- on retryable failure, emit `EventState{State: domain.ConnectionReconnecting, Attempt: n, RetryIn: backoff, Err: err, Message: err.Error()}`;
- on non-retryable auth failure, emit `EventState{State: domain.ConnectionAuthExpired, Err: err, Message: "refresh token and restart"}` and stop retrying.

Add a small helper, for example:

```go
func watchFailureState(err error) (domain.ConnectionState, bool)
```

In `internal/mock/backend.go`:

- add an internal watcher channel list or broadcast helper;
- keep existing ticker post behavior;
- interpret deterministic control messages in `sendPost`:
  - `mock:offline`
  - `mock:reconnect`
  - `mock:auth-expired`
- broadcast `EventState` accordingly instead of appending a normal post.

Do not add random failures.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/mattermost -run TestWatchFailureState -count=1
go test ./internal/mock -run 'TestMockControlMessageEmitsOfflineState|TestSendFailureTrigger' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/mattermost/websocket.go internal/mattermost/websocket_test.go internal/mock/backend.go internal/mock/backend_test.go
git commit -m "feat: emit connection state events"
```

---

### Task 3: Track connection state in the app model and render it

**Files:**
- Modify: `internal/app/model.go:36-95,168-183,342-413`
- Modify: `internal/app/view.go:1184-1212`
- Test: create `internal/app/connection_test.go`

**Step 1: Write the failing tests**

Create `internal/app/connection_test.go`:

```go
package app

import (
    "strings"
    "testing"
    "time"

    "band-tui/internal/domain"
)

func TestBackendEventStateReconnectingUpdatesConnectionState(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.status = "42 messages"

    updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventState, State: domain.ConnectionReconnecting, Attempt: 2, RetryIn: 5 * time.Second}})
    got := updated.(Model)
    if got.connectionState != domain.ConnectionReconnecting || got.connectionAttempt != 2 {
        t.Fatalf("unexpected connection state: %#v", got)
    }
}

func TestRenderStatusShowsAuthExpiredAction(t *testing.T) {
    m := Model{
        connectionState: domain.ConnectionAuthExpired,
        connectionMessage: "refresh token and restart",
        status: "42 messages",
        width: 120,
    }
    got := m.renderStatus(120)
    if !strings.Contains(got, "auth expired") || !strings.Contains(got, "refresh token") {
        t.Fatalf("status = %q", got)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestBackendEventStateReconnectingUpdatesConnectionState|TestRenderStatusShowsAuthExpiredAction' -count=1
```

Expected: FAIL because connection state fields/rendering do not exist.

**Step 3: Implement explicit model state**

Add fields to `Model`:

```go
connectionState   domain.ConnectionState
connectionAttempt int
connectionRetryIn time.Duration
connectionMessage string
```

Initialize startup state in `New()` or `Init()` as `domain.ConnectionConnecting`.

Add helpers in `model.go` or `connection.go`:

```go
func (m *Model) setConnectionState(state domain.ConnectionState, attempt int, retryIn time.Duration, message string, err error)
func connectionMessage(state domain.ConnectionState, attempt int, retryIn time.Duration, fallback string) string
```

Update flows:

- `sessionLoadedMsg` success => `ConnectionConnected`
- `sessionLoadedMsg` error => classify from `BackendError` into `ConnectionOffline` or `ConnectionAuthExpired`
- `backendEventMsg` handles `EventState` explicitly before/alongside posts/status/error
- `EventError` should become a legacy fallback, not the primary reconnect mechanism

In `view.go`, make `renderStatus()` prepend a `net:` segment derived from explicit connection state rather than `strings.Contains(status, "connected")` heuristics.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestBackendEventStateReconnectingUpdatesConnectionState|TestRenderStatusShowsAuthExpiredAction' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/connection_test.go
git commit -m "feat: render explicit connection state"
```

---

### Task 4: Make manual recovery and load failures reconnect-aware

**Files:**
- Modify: `internal/app/model.go:168-240,657-672,1237-1248`
- Test: modify `internal/app/connection_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestCtrlRReconnectsWhenOffline(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.connectionState = domain.ConnectionOffline
    m.focus = focusSidebar

    updated, cmd := m.handleKey(draftKey("ctrl+r"))
    got := updated.(Model)
    if got.connectionState != domain.ConnectionConnecting {
        t.Fatalf("ctrl+r should move offline state back to connecting, got %q", got.connectionState)
    }
    if cmd == nil {
        t.Fatal("expected reconnect command")
    }
}

func TestCtrlRStillReloadsCurrentChannelWhenConnected(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.connectionState = domain.ConnectionConnected
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.focus = focusSidebar

    _, cmd := m.handleKey(draftKey("ctrl+r"))
    if cmd == nil {
        t.Fatal("expected reload command")
    }
}

func TestAuthExpiredDoesNotPretendReconnectSucceeded(t *testing.T) {
    m := New(nil, testConfig(), false)
    updated, _ := m.Update(sessionLoadedMsg{err: &domain.BackendError{Kind: domain.BackendErrorAuth, Retryable: false}})
    got := updated.(Model)
    if got.connectionState != domain.ConnectionAuthExpired {
        t.Fatalf("expected auth expired state, got %q", got.connectionState)
    }
}
```

If `draftKey("ctrl+r")` is awkward, add a local key helper for control keys in `connection_test.go`.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestCtrlRReconnectsWhenOffline|TestCtrlRStillReloadsCurrentChannelWhenConnected|TestAuthExpiredDoesNotPretendReconnectSucceeded' -count=1
```

Expected: FAIL until `ctrl+r` is reconnect-aware and connect errors classify cleanly.

**Step 3: Implement reconnect-aware reload**

In `handleKey` `ctrl+r` branch:

- if connection state is `offline` or `connecting` before a healthy session exists, retry `connectCmd` or `loadCurrentScopeCmd` as appropriate;
- if `auth_expired`, keep actionable messaging (`refresh token and restart`) and do not silently claim success;
- if connected, keep current channel reload behavior.

Also update `channelsLoadedMsg` / `postsLoadedMsg` / `threadLoadedMsg` failures:

- classify typed backend errors;
- prefer `offline` / `auth expired` product messages over generic `could not load ...` strings when the failure is connection-related;
- keep already-rendered cached content whenever possible.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestCtrlRReconnectsWhenOffline|TestCtrlRStillReloadsCurrentChannelWhenConnected|TestAuthExpiredDoesNotPretendReconnectSucceeded|TestBackendEventStateReconnectingUpdatesConnectionState' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/connection_test.go
git commit -m "feat: make reconnect flow explicit"
```

---

### Task 5: Add deterministic mock QA and VHS smoke

**Files:**
- Modify: `README.md:94-123`
- Create: `vhs/connection-reliability.tape`
- Optional modify: `internal/app/connection_test.go`

**Step 1: Add any last failing docs/render test you need**

If useful, add:

```go
func TestHelpTextMentionsReconnectBehavior(t *testing.T) {
    m := Model{}
    got := m.helpText()
    if !strings.Contains(got, "ctrl+r") || !strings.Contains(got, "reconnect") {
        t.Fatalf("help text missing reconnect hint: %q", got)
    }
}
```

**Step 2: Run the failing test**

Run:

```bash
go test ./internal/app -run TestHelpTextMentionsReconnectBehavior -count=1
```

Expected: FAIL until docs/help are updated.

**Step 3: Update docs and add VHS tape**

Update `README.md` (and `helpText()` if tested) to document:

- `ctrl+r` reloads when healthy and retries connection when offline;
- expired auth requires refreshing token and restarting;
- mock QA control messages:
  - `mock:offline`
  - `mock:reconnect`
  - `mock:auth-expired`

Create `vhs/connection-reliability.tape`:

```text
# band-tui connection reliability smoke
# Run from repo root with: vhs vhs/connection-reliability.tape

Output tmp/connection-reliability.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

Type "mock:offline"
Enter
Sleep 1s

Type "mock:reconnect"
Enter
Sleep 1s

Type "mock:auth-expired"
Enter
Sleep 1s

Ctrl+C
```

Adjust focus keystrokes only if needed; the final GIF must visibly show offline/reconnecting, connected, and auth-expired states.

**Step 4: Run targeted tests and VHS**

Run:

```bash
go test ./internal/app -run 'TestHelpTextMentionsReconnectBehavior|TestBackendEventStateReconnectingUpdatesConnectionState|TestCtrlRReconnectsWhenOffline' -count=1
vhs vhs/connection-reliability.tape
```

Expected:

- tests PASS;
- `tmp/connection-reliability.gif` created.

**Step 5: Commit**

```bash
git add README.md internal/app/view.go vhs/connection-reliability.tape
git commit -m "docs: add connection reliability workflow"
```

---

### Task 6: Final verification

**Files:**
- No new files required.

**Step 1: Run focused connection tests**

```bash
go test ./internal/app -run 'TestBackendEventState|TestCtrlR|TestHelpTextMentionsReconnectBehavior|TestRenderStatusShowsAuthExpiredAction' -count=1
```

Expected: PASS.

**Step 2: Run app package tests**

```bash
go test ./internal/app -count=1
```

Expected: PASS.

**Step 3: Run full repository tests**

```bash
go test ./... -count=1
```

Expected: PASS.

**Step 4: Build**

```bash
go build ./cmd/band-tui
```

Expected: success with no output.

**Step 5: Run VHS smoke**

```bash
vhs vhs/connection-reliability.tape
```

Expected: `tmp/connection-reliability.gif` exists and visually confirms:

- offline/reconnecting state is visible;
- connected recovery is visible;
- auth-expired state is visible and actionable.

**Step 6: Review final diff**

```bash
git diff --stat HEAD
```

Expected: only connection/session reliability source/tests/docs/VHS changes.

---

## Notes for implementer

- Keep reconnect policy in the backend; do not build a second retry loop in the app.
- Do not parse error strings in the app when a typed `BackendError` is available.
- Auth expiry is terminal for the current process in this cycle. Be explicit; do not promise silent recovery.
- Prefer stale-but-readable UI over clearing current content during outages.
- Keep mock triggers deterministic and human-readable so VHS remains stable.
