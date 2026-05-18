# Triage Inbox Overlay Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a modal triage inbox overlay that lets the user quickly process all important chat signals — mentions, unread channels, and thread replies — from one keyboard-first queue.

**Architecture:** Keep inbox as a thin UI/workflow layer over existing app state. Do not introduce a new backend API or separate server-synced inbox on the first iteration. Build normalized `triageItem` values from existing `channels`, `recentEvents`, `posts`, `postsByChannel`, and known thread state, then render and navigate that queue through a dedicated overlay plus global next/previous queue actions.

**Tech Stack:** Go, Bubble Tea, Bubbles viewport/textarea, Lip Gloss, existing `Model` state in `internal/app`, existing `domain.Channel` and `domain.Post` types.

---

## Product direction

`band-tui` already has strong keyboard navigation, unread counters, mentions, and thread support. The next daily-use workflow is triage: the user should be able to open one overlay, see every important thing that needs attention, jump into context, mark it handled locally, and move to the next item without hunting through multiple panes.

The chosen UX shape is a **modal/overlay inbox** over the current screen, not a separate permanent mode and not only hidden hotkeys. The overlay is the visible triage center; the global `n/N` navigation remains useful for quickly moving through the queue without reopening the overlay.

## User-facing behavior

### Entry point

Add a dedicated key to open the inbox overlay. Prefer a key that does not conflict with the existing `a` mentions activity workflow. Candidate: `u` for inbox/unread.

Behavior:
- `u` toggles the triage inbox overlay.
- `esc` closes the overlay.
- Opening the overlay does not lose the current channel/thread context.

### Overlay contents

Each row must explain:
- **why** the item is in the queue,
- **where** it lives,
- **how fresh** it is,
- enough preview to decide whether to open it.

Target row shapes:

```text
@ #devops · Артур Зенков · 2m · “Нужен ingress…”
↳ @alisa thread · 5 replies · 8m · “Mock thread reply”
• #alerts · 3 unread · 12m
```

Rules:
- `@` = mention-driven item
- `↳` = thread reply item
- `•` = unread channel item
- selected row gets the existing strong selection treatment used elsewhere
- empty queue state must say `Nothing to triage.`

### Actions

Inside overlay:
- `j/k` or arrows move selection
- `enter` opens the selected item in its natural context
- `d` marks item handled locally for this session
- `n/N` moves next/previous item inside the queue
- `esc` closes overlay

Outside overlay:
- `n/N` may continue to use the same queue ordering for global triage navigation

### Open semantics

`enter` must route by item type:
- mention in a channel → open channel and focus the relevant post
- unread channel → open channel and land on the first important post if present, otherwise use normal channel selection behavior
- thread reply → open thread panel on the relevant root post

The overlay closes after `enter`.

### Done semantics

`d` must be **local dismiss**, not an immediate server-side mark-read operation.

The point is workflow safety:
- do not couple the first version of triage to perfect server read-state semantics
- do not hide items permanently
- do not silently mutate remote state beyond what normal navigation already does

Dismissed items disappear from the overlay for the current session until their underlying signal changes again.

---

## Data model

Add a normalized item type in `internal/app`:

```go
type triageKind int

const (
    triageMention triageKind = iota
    triageThreadReply
    triageUnreadChannel
)

type triageItem struct {
    Kind         triageKind
    ChannelID     string
    RootID        string
    PostID        string
    Title         string
    Actor         string
    Preview       string
    CreateAt      int64
    Score         int
    UnreadCount   int
    MentionCount  int
}
```

Add UI/model state:

```go
triageOpen      bool
triageSelected  int
triageItems     []triageItem
dismissedTriage map[string]struct{}
```

Use a stable string key for dismissals, for example:

```go
func triageDismissKey(item triageItem) string
```

Suggested key shape:

```text
<kind>|<channel>|<root>|<post>|<mentions>|<unread>|<timestamp>
```

The key must change when the meaningful server/local signal changes, so a fresh mention/unread/thread-reply reappears even if the previous version was dismissed.

## Data sources

First version must derive triage items only from current model state:
- `channels[].Mentions`
- `channels[].Unread`
- `recentEvents`
- `posts`
- `postsByChannel`
- known thread state from loaded posts (`ThreadUnread`, `ReplyCount`, `RootID`)

No new backend endpoints in v1.

If a row does not have full preview context, degraded rows are acceptable:

```text
• #alerts · 3 unread
```

The overlay is not a new source of truth; it is a synthesized queue over existing truth.

---

## Queue building and sorting

Build triage items with a deterministic pure function, e.g.:

```go
func buildTriageItems(m Model) []triageItem
```

Ordering for v1:
1. mentions
2. thread replies
3. unread channels
4. within each group, newest first

Additional rules:
- avoid duplicate rows for the same underlying signal
- prefer the richest context available
- if a mention and unread channel point to the same concrete post, keep the mention item and do not add a weaker unread duplicate

