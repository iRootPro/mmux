# Edit & Delete Own Messages Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add timeline-only edit and delete actions for the user’s own messages.

**Architecture:** Extend the backend interface with update/delete post mutations, add minimal app state for `editingPostID` and `pendingDeletePostID`, and route timeline keys `e` and `D` through a small set of local model helpers. Keep scope strictly to timeline focus and the existing shared composer.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` model/actions architecture, Mattermost REST API, mock backend, Go tests.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the quote/permalink message-actions slice.
- Read design: `docs/plans/2026-05-17-edit-delete-own-messages-design.md`.
- Relevant existing code:
  - `internal/domain/domain.go:139-152`
  - `internal/mattermost/client.go:316-360`
  - `internal/mock/backend.go:123-155`
  - `internal/app/actions.go:113-270`
  - `internal/app/model.go:717-743`
  - `internal/app/message_actions_test.go`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Scope is timeline focus only.
- Only own messages are editable/deletable.
- Do not add thread viewport actions.
- Keep delete confirmation lightweight: double-press `D`, no modal.

---

### Task 1: Extend backend interface and implementations for update/delete

**Files:**
- Modify: `internal/domain/domain.go`
- Modify: `internal/mattermost/client.go`
- Modify: `internal/mattermost/client_test.go`
- Modify: `internal/mock/backend.go`
- Optional create/modify: `internal/mock/backend_test.go`

**Step 1: Write the failing tests**

In `internal/mattermost/client_test.go`, add:

```go
func TestUpdatePost(t *testing.T) {
    // httptest server expecting PUT /api/v4/posts/p1 with {"message":"edited"}
    // return updated post JSON
}

func TestDeletePost(t *testing.T) {
    // httptest server expecting DELETE /api/v4/posts/p1
}
```

In `internal/mock/backend_test.go`, add:

```go
func TestUpdatePostMutatesStoredMessage(t *testing.T)
func TestDeletePostRemovesStoredMessage(t *testing.T)
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/mattermost -run 'TestUpdatePost|TestDeletePost' -count=1
go test ./internal/mock -run 'TestUpdatePostMutatesStoredMessage|TestDeletePostRemovesStoredMessage' -count=1
```

Expected: FAIL because the backend interface and implementations do not support update/delete yet.

**Step 3: Implement minimal backend support**

In `internal/domain/domain.go`, extend `Backend`:

```go
UpdatePost(ctx context.Context, postID, message string) (Post, error)
DeletePost(ctx context.Context, postID string) error
```

In `internal/mattermost/client.go`, add:

```go
func (c *Client) UpdatePost(ctx context.Context, postID, message string) (domain.Post, error)
func (c *Client) DeletePost(ctx context.Context, postID string) error
```

Use Mattermost endpoints:
- `PUT /api/v4/posts/{postID}` with `{"message": ...}`
- `DELETE /api/v4/posts/{postID}`

In `internal/mock/backend.go`, add corresponding methods that mutate/remove posts in memory.

**Step 4: Run tests to verify they pass**

Run the same commands.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/domain.go internal/mattermost/client.go internal/mattermost/client_test.go internal/mock/backend.go internal/mock/backend_test.go
git commit -m "feat: add update and delete post backends"
```

---

### Task 2: Add edit mode state and enter edit with `e`

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/actions.go`
- Modify: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Append to `internal/app/message_actions_test.go`:

```go
func TestEditSelectedOwnPostLoadsComposer(t *testing.T)
func TestCannotEditOtherUsersPost(t *testing.T)
func TestHandleTimelineKeyEEntersEditMode(t *testing.T)
```

Assertions:
- own selected post => composer contains post text, focus becomes composer, `editingPostID` is set;
- чужой post => status `can only edit your own messages`, no edit mode;
- `e` key routes to the same behavior in timeline focus.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestEditSelectedOwnPostLoadsComposer|TestCannotEditOtherUsersPost|TestHandleTimelineKeyEEntersEditMode' -count=1
```

Expected: FAIL because edit mode does not exist.

**Step 3: Implement minimal edit-entry behavior**

In `Model`, add:

```go
editingPostID string
```

In `internal/app/actions.go`, add:

```go
func (m Model) editSelectedPost() (tea.Model, tea.Cmd)
```

Behavior:
- require selected post and `m.isOwnPost(post)`;
- load composer with selected message text;
- focus composer;
- set `editingPostID = post.ID`;
- status `editing message`.

In timeline key handling add:

```go
case "e":
    return m.editSelectedPost()
```

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/actions.go internal/app/message_actions_test.go
git commit -m "feat: enter message edit mode"
```

---

### Task 3: Submit edits through composer and keep failure recovery

**Files:**
- Modify: `internal/app/commands.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/drafts.go` only if strictly needed
- Modify: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Add tests such as:

```go
func TestEditSubmitUsesUpdatePostPath(t *testing.T)
func TestSuccessfulEditClearsEditModeAndUpdatesPost(t *testing.T)
func TestFailedEditKeepsComposerTextAndEditMode(t *testing.T)
```

Use a small backend stub or message types as appropriate.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestEditSubmitUsesUpdatePostPath|TestSuccessfulEditClearsEditModeAndUpdatesPost|TestFailedEditKeepsComposerTextAndEditMode' -count=1
```

