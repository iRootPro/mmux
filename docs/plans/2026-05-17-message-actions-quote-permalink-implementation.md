# Quote Reply & Permalink Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add keyboard-first quote-reply insertion and selected-message permalink copy for timeline posts.

**Architecture:** Keep this slice entirely inside `internal/app`: build a pure quote formatter and a best-effort permalink builder, then wire new timeline keys (`r` and `p`) into the existing composer and clipboard action flow. No backend interface changes are required.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` action/model/view architecture, clipboard integration, current `domain.Post` and `domain.Session` state.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the unread correctness hardening milestone.
- Read design: `docs/plans/2026-05-17-message-actions-quote-permalink-design.md`.
- Relevant existing code:
  - `internal/app/actions.go:113-170`
  - `internal/app/model.go:717-735`
  - `internal/app/view.go:1389-1416`
  - `README.md:94-122`
- Existing supporting state already available:
  - `selectedPostIndex()`
  - `m.composer`
  - `m.session`
  - `m.channels`
  - `copySelectedPostText()`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Do not change `domain.Backend`.
- Do not add thread-viewport selection or a message action menu.
- Keep quote insertion local to the existing composer and current destination.
- Keep permalink generation best-effort with a clean fallback.

---

### Task 1: Add pure quote formatting helper

**Files:**
- Modify: `internal/app/actions.go`
- Create: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Create `internal/app/message_actions_test.go`:

```go
package app

import (
    "testing"

    "band-tui/internal/domain"
)

func TestFormatQuotedReplySingleLine(t *testing.T) {
    post := domain.Post{Username: "Alice", Message: "Hello"}
    got := formatQuotedReply(post)
    want := "> Alice:\n> Hello\n\n"
    if got != want {
        t.Fatalf("quote = %q, want %q", got, want)
    }
}

func TestFormatQuotedReplyMultiline(t *testing.T) {
    post := domain.Post{Username: "Alice", Message: "line 1\nline 2"}
    got := formatQuotedReply(post)
    want := "> Alice:\n> line 1\n> line 2\n\n"
    if got != want {
        t.Fatalf("quote = %q, want %q", got, want)
    }
}

