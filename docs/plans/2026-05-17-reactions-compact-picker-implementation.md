# Compact Timeline Reaction Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a compact timeline-only reaction picker that toggles reactions on the selected message and renders compact reaction badges.

**Architecture:** Extend `domain.Post` and the backend layer with explicit reaction support, store reactions directly on posts, add a lightweight picker overlay in the app model, and update local post copies immediately after successful reaction mutations. Keep the first slice limited to timeline focus and a fixed reaction catalog.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` overlay/timeline architecture, Mattermost REST API, mock backend, Go tests.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the edit/delete own-message slice.
- Read design: `docs/plans/2026-05-17-reactions-compact-picker-design.md`.
- Relevant existing code:
  - `internal/domain/domain.go`
  - `internal/mattermost/client.go:188-360,718-760`
  - `internal/mock/backend.go`
  - `internal/app/model.go:589-780`
  - `internal/app/view.go` overlay and message rendering sections
  - `internal/app/actions.go`
  - `internal/app/message_actions_test.go`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Timeline focus only.
- Fixed reaction set only.
- App layer decides add vs remove.
- No thread-viewport reactions in this slice.

---

### Task 1: Add reaction data model and backend methods

**Files:**
- Modify: `internal/domain/domain.go`
- Modify: `internal/mattermost/client.go`
- Modify: `internal/mattermost/client_test.go`
- Modify: `internal/mock/backend.go`
- Modify/create: `internal/mock/backend_test.go`

**Step 1: Write the failing tests**

In `internal/mattermost/client_test.go`, add:

```go
func TestAddReaction(t *testing.T)
func TestRemoveReaction(t *testing.T)
```

Assertions:
- add uses `POST /api/v4/reactions`
- body contains current user ID, post ID, and emoji name
- remove uses `DELETE /api/v4/users/{userID}/posts/{postID}/reactions/{emojiName}`
- returned `domain.Post` includes normalized reactions

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

Expected: FAIL.

**Step 3: Implement minimal backend support**

In `internal/domain/domain.go`, add:

```go
type PostReaction struct {
    Name    string
    Count   int
    Reacted bool
}
```

Extend `domain.Post`:

```go
Reactions []PostReaction
```

Extend `domain.Backend`:

```go
AddReaction(ctx context.Context, postID, emojiName string) (Post, error)
RemoveReaction(ctx context.Context, postID, emojiName string) (Post, error)
```

In Mattermost client:
- add a small `mmReaction` type matching Mattermost reaction JSON
- extend `mmPost` to parse reaction metadata if present
- add a helper that converts Mattermost reactions into aggregated `[]domain.PostReaction` using the current user ID
- implement `AddReaction` and `RemoveReaction`
- after mutation, fetch the updated post or post+reactions as needed so the returned `domain.Post` contains the new `Reactions`

In mock backend:
- mutate the stored post reactions in memory
- keep add/remove deterministic and idempotent enough for tests

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/domain.go internal/mattermost/client.go internal/mattermost/client_test.go internal/mock/backend.go internal/mock/backend_test.go
git commit -m "feat: add reaction backends"
```

---

### Task 2: Add pure reaction helpers

**Files:**
- Modify: `internal/app/actions.go`
- Create: `internal/app/reactions_test.go`

**Step 1: Write the failing tests**

Create `internal/app/reactions_test.go` with pure helper coverage:

```go
func TestReactionStateFindsExistingReaction(t *testing.T)
func TestMergeAddedReactionAddsMissingEmoji(t *testing.T)
func TestMergeRemovedReactionRemovesOwnReaction(t *testing.T)
func TestMergeRemovedReactionDropsZeroCountReaction(t *testing.T)
```

Recommended helpers:

```go
func reactionState(post domain.Post, emojiName string) (domain.PostReaction, bool)
func mergeAddedReaction(post domain.Post, emojiName string) domain.Post
func mergeRemovedReaction(post domain.Post, emojiName string) domain.Post
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestReactionState|TestMergeAddedReaction|TestMergeRemovedReaction' -count=1
```

Expected: FAIL.

**Step 3: Implement minimal pure helpers**

Keep this task pure. No picker/model work yet.

Rules:
- add missing reaction => `Count=1`, `Reacted=true`
- add existing reaction => increment count, set `Reacted=true`
- remove own reaction => decrement count, set/remove reaction accordingly
- preserve unrelated reactions

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/reactions_test.go
git commit -m "feat: add reaction toggle helpers"
```

---

### Task 3: Add picker overlay state and key handling

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/view.go`
- Modify: `internal/app/reactions_test.go`
- Optional create: `internal/app/reactions_render_test.go`

**Step 1: Write the failing tests**

Add tests such as:

```go
func TestHandleTimelineKeyROpensReactionPicker(t *testing.T)
func TestHandleReactionPickerEscCloses(t *testing.T)
func TestRenderReactionPickerShowsChoices(t *testing.T)
```

Assertions:
- `R` in timeline focus opens picker
- `esc` closes it
- render contains the fixed reaction catalog

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestHandleTimelineKeyROpensReactionPicker|TestHandleReactionPickerEscCloses|TestRenderReactionPickerShowsChoices' -count=1
```

Expected: FAIL.

**Step 3: Implement minimal picker state/rendering**

Add to `Model`:

```go
reactionPickerOpen     bool
reactionPickerSelected int
```

Add fixed catalog, e.g.:

```go
type reactionOption struct {
    Name  string
    Glyph string
}

var defaultReactions = []reactionOption{
    {Name: "+1", Glyph: "👍"},
    {Name: "eyes", Glyph: "👀"},
    {Name: "white_check_mark", Glyph: "✅"},
    {Name: "heart", Glyph: "❤️"},
    {Name: "tada", Glyph: "🎉"},
}
```

Add picker overlay render helper and key handling.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/view.go internal/app/reactions_test.go internal/app/reactions_render_test.go
git commit -m "feat: add reaction picker overlay"
```

---

### Task 4: Wire reaction toggle commands and local updates

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

Expected: FAIL.

**Step 3: Implement mutation flow**

In `internal/app/commands.go`, add a command/result type such as:

```go
type reactionToggledMsg struct {
    post  domain.Post
    added bool
    err   error
}
```

The command helper should:
- inspect the selected post’s current reaction state
- call backend add or remove explicitly
- return the updated post and whether the operation was add/remove

In the app model:
- picker `enter` dispatches the command
- on success, replace/update the post in:
  - `m.posts`
  - `postsByChannel`
  - `threadPosts`
- close picker
- status `reaction added` / `reaction removed`

On failure:
- close picker
- leave local state unchanged
- status `reaction failed`

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

Add tests:

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

Expected: FAIL.

**Step 3: Implement minimal rendering/docs**

Render compact reaction badges, using glyph lookup from reaction name.

Rules:
- render only non-zero reactions
- keep stable catalog order first
- visually distinguish `Reacted=true`

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

- Keep reactions keyed by emoji name, not glyph.
- Render glyphs through a small local lookup helper.
- App layer decides add vs remove; backend exposes explicit methods.
- No thread-viewport reactions yet.
- Preserve current reliability guarantees: failed reaction toggles must not mutate local message state.
