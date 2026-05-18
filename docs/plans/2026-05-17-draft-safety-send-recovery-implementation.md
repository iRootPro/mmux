# Draft Safety & Send Recovery Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Preserve per-channel and per-thread compose drafts across navigation and restore attempted text after send failures.

**Architecture:** Keep the existing single shared `textarea.Model`, but bind its value to a session-local draft destination key. Save the active draft before destination changes, load the new destination draft after the destination changes, and track pending send text by draft key until the backend confirms success or failure.

**Tech Stack:** Go, Bubble Tea, Bubbles textarea, existing `internal/app` model/update architecture, existing `domain.Backend` send APIs.

---

## Prerequisites

Before implementation:

- Finish or commit the current triage inbox working tree. Do not mix draft-safety commits with uncommitted triage changes.
- Read design: `docs/plans/2026-05-17-draft-safety-send-recovery-design.md`.
- Relevant existing code:
  - `internal/app/model.go:36-91` (`Model` fields)
  - `internal/app/model.go:93-139` (`New` composer initialization)
  - `internal/app/model.go:292-325` (`replySentMsg` / `postSentMsg` handling)
  - `internal/app/model.go:650-659` (channel send key handling)
  - `internal/app/model.go:921-929` (thread reply key handling)
  - `internal/app/model.go:1094-1100` (`selectChannel`)
  - `internal/app/actions.go:12-34` (`openSelectedThread`)
  - `internal/app/commands.go:39-56,164-175` (send command message shapes)

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Keep drafts session-local only.
- Keep one shared composer; do not add a second textarea.
- Do not persist drafts to disk in this cycle.
- Do not change backend APIs.

---

### Task 1: Add draft keys and draft storage helpers

**Files:**
- Modify: `internal/app/model.go:36-91,120-138`
- Create: `internal/app/drafts.go`
- Test: create `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Create `internal/app/drafts_test.go`:

```go
package app

import (
    "testing"

    "band-tui/internal/domain"
)

func TestDraftKeysAreStable(t *testing.T) {
    if got := channelDraftKey("dev"); got != "channel:dev" {
        t.Fatalf("channel key = %q", got)
    }
    if got := threadDraftKey("dev", "root-1"); got != "thread:dev:root-1" {
        t.Fatalf("thread key = %q", got)
    }
}

func TestSwitchDraftSavesAndLoadsDestinationText(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{
        {ID: "dev", Type: "O", DisplayName: "dev"},
        {ID: "alerts", Type: "O", DisplayName: "alerts"},
    }
    m.selectedChannel = 0
    m.loadDraft(channelDraftKey("dev"))
    m.composer.SetValue("dev draft")

    m.switchDraft(channelDraftKey("alerts"))
    if got := m.composer.Value(); got != "" {
        t.Fatalf("new destination composer = %q, want empty", got)
    }

    m.composer.SetValue("alerts draft")
    m.switchDraft(channelDraftKey("dev"))
    if got := m.composer.Value(); got != "dev draft" {
        t.Fatalf("restored dev draft = %q", got)
    }
    if got := m.drafts[channelDraftKey("alerts")]; got != "alerts draft" {
        t.Fatalf("saved alerts draft = %q", got)
    }
}

