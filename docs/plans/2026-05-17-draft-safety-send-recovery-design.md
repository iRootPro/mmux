# Draft Safety & Send Recovery Design

## Goal

Make `band-tui` safe to use as a daily driver while composing messages: text must survive channel/thread switching and send failures, and the UI must make the active destination explicit.

## Problem

The app currently has one shared `textarea.Model` used by the main channel composer and the thread reply composer. Sending resets the composer before the backend confirms success. Switching channels or opening/closing threads changes the logical destination while the same composer buffer is still in memory. This is compact, but it has two trust problems:

- a failed send leaves the user with an empty composer and only a status message;
- text typed for one destination can visually follow the user into another channel/thread unless the user manually manages it.

For a terminal chat client, losing a long message is more damaging than missing a convenience feature. Draft safety is therefore the next reliability foundation.

## Scope

Implement session-local drafts only. Do not add persistence to disk yet, do not sync drafts to Mattermost, and do not introduce another backend pipeline.

In scope:

- per-channel drafts;
- per-thread drafts;
- automatic save before destination changes;
- automatic restore after destination changes;
- successful send clears only the sent destination draft;
- failed send restores the attempted text into the correct draft/composer;
- visible status for restored failed sends.

Out of scope for this cycle:

- durable drafts across process restart;
- draft conflict resolution across devices;
- rich multi-composer UI;
- autosave telemetry;
- attachment drafts.

## User Model

A draft belongs to a destination, not to a pane.

Destination keys:

- channel message: `channel:<channelID>`
- thread reply: `thread:<channelID>:<rootID>`

The single shared composer remains. The app saves the current composer value into the old destination before switching, then loads the draft for the new destination into the same textarea.

Examples:

1. User types in `#dev`, switches to `#alerts`, then returns to `#dev`: the `#dev` text is restored.
2. User types a thread reply, closes the thread, writes a channel message, then reopens the thread: the reply draft is restored.
3. User sends a message and the backend fails: the attempted text is restored in that destination and the composer focus remains active.
4. User sends successfully: only that destination's draft is cleared.

## Architecture

Add draft state to `Model`:

```go
drafts            map[string]string
activeDraftKey    string
pendingSends      map[string]string
```

`drafts` stores non-empty local drafts. `activeDraftKey` records which destination is currently loaded into `m.composer`. `pendingSends` stores text that has been optimistically removed from the composer while a backend send is in flight.

Add helpers:

```go
func channelDraftKey(channelID string) string
func threadDraftKey(channelID, rootID string) string
func (m Model) currentDraftKey() string
func (m *Model) saveActiveDraft()
func (m *Model) loadDraft(key string)
func (m *Model) switchDraft(key string)
func (m *Model) clearDraft(key string)
func (m *Model) beginPendingSend(key, text string)
func (m *Model) completePendingSend(key string)
func (m *Model) restorePendingSend(key string)
```

All destination-changing paths call `saveActiveDraft()` before mutating channel/thread state, then `loadDraft()` after the new destination is known. Send paths call `beginPendingSend()` before dispatching backend commands and success/error handlers call `completePendingSend()` or `restorePendingSend()`.

## Integration Points

Primary destination changes:

- `New()` initializes the draft maps.
- `channelsLoadedMsg` / first channel selection loads the initial channel draft.
- `selectChannel()` or `openCurrentChannel()` saves the old draft before changing destination.
- `openSelectedThread()` saves the channel draft and loads `thread:<channel>:<root>`.
- thread `esc` saves the thread draft and restores the current channel draft.
- pending triage thread open paths should use the same thread draft key once the thread root is known.
- `switchTeam()` saves current draft, then clears active draft key and composer because scope/channel identity changes.

Send integration:

- channel send captures `key := currentDraftKey()` and `text := strings.TrimSpace(m.composer.Value())`.
- before `sendPostCmd`, call `beginPendingSend(key, text)` and reset composer.
- `postSentMsg` needs the attempted text or draft key to restore on failure even if the user has switched channels before the response arrives.
- same for `replySentMsg`.

Recommended command message shape:

```go
type postSentMsg struct {
    channelID string
    draftKey  string
    text      string
    post      domain.Post
    err       error
}

type replySentMsg struct {
    channelID string
    rootID    string
    draftKey  string
    text      string
    post      domain.Post
    err       error
}
```

## Error Handling

On send failure:

- restore attempted text into `drafts[draftKey]`;
- if `draftKey == activeDraftKey`, set `m.composer` to that text immediately;
- set status to `send failed · draft restored` or `reply failed · draft restored`;
- keep focus in the relevant composer if the user is still in that destination;
- do not duplicate the text if the user already typed more after switching away.

If a stale send response arrives for a destination that no longer exists, keep the draft in `drafts`; the user can recover it by returning to the destination during the session.

## Testing Strategy

Use TDD. Tests should exercise behavior through `Model` methods, not helper implementation details only.

Core tests:

- channel draft survives channel switch;
- thread draft survives closing/reopening thread;
- channel and thread drafts are isolated;
- successful channel send clears only that channel draft;
- failed channel send restores text;
- failed reply send restores thread draft;
- out-of-order send failure restores the draft for the original destination without overwriting current composer text;
- team/scope switch does not leak the previous destination text into the new scope.

Existing tests around composer focus and thread shared composer must remain green.

## UX Copy

Status messages:

- `draft saved` is too noisy; do not show on every switch.
- `send failed · draft restored`
- `reply failed · draft restored`
- `sent`
- `reply sent`

Optional later polish: show a subtle `draft` marker in composer label when the current destination has non-empty draft text. This is not required for the first cut.
