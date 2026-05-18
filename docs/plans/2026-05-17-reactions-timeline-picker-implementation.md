# Timeline Reaction Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a compact timeline-only reaction picker that toggles reactions on the selected message and renders compact reaction badges.

**Architecture:** Extend `domain.Post` and the backend layer with explicit reaction support, add a lightweight picker overlay in the app model, and update local post copies directly after reaction mutations. Keep the first slice limited to timeline focus and a fixed reaction set.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` overlay/timeline architecture, Mattermost REST API, mock backend, Go test.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the edit/delete own-messages slice.
- Read design: `docs/plans/2026-05-17-reactions-timeline-picker-design.md`.
- Relevant existing code:
  - `internal/domain/domain.go`
  - `internal/mattermost/client.go`
  - `internal/mock/backend.go`
  - `internal/app/model.go:589-780`
  - `internal/app/view.go` overlay render patterns and post rendering
  - `internal/app/actions.go`
  - `internal/app/message_actions_test.go`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Scope is timeline focus only.
- Keep the picker reaction list fixed in v1.
- Do not add thread-viewport reactions yet.
- Keep backend interface explicit with add/remove methods rather than toggle magic.

---

### Task 1: Add reaction data model and backend methods

**Files:**
- Modify: `internal/domain/domain.go`
- Modify: `internal/mattermost/client.go`
- Modify: `internal/mattermost/client_test.go`
- Modify: `internal/mock/backend.go`
- Modify/create: `internal/mock/backend_test.go`

**Step 1: Write the failing tests**

In `internal/mattermost/client_test.go`, add focused tests:

```go
func TestAddReaction(t *testing.T)
func TestRemoveReaction(t *testing.T)
```

Assert:
- add hits the expected Mattermost endpoint/method/body;
- remove hits the expected endpoint/method/body or delete path as appropriate for Mattermost API;
- returned `domain.Post` includes normalized reaction data.

In `internal/mock/backend_test.go`, add:

```go
func TestAddReactionMutatesStoredPost(t *testing.T)
func TestRemoveReactionMutatesStoredPost(t *testing.T)
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/mattermost -run 'TestAddReaction|TestRemoveReaction' -count=1
go test ./internal/mock -run 'TestAddReactionMutatesStoredPost|TestRemoveReactionMutatesStoredPost' -count=1
```

Expected: FAIL because reaction backend support does not exist.

**Step 3: Implement minimal reaction backend support**

In `internal/domain/domain.go`, add:

```go
type PostReaction struct {
    Emoji   string
    Count   int
    Reacted bool
}
```

and extend `domain.Post`:

```go
Reactions []PostReaction
```

Extend `domain.Backend` with:

```go
AddReaction(ctx context.Context, postID, emoji string) (Post, error)
RemoveReaction(ctx context.Context, postID, emoji string) (Post, error)
```

Mattermost client:
- implement both methods using the appropriate Mattermost reactions endpoints;
- normalize returned post reaction data into `domain.Post.Reactions`.

Mock backend:
- mutate in-memory posts;
- update/remove the selected emoji reaction correctly;
- keep deterministic behavior.

**Step 4: Run tests to verify they pass**

Run the same commands.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/domain.go internal/mattermost/client.go internal/mattermost/client_test.go internal/mock/backend.go internal/mock/backend_test.go
git commit -m "feat: add reaction backends"
```

---

### Task 2: Add pure reaction toggle helpers

**Files:**
- Modify: `internal/app/actions.go`
- Create: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Create `internal/app/reactions_test.go` with pure helper tests:

```go
func TestToggleReactionAddsMissingEmoji(t *testing.T)
func TestToggleReactionRemovesExistingOwnReaction(t *testing.T)
func TestToggleReactionPreservesOtherReactions(t *testing.T)
```

Recommended pure helpers:

```go
func reactionState(post domain.Post, emoji string) (domain.PostReaction, bool)
func mergeAddedReaction(post domain.Post, emoji string) domain.Post
func mergeRemovedReaction(post domain.Post, emoji string) domain.Post
```

Assertions:
- add creates `Count=1`, `Reacted=true` if missing;
- remove decrements/removes when `Reacted=true`;
- unrelated reactions remain intact.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestToggleReaction|TestReactionState' -count=1
```

Expected: FAIL because helpers do not exist.

**Step 3: Implement minimal pure helpers**

Keep this task pure and local. No app model changes yet except helper definitions.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/reactions_test.go
git commit -m "feat: add reaction toggle helpers"
```

---