func TestFormatQuotedReplyUsesUnknownWhenAuthorMissing(t *testing.T) {
    post := domain.Post{Message: "Hello"}
    got := formatQuotedReply(post)
    want := "> unknown:\n> Hello\n\n"
    if got != want {
        t.Fatalf("quote = %q, want %q", got, want)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestFormatQuotedReply' -count=1
```

Expected: FAIL because `formatQuotedReply` does not exist.

**Step 3: Implement minimal helper**

In `internal/app/actions.go`, add:

```go
func formatQuotedReply(post domain.Post) string
```

Rules:

- trim leading/trailing message whitespace;
- if trimmed message is empty, return `""`;
- author name order:
  - `post.Username`
  - `shortID(post.UserID)` if available
  - `"unknown"`
- first line: `> <author>:`
- then each message line as `> <line>`
- always terminate with a blank line.

Do not over-normalize content; preserve line order and ordinary text.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestFormatQuotedReply' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/message_actions_test.go
git commit -m "feat: add quote formatting helper"
```

---

### Task 2: Insert quote into the composer with `r`

**Files:**
- Modify: `internal/app/actions.go`
- Modify: `internal/app/model.go:717-735`
- Modify: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestQuoteSelectedPostIntoEmptyComposer(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.focus = focusTimeline
    m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
    m.selectedPost = 0

    updated, _ := m.quoteSelectedPost()
    got := updated.(Model)
    if got.focus != focusComposer {
        t.Fatalf("focus = %v, want composer", got.focus)
    }
    want := "> Alice:\n> Hello\n\n"
    if got.composer.Value() != want {
        t.Fatalf("composer = %q, want %q", got.composer.Value(), want)
    }
}

func TestQuoteSelectedPostAppendsBelowExistingDraft(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.focus = focusTimeline
    m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
    m.selectedPost = 0
    m.composer.SetValue("draft")

    updated, _ := m.quoteSelectedPost()
    got := updated.(Model)
    want := "draft\n> Alice:\n> Hello\n\n"
    if got.composer.Value() != want {
        t.Fatalf("composer = %q, want %q", got.composer.Value(), want)
    }
}

func TestHandleTimelineKeyRQuotesSelectedPost(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.focus = focusTimeline
    m.posts = []domain.Post{{ID: "p1", Username: "Alice", Message: "Hello"}}
    m.selectedPost = 0

    updated, _ := m.handleKey(draftKey("r"))
    got := updated.(Model)
    if got.focus != focusComposer || got.composer.Value() == "" {
        t.Fatalf("quote not inserted: %#v", got.composer.Value())
    }
}
```

Reuse an existing local key helper if one test file already has it; otherwise add a small helper in this file.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestQuoteSelectedPost|TestHandleTimelineKeyRQuotesSelectedPost' -count=1
```

Expected: FAIL because `quoteSelectedPost` and `r` handling do not exist.

**Step 3: Implement minimal quote action**

In `internal/app/actions.go`, add:

```go
func (m Model) quoteSelectedPost() (tea.Model, tea.Cmd)
```

Behavior:

- no-op if no selected post;
- build quote with `formatQuotedReply`;
- if quote is empty, set status `selected message is empty` and return;
- if composer already contains text and does not end with `\n`, insert one separating newline before the quote;
- append quote to composer;
- move focus to composer and call `applyFocus()`;
- set status `quote inserted`.

In `internal/app/model.go`, in timeline focus key handling add:

```go
case "r":
    return m.quoteSelectedPost()
```

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestQuoteSelectedPost|TestHandleTimelineKeyRQuotesSelectedPost' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/model.go internal/app/message_actions_test.go
git commit -m "feat: quote selected message into composer"
```

---

### Task 3: Build permalink helper and copy action with `p`

**Files:**
- Modify: `internal/app/actions.go`
- Modify: `internal/app/model.go:717-735`
- Modify: `internal/app/message_actions_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestSelectedPostPermalinkBuildsTeamScopedURL(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.session = &domain.Session{ServerURL: "https://chat.example.com", Teams: []domain.Team{{ID: "t1", Name: "eng", DisplayName: "Engineering"}}}
    m.channels = []domain.Channel{{ID: "c1", TeamID: "t1", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
    m.selectedPost = 0

    got, ok := m.selectedPostPermalink()
    if !ok {
        t.Fatal("expected permalink")
    }
    want := "https://chat.example.com/eng/pl/p1"
    if got != want {
        t.Fatalf("permalink = %q, want %q", got, want)
    }
}

func TestSelectedPostPermalinkFallsBackToRootPath(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.session = &domain.Session{ServerURL: "https://chat.example.com"}
    m.channels = []domain.Channel{{ID: "c1", Type: "D", DisplayName: "alisa"}}
    m.selectedChannel = 0
    m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
    m.selectedPost = 0

    got, ok := m.selectedPostPermalink()
    if !ok {
        t.Fatal("expected permalink")
    }
    want := "https://chat.example.com/pl/p1"
    if got != want {
        t.Fatalf("permalink = %q, want %q", got, want)
    }
}

func TestCopySelectedPostPermalinkSetsStatus(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.session = &domain.Session{ServerURL: "https://chat.example.com", Teams: []domain.Team{{ID: "t1", Name: "eng"}}}
    m.channels = []domain.Channel{{ID: "c1", TeamID: "t1", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
    m.selectedPost = 0

    updated, cmd := m.copySelectedPostPermalink()
    got := updated.(Model)
    if got.status != "copying permalink…" {
        t.Fatalf("status = %q", got.status)
    }
    if cmd == nil {
        t.Fatal("expected clipboard command")
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestSelectedPostPermalink|TestCopySelectedPostPermalinkSetsStatus' -count=1
```

Expected: FAIL because permalink helpers do not exist.

**Step 3: Implement minimal permalink action**

In `internal/app/actions.go`, add:

```go
func (m Model) selectedPostPermalink() (string, bool)
func (m Model) copySelectedPostPermalink() (tea.Model, tea.Cmd)
```

Rules:

- require selected post and non-empty `Session.ServerURL`;
- derive team slug from the selected post’s channel `TeamID` by matching `m.session.Teams[].ID` and using `Team.Name` first, then `DisplayName` if needed;
- if team slug is unavailable, fallback to `<server>/pl/<postID>`;
- `copySelectedPostPermalink()` should mirror `copySelectedPostText()`:
  - set status `copying permalink…`
  - return `actionDoneMsg{status: "permalink copied"}` on success.

In `internal/app/model.go`, in timeline focus key handling add:

```go
case "p":
    return m.copySelectedPostPermalink()
```

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestSelectedPostPermalink|TestCopySelectedPostPermalinkSetsStatus' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/model.go internal/app/message_actions_test.go
git commit -m "feat: copy selected message permalink"
```

---

### Task 4: Update help/docs and final verification

**Files:**
- Modify: `internal/app/view.go:1389-1416`
- Modify: `README.md:94-122`
- Optional: extend `internal/app/message_actions_test.go`

**Step 1: Add a last failing docs test if useful**

Example:

```go
func TestHelpTextMentionsQuoteAndPermalinkKeys(t *testing.T) {
    m := Model{}
    got := m.helpText()
    if !strings.Contains(got, "r") || !strings.Contains(got, "quote") || !strings.Contains(got, "p") || !strings.Contains(got, "permalink") {
        t.Fatalf("help text missing message action keys: %q", got)
    }
}
```

**Step 2: Run the failing test**

Run:

```bash
go test ./internal/app -run TestHelpTextMentionsQuoteAndPermalinkKeys -count=1
```

Expected: FAIL until docs are updated.

**Step 3: Update docs**

In `helpText()` add timeline actions:

- `r` — quote selected message into composer
- `p` — copy permalink for selected message

In `README.md`, update the timeline-focus key line or split it explicitly to include:

- `r` quote selected message into composer
- `p` copy selected message permalink

**Step 4: Run full verification**

Run:

```bash
go test ./internal/app -run 'TestFormatQuotedReply|TestQuoteSelectedPost|TestSelectedPostPermalink|TestHelpTextMentionsQuoteAndPermalinkKeys' -count=1
go test ./internal/app -count=1
go test ./... -count=1
go build ./cmd/band-tui
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go README.md internal/app/message_actions_test.go
git commit -m "docs: add message quote and permalink actions"
```

---

## Notes for implementer

- Keep quote insertion boring. Do not auto-open threads or infer reply destination.
- Do not parse markdown when quoting; prefix raw lines.
- Permalink should be best-effort, not all-or-nothing.
- This slice is intentionally backend-free. If you feel the urge to add update/delete/reaction APIs, stop — that is a later slice.