Preview text should be sanitized and truncated similarly to other list rows.

---

## Dismissal semantics

Dismissals are session-local only.

When rebuilding the queue:
- items whose dismissal key is present are hidden
- if the underlying signal changes and produces a different key, the item is visible again

Examples:
- channel had `Unread=3`, user dismissed it, then unread becomes `5` → item reappears
- mention item dismissed, then a new mention arrives in the same channel → item reappears
- thread reply item dismissed, then more replies arrive → item reappears

This preserves safety and avoids “lost” important work.

---

## Model transitions

Rebuild the queue whenever these events happen:
- channels loaded/refreshed
- posts loaded/refreshed
- older posts loaded
- websocket post event processed
- thread loaded
- reply sent to thread
- current channel viewed / unread counters changed
- scope/team switched
- triage item dismissed

The rebuild must be cheap and deterministic.

If the overlay is open and the selected index becomes out of range after rebuild, clamp it to the last valid item or zero.

If the queue becomes empty while overlay is open, keep the overlay open and show `Nothing to triage.` rather than closing abruptly.

---

## Open routing

Add a dispatcher such as:

```go
func (m Model) openTriageItem(item triageItem) (tea.Model, tea.Cmd)
```

Routing rules:
- mention item with `PostID` → select/open channel and jump to that post
- thread reply item with `RootID` → open thread panel for the root
- unread channel item → open the channel and rely on the existing important-post selection logic

If the target context is missing from cache, use the existing loading flows and pending jump fields rather than inventing a second navigation mechanism.

The overlay must close before or as part of routing.

---

## Global next/previous navigation

Keep `n/N` useful outside the overlay by aligning them with the inbox queue when possible.

First version may keep existing important-post navigation for timeline and add separate queue navigation only when overlay is open. If so, document that as an explicit product choice.

Preferred follow-up behavior:
- `n/N` outside overlay walks the same triage queue globally
- opening an item updates current context but not queue semantics

If this is too invasive for v1, defer it and keep overlay-local queue movement only.

---

## Rendering

Add a new overlay renderer in `internal/app/view.go`, parallel to the existing switcher/activity/info/team switcher overlays.

Suggested renderer:

```go
func (m Model) renderTriage(width, height int) string
```

Header:
- title: `Inbox <count>` or `Triage <count>`
- help: `enter open · d done · esc close`

Body:
- selected row uses the same `pillStyle`/selection conventions as other overlays
- show `Nothing to triage.` when empty
- keep width and height bounded like other overlays

Use the same focus-safe border and truncation patterns already present in `view.go`.

---

## Key handling

In `handleKey`:
- add `u` to open/close inbox overlay
- when `triageOpen`, route all keys through `handleTriageKey`

Add:

```go
func (m Model) handleTriageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)
```

Expected keys:
- `esc` close overlay
- `j/k`, arrows move selection
- `n/N` next/previous queue item
- `enter` open selected item
- `d` dismiss selected item
- `ctrl+c` quit (consistent with other overlays)

---

## Error handling

If `openTriageItem` cannot resolve a target item anymore because state changed:
- rebuild queue
- move selection to the next valid row
- set a short status like `triage item no longer available`
- keep overlay open

If opening requires asynchronous loading:
- close overlay
- use existing loading statuses such as `opening triage item…`

---

## Testing strategy

### 1. Pure queue tests

Create focused tests for:
- building mention items
- building unread-channel items
- building thread-reply items
- deduping stronger/weaker signals
- sorting order by kind then freshness
- dismissal key reappearance on changed signal

### 2. Model tests

Add tests for:
- `u` opens/closes overlay
- `enter` routes to channel/thread correctly
- `d` dismisses current item and moves selection sensibly
- empty queue keeps overlay open with empty state
- queue rebuild after simulated events
- clamped selection after queue shrink

### 3. Render tests

Add tests for:
- overlay header/help
- empty state text
- selected row rendering
- row truncation and preview presence

### 4. VHS smoke

Add a tape after implementation:
- open inbox
- move selection
- open a mention
- reopen inbox
- dismiss an item
- open a thread reply
- tab between thread messages/reply
- close and quit

---

## Scope boundaries for v1

Do now:
- modal inbox overlay
- local queue synthesis from existing state
- local dismiss semantics
- open selected triage item
- tests + VHS smoke

Do not do now:
- server-backed inbox or special API
- cross-session persistence of dismissed items
- complex scoring ML/rules
- batch actions
- search inside inbox
- multi-column inbox details
- remote “mark all read” workflow

---

## Recommendation

Build the inbox as a **workflow accelerator**, not a second chat mode. Keep it modal, fast, deterministic, and rooted in existing state. The first success criterion is simple: a user should be able to hit one key, see all important work, open the next relevant thing, dismiss it locally if handled, and move on without browsing the sidebar manually.