### Task 3: Add picker overlay state, rendering, and key handling

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/reactions_test.go`
- Optional create: `internal/app/reactions_render_test.go`

**Step 1: Write the failing tests**

Add tests like:

```go
func TestHandleTimelineKeyROpensReactionPicker(t *testing.T)
func TestHandleReactionPickerEscCloses(t *testing.T)
func TestRenderReactionPickerShowsChoices(t *testing.T)
```

Assertions:
- `R` in timeline focus opens picker if a post is selected;
- `esc` closes it;
- render contains the fixed emoji set.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestHandleTimelineKeyROpensReactionPicker|TestHandleReactionPickerEscCloses|TestRenderReactionPickerShowsChoices' -count=1
```

Expected: FAIL because picker state/rendering do not exist.

**Step 3: Implement minimal picker overlay**

Add to `Model`:

```go
reactionPickerOpen     bool
reactionPickerSelected int
```

Add fixed picker list, e.g.:

```go
var defaultReactions = []string{"👍", "👀", "✅", "❤️", "🎉"}
```

Add view helpers:

```go
func (m Model) renderReactionPicker(width, height int) string
```

Add key handling:
- `R` from timeline opens picker
- in picker: `j/k`, arrows, `enter`, `esc`

Place picker in the same precedence family as current centered overlays.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/reactions_test.go internal/app/reactions_render_test.go
git commit -m "feat: add reaction picker overlay"
```

---

### Task 4: Wire reaction toggle commands and local post updates

**Files:**
- Modify: `internal/app/commands.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/actions.go`
- Modify: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Add tests like:

```go
func TestReactionPickerEnterAddsReaction(t *testing.T)
func TestReactionPickerEnterRemovesExistingReaction(t *testing.T)
func TestSuccessfulReactionUpdateReplacesVisibleCachedAndThreadCopies(t *testing.T)
func TestFailedReactionToggleLeavesStateUnchanged(t *testing.T)
```

Use small backend stubs if needed.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestReactionPickerEnter|TestSuccessfulReactionUpdateReplacesVisibleCachedAndThreadCopies|TestFailedReactionToggleLeavesStateUnchanged' -count=1
```

Expected: FAIL because reaction mutation app flow does not exist.

**Step 3: Implement mutation flow**

In `internal/app/commands.go`, add command/result types such as:

```go
type reactionToggledMsg struct {
    post domain.Post
    added bool
    err error
}
```

and a command helper that chooses backend add/remove based on current local reaction state.

In app model/action handling:
- on picker `enter`, inspect selected post + selected emoji;
- dispatch add/remove command;
- on success update:
  - `m.posts`
  - `postsByChannel`
  - `threadPosts`
- close picker;
- set status `reaction added` / `reaction removed`.

On failure:
- close picker;
- leave local state unchanged;
- status `reaction failed`.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/commands.go internal/app/model.go internal/app/actions.go internal/app/reactions_test.go
git commit -m "feat: toggle reactions from timeline"
```

---

### Task 5: Render reaction badges and update help/docs

**Files:**
- Modify: `internal/app/view.go`
- Modify: `README.md`
- Modify: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Add tests like:

```go
func TestRenderPostShowsReactionBadges(t *testing.T)
func TestRenderPostHighlightsOwnReaction(t *testing.T)
func TestHelpTextMentionsReactionKey(t *testing.T)
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderPostShowsReactionBadges|TestRenderPostHighlightsOwnReaction|TestHelpTextMentionsReactionKey' -count=1
```

Expected: FAIL until badges/help/docs are updated.

**Step 3: Implement minimal render/docs**

Render compact badges under/alongside messages, e.g.:

```text
👍 2  👀 1
```

Highlight `Reacted=true` entries with existing accent/pill styling.

Update help and README with:
- `R` — open reaction picker for selected message

**Step 4: Run full verification**

Run:

```bash
go test ./internal/app -run 'TestReaction|TestRenderPostShowsReactionBadges|TestHelpTextMentionsReactionKey' -count=1
go test ./internal/app -count=1
go test ./... -count=1
go build ./cmd/band-tui
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go README.md internal/app/reactions_test.go
git commit -m "docs: add timeline reaction actions"
```

---

## Notes for implementer

- Keep the picker fixed-set and fast. No emoji search in this slice.
- Keep reactions on `domain.Post`, not in a separate side store.
- App layer decides add vs remove; backend exposes explicit methods.
- Do not open thread or change selection as part of reactions.
- Preserve current reliability work: failure paths must not mutate local message state.