Expected: FAIL because composer submit only knows send/reply today.

**Step 3: Implement update-post command path**

In `internal/app/commands.go`, add:

```go
type postUpdatedMsg struct {
    post domain.Post
    err  error
}

func updatePostCmd(ctx context.Context, backend domain.Backend, postID, text string) tea.Cmd
```

In composer `enter` handling:
- if `editingPostID != ""`, call `updatePostCmd` instead of send.

In `Update()`:
- handle `postUpdatedMsg`;
- on success:
  - replace the post in `m.posts`
  - replace it in `postsByChannel`
  - replace it in `threadPosts` if present
  - clear `editingPostID`
  - status `message updated`
- on failure:
  - leave composer text intact
  - keep `editingPostID`
  - status `update failed`

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/commands.go internal/app/model.go internal/app/message_actions_test.go
git commit -m "feat: submit edited messages"
```

---

### Task 4: Add delete confirmation and delete mutation with `D`

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/actions.go`
- Modify: `internal/app/commands.go`
- Modify: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Add tests:

```go
func TestDeleteOwnPostRequiresConfirmation(t *testing.T)
func TestDeleteConfirmationClearsOnSelectionChange(t *testing.T)
func TestCannotDeleteOtherUsersPost(t *testing.T)
func TestSuccessfulDeleteRemovesPostAndClampsSelection(t *testing.T)
```

Assertions:
- first `D` arms same-post confirmation only;
- second `D` on same selected own post triggers delete;
- moving selection clears `pendingDeletePostID`;
- successful delete removes the post from visible/cache/thread lists and clamps selection.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestDeleteOwnPostRequiresConfirmation|TestDeleteConfirmationClearsOnSelectionChange|TestCannotDeleteOtherUsersPost|TestSuccessfulDeleteRemovesPostAndClampsSelection' -count=1
```

Expected: FAIL because delete state/path does not exist.

**Step 3: Implement minimal delete flow**

Add to `Model`:

```go
pendingDeletePostID string
```

In `internal/app/commands.go` add:

```go
type postDeletedMsg struct {
    postID string
    err    error
}

func deletePostCmd(ctx context.Context, backend domain.Backend, postID string) tea.Cmd
```

In `internal/app/actions.go`, add:

```go
func (m Model) deleteSelectedPost() (tea.Model, tea.Cmd)
```

Behavior:
- require selected own post;
- if `pendingDeletePostID != post.ID`, set it and status `press D again to delete`;
- if same post already armed, dispatch `deletePostCmd`.

In timeline key handling add `case "D": return m.deleteSelectedPost()`.

On successful delete:
- remove post from `m.posts`, `postsByChannel`, and `threadPosts`;
- clear `pendingDeletePostID`;
- clamp `selectedPost`;
- refresh viewport;
- status `message deleted`.

Also clear `pendingDeletePostID` on selection change/navigation and when entering edit mode.

**Step 4: Run tests to verify they pass**

Run the same command.

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/model.go internal/app/commands.go internal/app/message_actions_test.go
git commit -m "feat: delete own messages from timeline"
```

---

### Task 5: Update help/docs and final verification

**Files:**
- Modify: `internal/app/view.go`
- Modify: `README.md`
- Modify: `internal/app/message_actions_test.go`

**Step 1: Add docs test**

Add:

```go
func TestHelpTextMentionsEditAndDeleteKeys(t *testing.T)
```

Assert exact help rows for `e` and `D`.

**Step 2: Run docs test to verify it fails**

```bash
go test ./internal/app -run TestHelpTextMentionsEditAndDeleteKeys -count=1
```

Expected: FAIL.

**Step 3: Update docs**

In help text and README timeline action lines, document:
- `e` edit selected own message
- `D` delete selected own message (double-press confirm)

**Step 4: Run full verification**

```bash
go test ./internal/app -run 'TestFormatQuotedReply|TestQuoteSelectedPost|TestSelectedPostPermalink|TestEdit|TestDelete|TestHelpTextMentions' -count=1
go test ./internal/app -count=1
go test ./... -count=1
go build ./cmd/band-tui
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go README.md internal/app/message_actions_test.go
git commit -m "docs: add edit and delete message actions"
```

---

## Notes for implementer

- Keep edit/delete timeline-only in this slice.
- Do not invent a generic message action abstraction unless repetition becomes obvious.
- Edit mode should reuse the current composer rather than creating a second input.
- Delete confirmation must be simple and local; no modal.
- Preserve the reliability work already done: failure paths should not lose text or leave stale local state.
