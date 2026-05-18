# Connection & Session Reliability Design

## Goal

Make `band-tui` safe to leave open all day by making connectivity and session state explicit, predictable, and recoverable.

## Problem

The app already reconnects its websocket internally, but the product contract is still weak:

- the UI mostly sees plain string errors (`connection failed`, `reconnecting…`), not typed connection state;
- auth expiry, network loss, and transient websocket reconnect all collapse into vague status text;
- the backend retries silently, so the user cannot tell whether the client is healthy, reconnecting, or dead;
- initial connect/load failures and live watch failures follow different paths and produce inconsistent UX;
- mock mode cannot reliably demonstrate reconnect or expired-session behavior.

That is good enough for a demo, but not for a daily-driver terminal client.

## Desired User Experience

The user should always be able to answer three questions immediately:

1. **Am I connected right now?**
2. **If not, is the client retrying automatically or waiting for me?**
3. **Is this a network problem or an expired/invalid session?**

Product behavior:

- transient websocket/network failures surface as `reconnecting` with retry information;
- once the watch is healthy again, the UI returns to `connected` without manual intervention;
- expired or invalid auth surfaces as `auth expired` with an explicit next step: refresh token / rerun auth / restart;
- current visible data stays on screen during outages whenever possible; the client should degrade to stale-but-readable, not blank-and-panicked;
- `ctrl+r` remains the manual recovery key, but becomes reconnect-aware instead of only reloading the current channel.

## Approaches Considered

### 1. UI-only string parsing

Keep current backend behavior and infer state in `app.Model` by parsing error strings.

Why not: brittle, backend-specific, impossible to test well, and guaranteed to rot once error text changes.

### 2. Typed reliability signals from the backend layer — recommended

Introduce typed backend error classification plus typed watch-state events. Keep reconnect policy inside the backend, but surface state transitions to the app.

Why this is right:

- the app renders state instead of guessing;
- websocket and REST failures can be classified consistently;
- mock backend can emit the same states deterministically;
- the change is load-bearing but still incremental: no second event pipeline, no token refresh subsystem, no background daemon.

### 3. Full connection/session manager in the app

Move reconnect logic, retry policy, and backend reconstruction into `app.Model`.

Why not now: too invasive. The backend already owns websocket connect/retry mechanics; ripping that out now would create a larger refactor than the product problem requires.

## Chosen Architecture

Add two typed reliability primitives in `internal/domain`:

1. **Connection state** for watch/session lifecycle:
   - `connecting`
   - `connected`
   - `reconnecting`
   - `offline`
   - `auth_expired`

2. **Backend error kind** for REST/websocket failures:
   - `auth`
   - `network`
   - `server`
   - `unknown`

The backend remains responsible for dialing, retrying, and classifying low-level failures. The app remains responsible for rendering product state and deciding what the user can do next.

### Domain shape

Extend `domain.Event` so state transitions are first-class, not stringly typed:

```go
type ConnectionState string

type BackendErrorKind string

type BackendError struct {
    Op         string
    Kind       BackendErrorKind
    StatusCode int
    Retryable  bool
    Err        error
}

type Event struct {
    Kind      string
    Post      Post
    UserID    string
    Status    string
    Err       error
    State     ConnectionState
    Attempt   int
    RetryIn   time.Duration
    Message   string
}
```

`EventState` already exists; this cycle makes it real.

## Backend Responsibilities

### Mattermost backend

`client.do()` and `login()` should stop returning anonymous `fmt.Errorf(...)` chains for interesting failures. They should wrap failures in `domain.BackendError` so the app can distinguish:

- HTTP 401/403 → `auth`
- network timeout / DNS / dial / websocket close during connect → `network`
- HTTP 5xx → `server`
- everything else → `unknown`

`WatchPosts()` keeps its retry loop, but emits `EventState` transitions:

- `connecting` before first dial;
- `connected` after websocket auth succeeds;
- `reconnecting` on retryable failure, with `Attempt` and `RetryIn`;
- `auth_expired` on terminal auth failure, then stops retrying;
- `offline` if the watch loop exits because network recovery is not currently possible or the backend is shut down.

Important constraint: **no in-app token refresh**. Band auth today is manual token/cookie acquisition. If the token expires, the correct UX is to tell the user to refresh credentials and restart, not to pretend silent recovery exists.

### Mock backend

Mock mode must emit the same `EventState` shapes as Mattermost.

Recommended deterministic controls:

- sending `mock:offline` triggers `offline` / `reconnecting` state;
- sending `mock:reconnect` triggers `connected`;
- sending `mock:auth-expired` triggers `auth_expired`;
- existing `fail-send` remains for send recovery.

This gives reliable VHS coverage without adding hidden keybindings.

## App Model Responsibilities

Add explicit connection fields in `Model`, e.g.:

```go
connectionState   domain.ConnectionState
connectionAttempt int
connectionRetryIn time.Duration
connectionMessage string
connectionErr     error
```

The app should treat watch/session reliability as **orthogonal UI state**, not overloaded `status` text.

Behavior:

- startup begins in `connecting`;
- successful `sessionLoadedMsg` moves to `connected` once initial connect succeeds;
- `backendEventMsg` with `EventState` updates connection state without destroying current channel/thread content;
- REST failures (`connect`, `channels`, `posts`, `thread`, `view channel`) map through typed backend errors into product states;
- on auth expiry, preserve current visible data, stop implying live health, and show action text;
- on reconnecting, preserve current content and make the retry visible.

## User-Facing UX

### Status bar

Status should separate **network badge** from **interaction status**.

Examples:

- `scope: WB Band · net: connected · 42 messages`
- `scope: WB Band · net: reconnecting in 4s · 42 messages`
- `scope: WB Band · net: offline · showing cached messages`
- `scope: WB Band · net: auth expired · refresh token and restart`

Current green success coloring for `connected`/`sent` should become more deliberate:

- connected = muted/success badge
- reconnecting/offline = warning/muted
- auth expired = error color

### Manual recovery

Keep `ctrl+r` as the boring recovery key:

- if connected: current behavior, reload current channel;
- if offline/reconnecting before session is healthy: retry connect/load scope;
- if auth expired: keep status actionable, but do not promise success with the same in-memory token.

## Testing Strategy

Use deterministic tests at three layers.

### Mattermost/backend tests

- classify 401/403 as auth;
- classify dial/timeout failures as network;
- websocket retry emits `reconnecting` then `connected`;
- auth failure emits `auth_expired` and stops retrying.

### App tests

- `EventState(reconnecting)` updates status badge without clearing posts;
- `EventState(connected)` returns to healthy badge;
- auth-expired errors from initial connect/load produce actionable state;
- `ctrl+r` retries connect when offline and reloads channel when connected.

### Mock/VHS

- mock trigger → offline/reconnecting visible;
- mock trigger → reconnect visible;
- mock trigger → auth expired visible;
- existing draft-safety and triage flows remain unaffected.

## Non-Goals

Not in this cycle:

- automatic token refresh;
- background process restart;
- multi-profile credential rotation;
- in-app auth browser flow;
- attachment resume or upload retry;
- message dedupe beyond current logic.

## Recommended Next Slice

Implement connection/session reliability in this order:

1. typed backend error + state events;
2. model connection state + status rendering;
3. reconnect-aware `ctrl+r` behavior;
4. deterministic mock/VHS scenarios.

That keeps the scope product-focused: first make state honest, then make recovery explicit, then make it demoable and testable.