func TestSaveActiveDraftDropsEmptyDraft(t *testing.T) {
    m := New(nil, testConfig(), false)
    key := channelDraftKey("dev")
    m.drafts[key] = "old"
    m.loadDraft(key)
    m.composer.SetValue("   ")

    m.saveActiveDraft()
    if _, ok := m.drafts[key]; ok {
        t.Fatalf("empty draft should be removed: %#v", m.drafts)
    }
}
```

Add local key helper if this test file needs to drive key handlers:

```go
func draftKey(s string) tea.KeyMsg {
    switch s {
    case "esc":
        return tea.KeyMsg{Type: tea.KeyEsc}
    case "enter":
        return tea.KeyMsg{Type: tea.KeyEnter}
    }
    return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
```

If `testConfig()` does not exist, add a small helper in this test file:

```go
func testConfig() config.Config { return config.Config{Mock: true} }
```

and import `band-tui/internal/config` and `tea "github.com/charmbracelet/bubbletea"` as needed.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestDraftKeys|TestSwitchDraft|TestSaveActiveDraft' -count=1
```

Expected: FAIL with undefined draft helper/field symbols.

**Step 3: Implement minimal helpers**

In `Model`, add:

```go
drafts         map[string]string
activeDraftKey string
pendingSends   map[string]string
```

Initialize in `New()`:

```go
drafts:       map[string]string{},
pendingSends: map[string]string{},
```

Create `internal/app/drafts.go`:

```go
package app

import "strings"

func channelDraftKey(channelID string) string {
    if channelID == "" {
        return ""
    }
    return "channel:" + channelID
}

func threadDraftKey(channelID, rootID string) string {
    if channelID == "" || rootID == "" {
        return ""
    }
    return "thread:" + channelID + ":" + rootID
}

func (m Model) currentDraftKey() string {
    if m.threadOpen && m.threadRootID != "" {
        return threadDraftKey(m.currentChannelID(), m.threadRootID)
    }
    return channelDraftKey(m.currentChannelID())
}

func (m *Model) saveActiveDraft() {
    if m.activeDraftKey == "" {
        return
    }
    text := strings.TrimSpace(m.composer.Value())
    if text == "" {
        delete(m.drafts, m.activeDraftKey)
        return
    }
    m.drafts[m.activeDraftKey] = m.composer.Value()
}

func (m *Model) loadDraft(key string) {
    m.activeDraftKey = key
    m.composer.SetValue(m.drafts[key])
}

func (m *Model) switchDraft(key string) {
    if key == m.activeDraftKey {
        return
    }
    m.saveActiveDraft()
    m.loadDraft(key)
}

func (m *Model) clearDraft(key string) {
    if key == "" {
        return
    }
    delete(m.drafts, key)
    delete(m.pendingSends, key)
    if key == m.activeDraftKey {
        m.composer.Reset()
    }
}
```

Guard nil maps in helpers if tests instantiate `Model{}` directly:

```go
if m.drafts == nil { m.drafts = map[string]string{} }
if m.pendingSends == nil { m.pendingSends = map[string]string{} }
```

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestDraftKeys|TestSwitchDraft|TestSaveActiveDraft' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/drafts.go internal/app/drafts_test.go
git commit -m "feat: add local draft storage"
```

---

### Task 2: Preserve channel drafts across channel navigation

**Files:**
- Modify: `internal/app/model.go:174-240,1094-1100,1135-1156`
- Test: modify `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestChannelDraftSurvivesChannelSwitch(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{
        {ID: "dev", Type: "O", DisplayName: "dev"},
        {ID: "alerts", Type: "O", DisplayName: "alerts"},
    }
    m.selectedChannel = 0
    m.loadDraft(channelDraftKey("dev"))
    m.composer.SetValue("dev text")

    updated, _ := m.selectChannel(1)
    m = updated.(Model)
    if got := m.composer.Value(); got != "" {
        t.Fatalf("alerts composer = %q, want empty", got)
    }

    m.composer.SetValue("alerts text")
    updated, _ = m.selectChannel(0)
    m = updated.(Model)
    if got := m.composer.Value(); got != "dev text" {
        t.Fatalf("dev draft restored = %q", got)
    }
    if got := m.drafts[channelDraftKey("alerts")]; got != "alerts text" {
        t.Fatalf("alerts draft saved = %q", got)
    }
}

func TestInitialChannelLoadsChannelDraftKey(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0

    m.switchDraft(m.currentDraftKey())
    if got := m.activeDraftKey; got != channelDraftKey("dev") {
        t.Fatalf("active draft key = %q", got)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestChannelDraftSurvivesChannelSwitch|TestInitialChannelLoadsChannelDraftKey' -count=1
```

Expected: first test FAILS because channel switching does not save/load destination drafts yet.

**Step 3: Implement channel draft switching**

Update destination transitions:

- In `channelsLoadedMsg`, after `m.selectedChannel = m.pickChannel()`, call:

```go
m.switchDraft(m.currentDraftKey())
```

- In `selectChannel(index int)`, save old draft before changing selected channel, then load new channel draft:

```go
m.saveActiveDraft()
m.selectedChannel = index
m.loadDraft(m.currentDraftKey())
return m.openCurrentChannel()
```

- In `openCurrentChannel()`, do not overwrite composer value directly. It should assume the caller already selected/loaded the destination draft. For defensive direct calls, add:

```go
if m.activeDraftKey == "" {
    m.loadDraft(m.currentDraftKey())
}
```

Do not emit `draft saved` status on every switch.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestDraft|TestChannelDraft' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/drafts_test.go
git commit -m "feat: preserve channel drafts"
```

---

### Task 3: Preserve thread drafts across thread open/close

**Files:**
- Modify: `internal/app/actions.go:12-34`
- Modify: `internal/app/model.go:230-245,865-878,921-929`
- Test: modify `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestThreadDraftSurvivesCloseAndReopen(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.posts = []domain.Post{{ID: "root", ChannelID: "dev", Username: "Alice", Message: "root"}}
    m.selectedPost = 0
    m.loadDraft(channelDraftKey("dev"))
    m.composer.SetValue("channel text")

    updated, _ := m.openSelectedThread()
    m = updated.(Model)
    if got := m.activeDraftKey; got != threadDraftKey("dev", "root") {
        t.Fatalf("active thread draft key = %q", got)
    }
    if got := m.composer.Value(); got != "" {
        t.Fatalf("new thread composer = %q, want empty", got)
    }

    m.composer.SetValue("reply text")
    updated, _ = m.handleThreadKey(draftKey("esc"))
    m = updated.(Model)
    if got := m.composer.Value(); got != "channel text" {
        t.Fatalf("channel draft restored after closing thread = %q", got)
    }

    updated, _ = m.openSelectedThread()
    m = updated.(Model)
    if got := m.composer.Value(); got != "reply text" {
        t.Fatalf("thread draft restored = %q", got)
    }
}

func TestChannelAndThreadDraftsAreIsolated(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.drafts[channelDraftKey("dev")] = "channel text"
    m.drafts[threadDraftKey("dev", "root")] = "reply text"

    m.loadDraft(channelDraftKey("dev"))
    if got := m.composer.Value(); got != "channel text" {
        t.Fatalf("channel composer = %q", got)
    }
    m.loadDraft(threadDraftKey("dev", "root"))
    if got := m.composer.Value(); got != "reply text" {
        t.Fatalf("thread composer = %q", got)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestThreadDraft|TestChannelAndThreadDrafts' -count=1
```

Expected: FAIL because thread open/close does not switch draft keys.

**Step 3: Implement thread draft switching**

In `openSelectedThread()`:

- before setting `threadOpen`, call `m.saveActiveDraft()`;
- after setting `threadRootID`, call:

```go
m.loadDraft(threadDraftKey(m.currentChannelID(), rootID))
```

In `handleThreadKey` `esc` branch:

- before clearing thread fields, call `m.saveActiveDraft()`;
- after clearing `threadOpen/threadRootID/threadPosts`, restore current channel draft:

```go
m.loadDraft(channelDraftKey(m.currentChannelID()))
```

In pending triage/pending jump thread load branch in `postsLoadedMsg`, after `m.threadRootID = pendingThreadID`, call:

```go
m.saveActiveDraft()
m.loadDraft(threadDraftKey(msg.channelID, pendingThreadID))
```

Ensure `threadFocusComposer` behavior remains unchanged.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestThreadDraft|TestChannelAndThreadDrafts|TestThread|TestFocus' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/model.go internal/app/drafts_test.go
git commit -m "feat: preserve thread drafts"
```

---

### Task 4: Restore failed channel sends

**Files:**
- Modify: `internal/app/commands.go:39-49,164-168`
- Modify: `internal/app/model.go:311-325,650-659`
- Modify: `internal/app/drafts.go`
- Test: modify `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestFailedChannelSendRestoresDraft(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.loadDraft(channelDraftKey("dev"))
    m.composer.SetValue("hello")

    updated, _ := m.handleKey(draftKey("enter"))
    m = updated.(Model)
    if got := m.composer.Value(); got != "" {
        t.Fatalf("composer should clear while sending, got %q", got)
    }

    key := channelDraftKey("dev")
    updated, _ = m.Update(postSentMsg{channelID: "dev", draftKey: key, text: "hello", err: assertErr{}})
    got := updated.(Model)
    if got.composer.Value() != "hello" {
        t.Fatalf("failed send restored composer = %q", got.composer.Value())
    }
    if got.drafts[key] != "hello" {
        t.Fatalf("failed send restored draft = %q", got.drafts[key])
    }
    if got.status != "send failed · draft restored" {
        t.Fatalf("status = %q", got.status)
    }
}

func TestSuccessfulChannelSendClearsOnlySentDraft(t *testing.T) {
    key := channelDraftKey("dev")
    other := channelDraftKey("alerts")
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.loadDraft(key)
    m.drafts[key] = "hello"
    m.drafts[other] = "keep me"
    m.pendingSends[key] = "hello"

    updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: key, text: "hello", post: domain.Post{ID: "p1", ChannelID: "dev", Message: "hello"}})
    got := updated.(Model)
    if _, ok := got.drafts[key]; ok {
        t.Fatalf("sent draft should clear: %#v", got.drafts)
    }
    if got.drafts[other] != "keep me" {
        t.Fatalf("other draft lost: %#v", got.drafts)
    }
}
```

Add test helper:

```go
type assertErr struct{}
func (assertErr) Error() string { return "boom" }
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestFailedChannelSendRestoresDraft|TestSuccessfulChannelSendClearsOnlySentDraft' -count=1
```

Expected: FAIL because `postSentMsg` does not carry draft key/text and failure does not restore text.

**Step 3: Implement pending send helpers**

In `drafts.go` add:

```go
func (m *Model) beginPendingSend(key, text string) {
    if key == "" {
        return
    }
    if m.pendingSends == nil {
        m.pendingSends = map[string]string{}
    }
    m.pendingSends[key] = text
    delete(m.drafts, key)
    if key == m.activeDraftKey {
        m.composer.Reset()
    }
}

func (m *Model) completePendingSend(key string) {
    if key == "" {
        return
    }
    delete(m.pendingSends, key)
    delete(m.drafts, key)
    if key == m.activeDraftKey {
        m.composer.Reset()
    }
}

func (m *Model) restorePendingSend(key, text string) {
    if key == "" {
        return
    }
    if text == "" {
        text = m.pendingSends[key]
    }
    delete(m.pendingSends, key)
    if strings.TrimSpace(text) == "" {
        return
    }
    if m.drafts == nil {
        m.drafts = map[string]string{}
    }
    m.drafts[key] = text
    if key == m.activeDraftKey {
        m.composer.SetValue(text)
    }
}
```

**Step 4: Thread draft metadata through channel send**

Update `postSentMsg` and `sendPostCmd`:

```go
type postSentMsg struct {
    channelID string
    draftKey  string
    text      string
    post      domain.Post
    err       error
}

func sendPostCmd(ctx context.Context, backend domain.Backend, channelID, draftKey, text string) tea.Cmd {
    return func() tea.Msg {
        post, err := backend.SendPost(ctx, channelID, text)
        return postSentMsg{channelID: channelID, draftKey: draftKey, text: text, post: post, err: err}
    }
}
```

Update channel `enter` handling:

```go
text := strings.TrimSpace(m.composer.Value())
key := m.currentDraftKey()
m.beginPendingSend(key, text)
m.status = "sending…"
m.loading = true
return m, sendPostCmd(m.ctx, m.backend, m.currentChannelID(), key, text)
```

Update `postSentMsg` handling:

```go
if msg.err != nil {
    m.err = msg.err.Error()
    m.restorePendingSend(msg.draftKey, msg.text)
    m.status = "send failed · draft restored"
    m.loading = false
    return m, nil
}
m.completePendingSend(msg.draftKey)
```

If response channel no longer matches `currentChannelID()`, still complete/restore the pending send by key before returning.

**Step 5: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestFailedChannelSendRestoresDraft|TestSuccessfulChannelSendClearsOnlySentDraft|TestDraft' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/app/commands.go internal/app/model.go internal/app/drafts.go internal/app/drafts_test.go
git commit -m "feat: restore failed channel sends"
```

---

### Task 5: Restore failed thread replies

**Files:**
- Modify: `internal/app/commands.go:51-56,171-175`
- Modify: `internal/app/model.go:292-309,921-929`
- Test: modify `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestFailedThreadReplyRestoresDraft(t *testing.T) {
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.threadOpen = true
    m.threadRootID = "root"
    key := threadDraftKey("dev", "root")
    m.loadDraft(key)
    m.composer.SetValue("reply text")

    updated, _ := m.handleThreadKey(draftKey("enter"))
    m = updated.(Model)
    if got := m.composer.Value(); got != "" {
        t.Fatalf("composer should clear while sending reply, got %q", got)
    }

    updated, _ = m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: key, text: "reply text", err: assertErr{}})
    got := updated.(Model)
    if got.composer.Value() != "reply text" {
        t.Fatalf("failed reply restored composer = %q", got.composer.Value())
    }
    if got.status != "reply failed · draft restored" {
        t.Fatalf("status = %q", got.status)
    }
}

func TestSuccessfulThreadReplyClearsOnlyThreadDraft(t *testing.T) {
    threadKey := threadDraftKey("dev", "root")
    channelKey := channelDraftKey("dev")
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.threadOpen = true
    m.threadRootID = "root"
    m.loadDraft(threadKey)
    m.drafts[threadKey] = "reply"
    m.drafts[channelKey] = "channel"
    m.pendingSends[threadKey] = "reply"

    updated, _ := m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadKey, text: "reply", post: domain.Post{ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply"}})
    got := updated.(Model)
    if _, ok := got.drafts[threadKey]; ok {
        t.Fatalf("sent thread draft should clear: %#v", got.drafts)
    }
    if got.drafts[channelKey] != "channel" {
        t.Fatalf("channel draft lost: %#v", got.drafts)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestFailedThreadReplyRestoresDraft|TestSuccessfulThreadReplyClearsOnlyThreadDraft' -count=1
```

Expected: FAIL because `replySentMsg` does not carry draft metadata and failure does not restore.

**Step 3: Implement reply pending send flow**

Update `replySentMsg` and `sendReplyCmd`:

```go
type replySentMsg struct {
    channelID string
    rootID    string
    draftKey  string
    text      string
    post      domain.Post
    err       error
}

func sendReplyCmd(ctx context.Context, backend domain.Backend, channelID, rootID, draftKey, text string) tea.Cmd {
    return func() tea.Msg {
        post, err := backend.SendReply(ctx, channelID, rootID, text)
        return replySentMsg{channelID: channelID, rootID: rootID, draftKey: draftKey, text: text, post: post, err: err}
    }
}
```

Update thread composer `enter`:

```go
text := strings.TrimSpace(m.composer.Value())
key := m.currentDraftKey()
m.beginPendingSend(key, text)
m.status = "sending reply…"
return m, sendReplyCmd(m.ctx, m.backend, m.currentChannelID(), m.threadRootID, key, text)
```

Update `replySentMsg` handling:

```go
if msg.err != nil {
    m.err = msg.err.Error()
    m.restorePendingSend(msg.draftKey, msg.text)
    m.status = "reply failed · draft restored"
    return m, nil
}
m.completePendingSend(msg.draftKey)
```

If the reply response is for a root that is no longer open, still complete/restore the draft key before returning.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestFailedThreadReplyRestoresDraft|TestSuccessfulThreadReplyClearsOnlyThreadDraft|TestDraft' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/commands.go internal/app/model.go internal/app/drafts_test.go
git commit -m "feat: restore failed thread replies"
```

---

### Task 6: Handle stale/out-of-order send responses safely

**Files:**
- Modify: `internal/app/model.go:292-325`
- Test: modify `internal/app/drafts_test.go`

**Step 1: Write the failing tests**

Append:

```go
func TestFailedSendForInactiveChannelDoesNotOverwriteCurrentComposer(t *testing.T) {
    devKey := channelDraftKey("dev")
    alertsKey := channelDraftKey("alerts")
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{
        {ID: "dev", Type: "O", DisplayName: "dev"},
        {ID: "alerts", Type: "O", DisplayName: "alerts"},
    }
    m.selectedChannel = 1
    m.loadDraft(alertsKey)
    m.composer.SetValue("current alerts text")
    m.pendingSends[devKey] = "failed dev text"

    updated, _ := m.Update(postSentMsg{channelID: "dev", draftKey: devKey, text: "failed dev text", err: assertErr{}})
    got := updated.(Model)
    if got.composer.Value() != "current alerts text" {
        t.Fatalf("inactive failure overwrote current composer: %q", got.composer.Value())
    }
    if got.drafts[devKey] != "failed dev text" {
        t.Fatalf("inactive failed draft not stored: %#v", got.drafts)
    }
}

func TestFailedReplyForInactiveThreadDoesNotOverwriteChannelComposer(t *testing.T) {
    threadKey := threadDraftKey("dev", "root")
    channelKey := channelDraftKey("dev")
    m := New(nil, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
    m.selectedChannel = 0
    m.threadOpen = false
    m.loadDraft(channelKey)
    m.composer.SetValue("channel text")
    m.pendingSends[threadKey] = "failed reply"

    updated, _ := m.Update(replySentMsg{channelID: "dev", rootID: "root", draftKey: threadKey, text: "failed reply", err: assertErr{}})
    got := updated.(Model)
    if got.composer.Value() != "channel text" {
        t.Fatalf("inactive reply failure overwrote composer: %q", got.composer.Value())
    }
    if got.drafts[threadKey] != "failed reply" {
        t.Fatalf("inactive failed reply draft not stored: %#v", got.drafts)
    }
}
```

**Step 2: Run tests to verify they fail or expose behavior**

Run:

```bash
go test ./internal/app -run 'TestFailedSendForInactive|TestFailedReplyForInactive' -count=1
```

Expected: FAIL until response handlers restore only by draft key and only update composer when `draftKey == activeDraftKey`.

**Step 3: Fix response ordering**

Make `postSentMsg` and `replySentMsg` handling draft-first:

- on failure, always `restorePendingSend(msg.draftKey, msg.text)` before any channel/root current checks;
- on success, always `completePendingSend(msg.draftKey)` before current checks;
- only append post to current timeline/thread if the response still belongs to the visible destination.

This keeps backend responses from destroying the user's current composer text.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestFailedSendForInactive|TestFailedReplyForInactive|TestDraft' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/drafts_test.go
git commit -m "fix: isolate stale send responses"
```

---

### Task 7: Add mock send-failure support and VHS smoke

**Files:**
- Modify: `internal/mock/backend.go`
- Modify or create: `vhs/draft-safety.tape`
- Optional test: create `internal/mock/backend_test.go`

**Step 1: Write the failing mock test**

Create `internal/mock/backend_test.go` if needed:

```go
package mock

import (
    "context"
    "testing"
)

func TestSendFailureTrigger(t *testing.T) {
    b := New()
    _, err := b.SendPost(context.Background(), "dev", "fail-send")
    if err == nil {
        t.Fatal("expected fail-send to trigger mock send failure")
    }
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/mock -run TestSendFailureTrigger -count=1
```

Expected: FAIL until mock failure hook exists.

**Step 3: Implement minimal mock failure hook**

In `internal/mock/backend.go`, in `sendPost` / `sendReply` path, return an error for explicit test strings:

```go
if strings.TrimSpace(message) == "fail-send" {
    return domain.Post{}, fmt.Errorf("mock send failure")
}
```

Do not make random failures. Deterministic QA only.

**Step 4: Add VHS smoke**

Create `vhs/draft-safety.tape`:

```text
# band-tui draft safety smoke recording
# Run from repo root with: vhs vhs/draft-safety.tape

Output tmp/draft-safety.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

# Type a channel draft, switch away, return, and verify visually it remains.
Type "dev draft"
Sleep 500ms
Tab
Sleep 300ms
Down
Sleep 800ms
Up
Sleep 800ms
Tab
Tab
Sleep 500ms

# Trigger deterministic send failure and show restored text.
Ctrl+U
Type "fail-send"
Enter
Sleep 1s

Ctrl+C
```

Adjust key steps if focus order differs after implementation. The final GIF must visibly show restored text after failure.

**Step 5: Run tests and VHS**

Run:

```bash
go test ./internal/mock -run TestSendFailureTrigger -count=1
go test ./internal/app -run 'TestDraft|TestFailed|TestSuccessful' -count=1
vhs vhs/draft-safety.tape
```

Expected:

- tests PASS;
- `tmp/draft-safety.gif` created;
- GIF shows draft survives navigation and failed send restores text.

**Step 6: Commit**

```bash
git add internal/mock/backend.go internal/mock/backend_test.go vhs/draft-safety.tape
git commit -m "test: add draft safety smoke coverage"
```

---

### Task 8: Final verification

**Files:**
- No new source files required.

**Step 1: Run focused app tests**

```bash
go test ./internal/app -run 'TestDraft|TestFailed|TestSuccessful|TestThread|TestFocus' -count=1
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
vhs vhs/draft-safety.tape
```

Expected: `tmp/draft-safety.gif` exists and visually confirms:

- channel draft survives switching away/back;
- failed send restores text;
- status says draft was restored.

**Step 6: Review final diff**

```bash
git diff --stat HEAD
```

Expected: only draft-safety source/tests/docs/VHS changes beyond already-committed triage work.

---

## Notes for implementer

- The current single shared composer is intentional. Do not introduce separate channel/thread textareas.
- Save drafts silently; noisy “draft saved” status will make navigation feel broken.
- Prefer destination-key helpers over ad hoc string concatenation at call sites.
- Keep pending send restoration keyed by original destination. Backend responses are asynchronous and may arrive after the user navigates elsewhere.
- Failed sends should restore text, not append to existing current composer text unless the failed destination is active and currently empty because it was just sent.
- If a test needs a key event helper, reuse or duplicate a local `tea.KeyMsg` helper in the test file; do not export test-only production helpers.
