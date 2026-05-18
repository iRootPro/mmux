# Triage Inbox Overlay Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a modal triage inbox overlay that lets the user quickly open and locally dismiss mentions, unread channels, and thread replies from one keyboard-first queue.

**Architecture:** Implement inbox as a thin workflow layer over existing app state, not as a new backend feature. Build normalized triage items from `channels`, `recentEvents`, `posts`, `postsByChannel`, and known thread state; keep dismissed items in session-local UI state; render a centered modal box in the same family as existing activity/switcher overlays. For v1, `u` opens the inbox and `n/N` inside the inbox move through the queue; existing global `n/N` unread navigation stays unchanged.

**Tech Stack:** Go, Bubble Tea, Bubbles viewport/textarea, Lip Gloss, existing `internal/app` model/view architecture, existing `domain.Channel` and `domain.Post` types.

---

## Prerequisites

Read before implementing:
- Design: `docs/plans/2026-05-17-triage-inbox-design.md`
- Existing overlay references:
  - `internal/app/model.go:418-624`
  - `internal/app/model.go:743-790`
  - `internal/app/model.go:1556-1618`
  - `internal/app/view.go:45-98`
  - `internal/app/view.go:183-219`
  - `internal/app/view.go:484-540`
- Existing routing/jump helpers:
  - `internal/app/model.go:1135-1170`
  - `internal/app/model.go:1469-1511`
- Existing help/docs:
  - `internal/app/view.go:1299-1324`
  - `README.md:94-121`

Session decision for v1:
- open inbox with `u`
- modal box, centered like existing popups
- queue navigation inside overlay with `j/k`, arrows, `n/N`
- `d` = local dismiss only
- do **not** replace existing global `n/N` behavior yet

---

### Task 1: Add triage types and pure queue builder

**Files:**
- Create: `internal/app/triage.go`
- Test: create `internal/app/triage_test.go`
- Reference only: `internal/app/model.go:1556-1618`

**Step 1: Write the failing tests**

Create `internal/app/triage_test.go` with focused pure tests:

```go
package app

import (
	"testing"
	"time"

	"band-tui/internal/domain"
)

func TestBuildTriageItemsOrdersMentionThreadUnread(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	m := Model{
		channels: []domain.Channel{
			{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3},
			{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1},
		},
		recentEvents: []domain.Post{{ID: "p1", ChannelID: "dev", Username: "Artyom", Message: "Need help", Mentioned: true, CreateAt: now.Add(-2 * time.Minute).UnixMilli()}},
		postsByChannel: map[string][]domain.Post{
			"alerts": {{ID: "a1", ChannelID: "alerts", Username: "bot", Message: "3 unread", Unread: true, CreateAt: now.Add(-10 * time.Minute).UnixMilli()}},
			"dev":    {{ID: "t1", ChannelID: "dev", RootID: "root", Username: "Nika", Message: "new reply", ThreadUnread: true, CreateAt: now.Add(-5 * time.Minute).UnixMilli()}},
		},
	}

	items := buildTriageItems(m)
	if len(items) != 3 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if items[0].Kind != triageMention || items[1].Kind != triageThreadReply || items[2].Kind != triageUnreadChannel {
		t.Fatalf("unexpected order: %#v", items)
	}
}

func TestBuildTriageItemsPrefersMentionOverWeakerUnreadDuplicate(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Mentions: 1, Unread: 4}},
		recentEvents: []domain.Post{{ID: "p1", ChannelID: "dev", Username: "Artyom", Message: "Need help", Mentioned: true, CreateAt: 100}},
	}

	items := buildTriageItems(m)
	if len(items) != 1 || items[0].Kind != triageMention {
		t.Fatalf("expected one mention item, got %#v", items)
	}
}

func TestTriageDismissKeyChangesWhenSignalChanges(t *testing.T) {
	base := triageItem{Kind: triageUnreadChannel, ChannelID: "alerts", UnreadCount: 3, CreateAt: 100}
	changed := base
	changed.UnreadCount = 5
	if triageDismissKey(base) == triageDismissKey(changed) {
		t.Fatal("dismiss key should change when signal changes")
	}
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestBuildTriageItems|TestTriageDismissKeyChangesWhenSignalChanges' -count=1
```

Expected: FAIL with undefined `triageItem`, `buildTriageItems`, and `triageDismissKey` symbols.

**Step 3: Write the minimal implementation**

Create `internal/app/triage.go` with the core types and pure builder.

Start with:

