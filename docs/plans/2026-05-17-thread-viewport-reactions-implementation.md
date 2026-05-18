# Thread Viewport Reactions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add thread-message selection and compact reaction picker support inside the thread viewport.

**Architecture:** Introduce a minimal thread selection model in `internal/app`, reuse the existing compact reaction picker with an explicit target context, and update both thread-local and cached/timeline copies of reacted posts after successful toggles.

**Tech Stack:** Go, Bubble Tea, existing thread layout/rendering, existing compact reaction picker, Go tests.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the compact timeline reactions slice.
- Read design: `docs/plans/2026-05-17-thread-viewport-reactions-design.md`.
- Relevant existing code:
  - `internal/app/model.go:1035-1147`
  - `internal/app/view.go:436-540,539-629`
  - `internal/app/commands.go` reaction toggle flow
  - `internal/app/reactions_test.go`
  - `internal/app/thread_polish_test.go`
  - `internal/app/thread_shared_composer_test.go`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Reuse the existing reaction picker; do not fork it.
- Keep scope to thread selection + thread reactions only.
- Do not add quote/edit/delete to thread viewport in this slice.

---

### Task 1: Add thread message selection state and navigation

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/view.go`
- Create: `internal/app/thread_selection_test.go`

**Step 1: Write the failing tests**

Create `internal/app/thread_selection_test.go`:

```go
package app

import (
    "testing"

    "band-tui/internal/domain"
    tea "github.com/charmbracelet/bubbletea"
)