```go
package app

import (
	"fmt"
	"sort"
	"strings"

	"band-tui/internal/domain"
)

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

Implement these helpers in the same file:

```go
func buildTriageItems(m Model) []triageItem
func buildMentionTriageItems(m Model) []triageItem
func buildThreadReplyTriageItems(m Model) []triageItem
func buildUnreadChannelTriageItems(m Model, blocked map[string]struct{}) []triageItem
func triageDismissKey(item triageItem) string
func triageSortLess(a, b triageItem) bool
func triageKindPriority(kind triageKind) int
```

Implementation requirements:
- mentions first, then thread replies, then unread channels
- newer items first within a kind
- use `channelLabel(channelID)` for `Title` when possible
- sanitize preview with `sanitizeMessageText` and `sanitizeTerminal`
- dedupe weaker unread-channel rows when a mention already exists for the same channel
- return stable deterministic order via `sort.SliceStable`

For v1, thread reply items may be synthesized from loaded posts where `ThreadUnread` is true; if root preview is missing, degraded preview is acceptable.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestBuildTriageItems|TestTriageDismissKeyChangesWhenSignalChanges' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/triage.go internal/app/triage_test.go
git commit -m "feat: add triage queue builder"
```

---

### Task 2: Add triage state to the model and queue refresh logic

**Files:**
- Modify: `internal/app/model.go:36-91`
- Modify: `internal/app/model.go:203-281,354-362,680-684,1469-1511`
- Test: modify `internal/app/triage_test.go`

**Step 1: Write the failing tests**

Append to `internal/app/triage_test.go`:

```go
func TestRebuildTriageItemsSkipsDismissedButReaddsChangedSignal(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3}},
		dismissedTriage: map[string]struct{}{},
	}

	m.rebuildTriageItems()
	if len(m.triageItems) != 1 {
		t.Fatalf("triage len = %d", len(m.triageItems))
	}
	key := triageDismissKey(m.triageItems[0])
	m.dismissedTriage[key] = struct{}{}
	m.rebuildTriageItems()
	if len(m.triageItems) != 0 {
		t.Fatalf("dismissed item should be hidden: %#v", m.triageItems)
	}

	m.channels[0].Unread = 5
	m.rebuildTriageItems()
	if len(m.triageItems) != 1 {
		t.Fatalf("changed signal should reappear: %#v", m.triageItems)
	}
}

func TestRebuildTriageItemsClampsSelection(t *testing.T) {
	m := Model{
		channels: []domain.Channel{
			{ID: "a", Type: "O", DisplayName: "a", Unread: 1},
			{ID: "b", Type: "O", DisplayName: "b", Unread: 1},
		},
		triageSelected: 1,
	}
	m.rebuildTriageItems()
	if m.triageSelected != 1 {
		t.Fatalf("selected = %d", m.triageSelected)
	}
	m.channels = m.channels[:1]
	m.rebuildTriageItems()
	if m.triageSelected != 0 {
		t.Fatalf("selected should clamp to 0, got %d", m.triageSelected)
	}
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRebuildTriageItems' -count=1
```

Expected: FAIL with undefined `dismissedTriage`, `triageItems`, `triageSelected`, or `rebuildTriageItems`.

**Step 3: Write the minimal implementation**

In `internal/app/model.go`, add new fields to `Model`:

```go
triageOpen      bool
triageSelected  int
triageItems     []triageItem
dismissedTriage map[string]struct{}
```

Initialize `dismissedTriage` in `New()`.

Add helpers in `model.go` or `triage.go`:

```go
func (m *Model) rebuildTriageItems()
func (m *Model) clampTriageSelection()
func (m *Model) dismissCurrentTriageItem() bool
```

Implementation requirements:
- `rebuildTriageItems()` builds from `buildTriageItems(*m)`
- filter out dismissed keys
- clamp selection after rebuild
- when queue becomes empty, keep `triageSelected` at 0

Call `rebuildTriageItems()` in the existing update flows after state changes that affect important items:
- after channels load
- after posts load
- after older posts load
- after websocket post insertion
- after thread load
- after reply sent
- after current channel view/read-state changes
- after scope reset/clear state

Use the existing state transitions already in `model.go`; do not create a second asynchronous data pipeline.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestBuildTriageItems|TestRebuildTriageItems' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/triage.go internal/app/triage_test.go
 git commit -m "feat: track triage queue state"
```

---

### Task 3: Add the modal triage renderer and key handling

**Files:**
- Modify: `internal/app/model.go:418-624,743-790`
- Modify: `internal/app/view.go:45-98,183-219,484-540,1299-1324`
- Test: create `internal/app/triage_render_test.go`

**Step 1: Write the failing tests**

Create `internal/app/triage_render_test.go`:

```go
package app

import (
	"strings"
	"testing"
)

func TestRenderTriageShowsEmptyState(t *testing.T) {
	m := Model{triageOpen: true}
	got := m.renderTriage(120, 30)
	if !strings.Contains(got, "Triage 0") || !strings.Contains(got, "Nothing to triage.") {
		t.Fatalf("triage overlay = %q", got)
	}
}

func TestRenderTriageShowsSelectedRow(t *testing.T) {
	m := Model{
		triageOpen: true,
		triageItems: []triageItem{{Kind: triageMention, Title: "#dev", Actor: "Artyom", Preview: "Need help", CreateAt: 100}},
	}
	got := m.renderTriage(120, 30)
	if !strings.Contains(got, "Artyom") || !strings.Contains(got, "Need help") {
		t.Fatalf("triage overlay lost row content: %q", got)
	}
}

func TestHandleKeyTogglesTriageOverlay(t *testing.T) {
	m := Model{}
	updated, _ := m.handleKey(key("u"))
	m = updated.(Model)
	if !m.triageOpen {
		t.Fatal("triage should open")
	}
	updated, _ = m.handleKey(key("esc"))
	m = updated.(Model)
	if m.triageOpen {
		t.Fatal("triage should close")
	}
}
```

If you need a local helper for tests, add this in the same test file:

```go
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
```

Or use explicit `tea.KeyMsg` values if that helper conflicts with existing tests.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderTriage|TestHandleKeyTogglesTriageOverlay' -count=1
```

Expected: FAIL because `triageOpen`, `renderTriage`, and `u` handling do not exist.

**Step 3: Write the minimal implementation**

In `internal/app/view.go`, add:

```go
func (m Model) renderTriage(width, height int) string
func (m Model) renderTriageItem(item triageItem, width int) string
```

Render requirements:
- centered box using the same `boxStyle` and size heuristics as the existing overlays
- header like `Triage <count>`
- help line `enter open · d done · esc close`
- empty body text `Nothing to triage.`
- selected row uses `pillStyle`
- row format examples:
  - mention: `@ #dev · Artyom · Need help`
  - thread reply: `↳ #dev · Nika · new reply`
  - unread channel: `• #alerts · 3 unread`

In `internal/app/model.go`:
- add `triageOpen` precedence to `View()` before the normal main/thread rendering branch
- add `u` handling in `handleKey`
- add `handleTriageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)` patterned after `handleActivityKey`
- handle `esc`, `j/k`, arrows, `n/N`, `ctrl+c`

For v1, implement movement and close behavior in this task; reserve `enter` and `d` behavior for the next task.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestRenderTriage|TestHandleKeyTogglesTriageOverlay' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/triage_render_test.go
 git commit -m "feat: add triage inbox overlay"
```

---

### Task 4: Implement open and local dismiss semantics

**Files:**
- Modify: `internal/app/model.go:472-552,1135-1170`
- Modify: `internal/app/actions.go:12-38`
- Modify: `internal/app/triage.go`
- Test: modify `internal/app/triage_test.go`

**Step 1: Write the failing tests**

Append to `internal/app/triage_test.go`:

```go
func TestHandleTriageEnterOpensUnreadChannel(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "alerts", Type: "O", DisplayName: "alerts", Unread: 3}},
		selectedChannel: 0,
		triageOpen: true,
		triageItems: []triageItem{{Kind: triageUnreadChannel, ChannelID: "alerts", Title: "#alerts", UnreadCount: 3}},
	}

	updated, _ := m.handleTriageKey(key("enter"))
	got := updated.(Model)
	if got.triageOpen {
		t.Fatal("triage should close after open")
	}
	if got.status == "" {
		t.Fatal("open should set loading/refresh status")
	}
}

func TestHandleTriageEnterOpensThreadRoot(t *testing.T) {
	m := Model{
		triageOpen: true,
		triageItems: []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root-1", PostID: "reply-1"}},
	}

	updated, _ := m.handleTriageKey(key("enter"))
	got := updated.(Model)
	if !got.threadOpen || got.threadRootID != "root-1" {
		t.Fatalf("thread not opened: %#v", got)
	}
}