func threadKey(s string) tea.KeyMsg {
    switch s {
    case "up":
        return tea.KeyMsg{Type: tea.KeyUp}
    case "down":
        return tea.KeyMsg{Type: tea.KeyDown}
    }
    return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestThreadSelectionDefaultsToLastReply(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.threadOpen = true
    m.threadPosts = []domain.Post{
        {ID: "root", ChannelID: "dev", Message: "root"},
        {ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply 1"},
        {ID: "r2", ChannelID: "dev", RootID: "root", Message: "reply 2"},
    }

    m.clampThreadSelection()
    if m.threadSelected != 2 {
        t.Fatalf("threadSelected = %d", m.threadSelected)
    }
}

func TestHandleThreadKeyMovesSelectedThreadPost(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.threadOpen = true
    m.threadPosts = []domain.Post{
        {ID: "root", ChannelID: "dev", Message: "root"},
        {ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply 1"},
    }
    m.threadSelected = 1

    updated, _ := m.handleThreadKey(threadKey("up"))
    got := updated.(Model)
    if got.threadSelected != 0 {
        t.Fatalf("threadSelected = %d", got.threadSelected)
    }
}
```

Add more edge-clamp tests if needed.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestThreadSelectionDefaultsToLastReply|TestHandleThreadKeyMovesSelectedThreadPost' -count=1
```

Expected: FAIL because `threadSelected`/selection helpers do not exist.

**Step 3: Implement minimal thread selection model**

Add to `Model`:

```go
threadSelected int
```

Add helpers:

```go
func (m *Model) clampThreadSelection()
func (m *Model) defaultThreadSelection()
func (m Model) selectedThreadPost() (domain.Post, bool)
```

Rules:
- if thread has replies, default to the last post;
- if only root exists, select root;
- clamp on load/update/close.

In thread message-mode key handling (`!threadFocusComposer`), change `j/k` and arrows from pure scroll to selection movement, and keep viewport scroll following selection.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/thread_selection_test.go
git commit -m "feat: add thread message selection"
```

---

### Task 2: Render selected thread message explicitly

**Files:**
- Modify: `internal/app/view.go`
- Modify: `internal/app/thread_selection_test.go`

**Step 1: Write the failing tests**

Add render-focused tests:

```go
func TestRenderThreadPostsMarksSelectedMessage(t *testing.T)
```

Set up a small thread and assert that the rendered output includes the selected marker for the chosen thread message.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run TestRenderThreadPostsMarksSelectedMessage -count=1
```

Expected: FAIL until render uses `threadSelected`.

**Step 3: Implement minimal selection rendering**

In `renderThreadPosts`, use `threadSelected` to mark the selected thread message with the same family of visual marker used in timeline selection.

Do not redesign the thread layout beyond that.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go internal/app/thread_selection_test.go
git commit -m "feat: render selected thread message"
```

---

### Task 3: Reuse reaction picker with explicit thread target context

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/actions.go`
- Modify: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Add tests like:

```go
func TestHandleThreadKeyROpensReactionPickerForSelectedThreadPost(t *testing.T)
func TestReactionPickerTargetsSelectedThreadPost(t *testing.T)
```

Assertions:
- `R` from thread messages opens picker
- picker target is the selected thread post, not timeline selected post

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestHandleThreadKeyROpensReactionPickerForSelectedThreadPost|TestReactionPickerTargetsSelectedThreadPost' -count=1
```

Expected: FAIL until picker target context exists.

**Step 3: Implement explicit reaction target context**

Add minimal context state:

```go
type reactionTargetKind int

const (
    reactionTargetTimeline reactionTargetKind = iota
    reactionTargetThread
)

reactionTargetKind  reactionTargetKind
reactionTargetPostID string
```

Add helper:

```go
func (m Model) selectedReactionTarget() (domain.Post, bool)
```

When opening picker:
- timeline `R` sets timeline target
- thread messages `R` sets thread target using selected thread post ID

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/actions.go internal/app/reactions_test.go
git commit -m "feat: target reactions from thread messages"
```

---

### Task 4: Apply reaction toggles to thread posts and mirrored copies

**Files:**
- Modify: `internal/app/commands.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Add tests like:

```go
func TestThreadReactionToggleUpdatesThreadAndCachedCopies(t *testing.T)
func TestThreadReactionToggleUpdatesTimelineWhenRootAlsoVisible(t *testing.T)
func TestFailedThreadReactionLeavesStateUnchanged(t *testing.T)
```

Assertions:
- toggling on a reply updates `threadPosts` and `postsByChannel`
- toggling on a root that is also in timeline updates `m.posts`
- failure leaves local state untouched

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestThreadReactionToggleUpdatesThreadAndCachedCopies|TestThreadReactionToggleUpdatesTimelineWhenRootAlsoVisible|TestFailedThreadReactionLeavesStateUnchanged' -count=1
```

Expected: FAIL until thread-target toggle flow is wired.

**Step 3: Implement minimal thread-aware toggle flow**

Rework the existing toggle command path so it operates on `selectedReactionTarget()` instead of assuming timeline selection.

On success:
- update all matching local copies (`threadPosts`, `postsByChannel`, `m.posts` if same post is visible there)
- close picker
- set status `reaction added` / `reaction removed`

On failure:
- close picker
- leave local state unchanged
- set status `reaction failed`

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/commands.go internal/app/model.go internal/app/reactions_test.go
git commit -m "feat: toggle reactions from thread viewport"
```

---

### Task 5: Render thread reaction badges and final verification

**Files:**
- Modify: `internal/app/view.go`
- Modify: `internal/app/reactions_test.go`
- Optional modify: `README.md` / help text only if thread reaction usage needs explicit mention

**Step 1: Write the failing tests**

Add tests like:

```go
func TestRenderThreadPostShowsReactionBadges(t *testing.T)
func TestRenderThreadPostHighlightsOwnReaction(t *testing.T)
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderThreadPostShowsReactionBadges|TestRenderThreadPostHighlightsOwnReaction' -count=1
```

Expected: FAIL until thread rendering includes reactions.

**Step 3: Implement minimal thread reaction rendering**

Use the same compact reaction badge renderer in thread post rendering as timeline posts.

If docs/help need a note, keep it minimal: thread messages mode also supports `R`.

**Step 4: Run full verification**

Run:

```bash
go test ./internal/app -run 'TestThreadSelection|TestThreadReaction|TestRenderThreadPost|TestReaction' -count=1
go test ./internal/app -count=1
go test ./... -count=1
go build ./cmd/band-tui
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go internal/app/reactions_test.go README.md
git commit -m "feat: add thread viewport reactions"
```

---

## Notes for implementer

- The real primitive in this slice is thread message selection.
- Reuse the existing picker and reaction command path; do not fork reaction logic.
- Prefer storing picker target post ID/context over relying on mutable selected indexes after open.
- Keep reply composer behavior unchanged.
- Do not pull edit/delete/quote into thread viewport in this slice.