func TestHandleTriageDismissHidesCurrentItemAndMovesSelection(t *testing.T) {
	m := Model{
		triageOpen: true,
		dismissedTriage: map[string]struct{}{},
		triageItems: []triageItem{
			{Kind: triageUnreadChannel, ChannelID: "a", UnreadCount: 1},
			{Kind: triageUnreadChannel, ChannelID: "b", UnreadCount: 1},
		},
	}

	ok := m.dismissCurrentTriageItem()
	if !ok {
		t.Fatal("dismiss should succeed")
	}
	if len(m.triageItems) != 1 || m.triageItems[0].ChannelID != "b" {
		t.Fatalf("unexpected queue after dismiss: %#v", m.triageItems)
	}
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestHandleTriageEnter|TestHandleTriageDismiss' -count=1
```

Expected: FAIL because `enter` and `d` actions are not implemented.

**Step 3: Write the minimal implementation**

In `internal/app/triage.go`, add:

```go
func (m Model) openTriageItem(item triageItem) (tea.Model, tea.Cmd)
```

Routing requirements:
- `triageUnreadChannel`: locate channel index, set `selectedChannel`, close overlay, call `openCurrentChannel()`
- `triageMention`: set `pendingJumpChannelID` / `pendingJumpPostID` and open the channel
- `triageThreadReply`: set `pendingJumpChannelID`, `pendingJumpPostID`, `pendingJumpThreadID`, then open channel; if the root is already current and loaded, opening the thread directly is also acceptable

In `handleTriageKey`:
- `enter` opens selected item
- `d` calls `dismissCurrentTriageItem()`
- after dismiss, keep overlay open and selection clamped
- when queue becomes empty, keep overlay open so the empty state is visible

Dismiss implementation must:
- add the current `triageDismissKey` to `dismissedTriage`
- call `rebuildTriageItems()`

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestHandleTriageEnter|TestHandleTriageDismiss|TestRebuildTriageItems' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/actions.go internal/app/triage.go internal/app/triage_test.go
 git commit -m "feat: open and dismiss triage items"
```

---

### Task 5: Update help, README, and add VHS smoke

**Files:**
- Modify: `internal/app/view.go:1299-1324`
- Modify: `README.md:94-121`
- Create: `vhs/triage-inbox.tape`
- Optional test extension: `internal/app/triage_render_test.go`

**Step 1: Add any last failing tests you need**

If you want a render/help assertion, add one minimal test such as:

```go
func TestHelpTextMentionsTriageInbox(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "u") || !strings.Contains(got, "triage") {
		t.Fatalf("help text missing triage key: %q", got)
	}
}
```

**Step 2: Run the failing test**

Run:

```bash
go test ./internal/app -run TestHelpTextMentionsTriageInbox -count=1
```

Expected: FAIL until help text is updated.

**Step 3: Write the minimal implementation**

Update `helpText()` and `README.md` to document:
- `u` opens triage inbox
- overlay-local `enter open · d done · esc close`
- `n/N` move inside the queue while triage is open
- local dismiss semantics for v1

Create `vhs/triage-inbox.tape` with a smoke flow:

```text
Output tmp/triage-inbox.gif
Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s
Type "u"
Sleep 1s
Type "j"
Sleep 500ms
Type "d"
Sleep 500ms
Type "u"
Sleep 500ms
Type "esc"
Sleep 500ms
Ctrl+C
```

Adjust the steps after implementation so they exercise a real triage item in mock mode. If mock mode does not naturally produce triage items, extend the mock backend in a separate implementation task before finalizing the tape.

**Step 4: Run targeted tests and VHS**

Run:

```bash
go test ./internal/app -run 'TestHelpTextMentionsTriageInbox|TestRenderTriage|TestHandleTriage' -count=1
vhs vhs/triage-inbox.tape
```

Expected:
- tests PASS
- `tmp/triage-inbox.gif` is created

**Step 5: Commit**

```bash
git add internal/app/view.go README.md vhs/triage-inbox.tape
 git commit -m "docs: add triage inbox workflow"
```

---

### Task 6: Final verification

**Files:**
- No new files required

**Step 1: Run app tests**

```bash
go test ./internal/app -count=1
```

Expected: PASS.

**Step 2: Run full repository tests**

```bash
go test ./... -count=1
```

Expected: PASS.

**Step 3: Run build**

```bash
go build ./cmd/band-tui
```

Expected: success, no output.

**Step 4: Run VHS smoke**

```bash
vhs vhs/triage-inbox.tape
```

Expected: `tmp/triage-inbox.gif` exists and visually confirms:
- inbox opens/closes
- selected row moves
- dismiss removes current item
- open routes into channel or thread context

**Step 5: Commit final polish if needed**

If verification required any final non-doc changes:

```bash
git add <changed files>
git commit -m "test: verify triage inbox workflow"
```

---

## Notes for implementation

- Follow `@superpowers:test-driven-development` strictly for each task.
- Keep v1 inbox overlay-local; do not replace global `n/N` unread navigation yet.
- Reuse existing pending-jump machinery instead of inventing a new navigation path.
- Prefer new triage-specific code in `internal/app/triage.go` rather than growing `model.go` further.
- Keep dismiss semantics local to the session; do not call backend mark-read APIs from `d`.
- If mock mode lacks enough signal variety to demonstrate the inbox, extend `internal/mock/backend.go` in a small dedicated step before the VHS task.
