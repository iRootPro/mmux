# Unread Correctness Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make channel unread counts, per-post importance flags, triage items, and unread navigation behave as one consistent local system.

**Architecture:** Keep the current state shape, but route all important-state mutations through a small normalization layer inside `internal/app`. Reconcile counters and post flags from the same rules so channel opens, thread opens, websocket posts, triage, and `n/N` all observe the same contract.

**Tech Stack:** Go, Bubble Tea, existing `internal/app` model/triage/view architecture, current Mattermost/mock backends, Go test.

---

## Prerequisites

Before implementation:

- Start from a clean tree after the connection/session reliability commits.
- Read design: `docs/plans/2026-05-17-unread-correctness-hardening-design.md`.
- Relevant existing code:
  - `internal/app/model.go:1674-1992`
  - `internal/app/actions.go:42-66`
  - `internal/app/triage.go:35-607`
  - `internal/app/unread_navigation_test.go`
  - `internal/app/read_test.go`
  - `internal/app/triage_test.go`
  - `internal/app/activity_test.go`

Implementation rules:

- Use `@superpowers:test-driven-development` for every behavior change.
- Do not add a new backend API or new persistence.
- Do not make triage the source of truth.
- Prefer deleting direct flag/counter mutations and replacing them with normalization helpers.
- Keep the scope on correctness and invariants, not UI redesign.

---

### Task 1: Lock down unread/thread/triage regressions with tests

**Files:**
- Modify: `internal/app/read_test.go`
- Modify: `internal/app/unread_navigation_test.go`
- Modify: `internal/app/triage_test.go`
- Optional create: `internal/app/important_state_test.go`

**Step 1: Write the failing tests**

Add focused regression coverage for the hardening cycle.

In `internal/app/read_test.go` append tests like:

```go
func TestClearThreadReadSignalReconcilesChannelUnreadAndMentions(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Unread: 3, Mentions: 1}},
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 2},
                {ID: "reply-1", ChannelID: "dev", RootID: "root", Unread: true},
                {ID: "reply-2", ChannelID: "dev", RootID: "root", Mentioned: true},
                {ID: "other", ChannelID: "dev", Unread: true},
            },
        },
    }

    m.clearThreadReadSignal("dev", "root")

    if m.channels[0].Unread != 1 || m.channels[0].Mentions != 0 {
        t.Fatalf("channel counters not reconciled: %#v", m.channels[0])
    }
}

func TestMarkChannelReadDoesNotLeaveBackgroundThreadSignalInCurrentChannel(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Unread: 2, Mentions: 1}},
        posts: []domain.Post{
            {ID: "root", ChannelID: "dev", ThreadUnread: true},
            {ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
        },
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "root", ChannelID: "dev", ThreadUnread: true},
                {ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, Mentioned: true},
            },
        },
    }

    m.markChannelRead("dev")

    if m.channels[0].Unread != 0 || m.channels[0].Mentions != 0 {
        t.Fatalf("channel not cleared: %#v", m.channels[0])
    }
    for _, p := range m.postsByChannel["dev"] {
        if p.Unread || p.Mentioned || p.ThreadUnread {
            t.Fatalf("stale important flags remain: %#v", m.postsByChannel["dev"])
        }
    }
}
```

In `internal/app/unread_navigation_test.go` append:

```go
func TestInitialSelectedPostAndUnreadNavigationUseSamePriority(t *testing.T) {
    m := Model{posts: []domain.Post{
        {ID: "a"},
        {ID: "b", ThreadUnread: true},
        {ID: "c", Unread: true},
        {ID: "d", Mentioned: true},
    }}

    if got := m.initialSelectedPost("c1"); got != 3 {
        t.Fatalf("initial selected = %d", got)
    }

    m.selectedPost = 0
    updated, _ := m.selectRelativeImportantPost(1)
    got := updated.(Model)
    if got.selectedPost != 1 {
        t.Fatalf("first jump selected = %d", got.selectedPost)
    }
}
```

In `internal/app/triage_test.go` append:

```go
func TestBuildTriageItemsDoesNotResurrectThreadAfterThreadOpenClearsSameWork(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1, CreateAt: 100},
                {ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, ThreadUnread: true, CreateAt: 200},
            },
        },
    }

    m.clearThreadReadSignal("dev", "root")
    items := buildTriageItems(m)
    if len(items) != 0 {
        t.Fatalf("cleared thread work should not resurrect in triage: %#v", items)
    }
}
```

Keep test names explicit and behavior-focused.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestClearThreadReadSignalReconcilesChannelUnreadAndMentions|TestInitialSelectedPostAndUnreadNavigationUseSamePriority|TestBuildTriageItemsDoesNotResurrectThreadAfterThreadOpenClearsSameWork' -count=1
```

Expected: at least one FAIL that exposes current inconsistency.

**Step 3: Do not implement yet**

This task is red-first only. Once failures are real and meaningful, stop and move to Task 2.

**Step 4: Commit the failing tests only if your workflow explicitly wants a red checkpoint**

Normally skip commit here and continue immediately to Task 2.

---

### Task 2: Introduce normalization helpers for important state mutations

**Files:**
- Create: `internal/app/important_state.go`
- Modify: `internal/app/model.go:1855-1992`
- Test: modify `internal/app/read_test.go`
- Test: optional `internal/app/important_state_test.go`

**Step 1: Write the failing unit tests for helpers**

Add direct helper tests:

```go
func TestReconcileChannelImportanceCountsRemainingSignals(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev"}},
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "a", ChannelID: "dev", Unread: true},
                {ID: "b", ChannelID: "dev", Mentioned: true},
                {ID: "root", ChannelID: "dev", ThreadUnread: true},
            },
        },
    }

    m.reconcileChannelImportance("dev")
    if m.channels[0].Unread != 2 || m.channels[0].Mentions != 1 {
        t.Fatalf("reconciled counters = %#v", m.channels[0])
    }
}
```

Desired counting rule for this cycle:

- `Unread` counts posts with `Unread` plus thread roots with `ThreadUnread` when no explicit unread reply remains;
- `Mentions` counts posts with `Mentioned`.

That rule must be encoded once, not guessed at call sites.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestReconcileChannelImportanceCountsRemainingSignals|TestClearThreadReadSignalReconcilesChannelUnreadAndMentions' -count=1
```

Expected: FAIL because reconcile helper does not exist or current logic disagrees.

**Step 3: Implement normalization helpers**

Create `internal/app/important_state.go` with helpers like:

```go
package app

import "band-tui/internal/domain"

func (m *Model) applyChannelRead(channelID string)
func (m *Model) applyThreadRead(channelID, rootID string)
func (m *Model) reconcileChannelImportance(channelID string)
func (m *Model) reconcileAllImportance()
func importantPost(post domain.Post) bool
func mentionPost(post domain.Post) bool
```

Implementation guidance:

- `applyChannelRead(channelID)` replaces the semantic role of `markChannelRead`; it clears important flags for that channel, then reconciles counters to zero.
- `applyThreadRead(channelID, rootID)` replaces the semantic role of `clearThreadReadSignal`; it clears important flags for that thread in `posts`, `postsByChannel`, and `threadPosts` if loaded, then calls `reconcileChannelImportance(channelID)`.
- `reconcileChannelImportance(channelID)` recomputes `Unread`/`Mentions` from remaining normalized post flags in `postsByChannel[channelID]`.

Update `markChannelRead` and `clearThreadReadSignal` to delegate to the new helpers or delete them entirely if no longer needed.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestReconcileChannelImportanceCountsRemainingSignals|TestClearThreadReadSignalReconcilesChannelUnreadAndMentions|TestMarkChannelReadDoesNotLeaveBackgroundThreadSignalInCurrentChannel' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/important_state.go internal/app/model.go internal/app/read_test.go internal/app/important_state_test.go
git commit -m "feat: normalize unread state mutations"
```

---

### Task 3: Route channel/thread open paths through the normalization layer

**Files:**
- Modify: `internal/app/model.go:229-256,1225-1245,1886-1947`
- Modify: `internal/app/actions.go:12-34`
- Modify: `internal/app/triage.go:166-203`
- Test: modify `internal/app/read_test.go`
- Test: modify `internal/app/triage_test.go`

**Step 1: Write the failing tests**

Add tests that exercise end-to-end consumption semantics:

```go
func TestOpenCurrentChannelClearsChannelImportantStateOnce(t *testing.T) {
    m := New(noopBackend{}, testConfig(), false)
    m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2, Mentions: 1}}
    m.selectedChannel = 0
    m.postsByChannel = map[string][]domain.Post{
        "dev": {
            {ID: "a", ChannelID: "dev", Unread: true},
            {ID: "b", ChannelID: "dev", Mentioned: true},
        },
    }

    updated, _ := m.openCurrentChannel()
    got := updated.(Model)
    if got.channels[0].Unread != 0 || got.channels[0].Mentions != 0 {
        t.Fatalf("openCurrentChannel did not clear important state: %#v", got.channels[0])
    }
}

func TestOpenTriageThreadClearsSameThreadWorkEverywhere(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 1}},
        selectedChannel: 0,
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "root", ChannelID: "dev", ThreadUnread: true, ReplyCount: 1, CreateAt: 100},
                {ID: "reply", ChannelID: "dev", RootID: "root", Unread: true, ThreadUnread: true, CreateAt: 200},
            },
        },
        triageOpen: true,
        triageItems: []triageItem{{Kind: triageThreadReply, ChannelID: "dev", RootID: "root", PostID: "reply"}},
    }

    updated, _ := m.handleTriageKey(draftKey("enter"))
    got := updated.(Model)
    if got.channels[0].Unread != 0 || len(got.triageItems) != 0 {
        t.Fatalf("thread open did not clear same work: channel=%#v triage=%#v", got.channels[0], got.triageItems)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestOpenCurrentChannelClearsChannelImportantStateOnce|TestOpenTriageThreadClearsSameThreadWorkEverywhere' -count=1
```

Expected: FAIL until the open paths consistently use the normalization helpers.

**Step 3: Implement route-through changes**

Update:

- `openCurrentChannel()` to call `applyChannelRead(channelID)` instead of ad hoc clearing.
- `postsLoadedMsg` current-channel path to avoid re-clearing through a second divergent path.
- `openSelectedThread()` and `openTriageThread()` to call `applyThreadRead(channelID, rootID)` when the user is explicitly consuming that thread.
- any remaining `markChannelRead` / `clearThreadReadSignal` call sites should either delegate cleanly or disappear.

The outcome should be: opening a channel/thread consumes that work exactly once and every representation agrees.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestOpenCurrentChannelClearsChannelImportantStateOnce|TestOpenTriageThreadClearsSameThreadWorkEverywhere|TestBuildTriageItemsDoesNotResurrectThreadAfterThreadOpenClearsSameWork' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/actions.go internal/app/triage.go internal/app/read_test.go internal/app/triage_test.go
git commit -m "feat: consume unread state through one path"
```

---

### Task 4: Route websocket/live-post updates through one helper

**Files:**
- Modify: `internal/app/model.go:345-415`
- Modify: `internal/app/activity_test.go`
- Modify: `internal/app/read_test.go`
- Optional create: `internal/app/live_importance_test.go`

**Step 1: Write the failing tests**

Add tests for live post application:

```go
func TestLiveReplyInVisibleThreadDoesNotCreateUnreadOrTriage(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}},
        selectedChannel: 0,
        threadOpen: true,
        threadRootID: "root",
        threadPosts: []domain.Post{{ID: "root", ChannelID: "dev", Message: "root"}},
        postsByChannel: map[string][]domain.Post{"dev": {{ID: "root", ChannelID: "dev", Message: "root"}}},
        events: make(chan domain.Event),
    }

    updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "r1", ChannelID: "dev", RootID: "root", Message: "reply"}}})
    got := updated.(Model)
    if got.channels[0].Unread != 0 || len(got.triageItems) != 0 {
        t.Fatalf("visible thread reply should not create background unread/triage: %#v %#v", got.channels[0], got.triageItems)
    }
}

func TestLiveReplyInBackgroundChannelCreatesCoherentUnreadSignal(t *testing.T) {
    m := Model{
        channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}, {ID: "alerts", Type: "O", DisplayName: "alerts"}},
        selectedChannel: 0,
        postsByChannel: map[string][]domain.Post{"alerts": {{ID: "root", ChannelID: "alerts", Message: "root"}}},
        events: make(chan domain.Event),
    }

    updated, _ := m.Update(backendEventMsg{event: domain.Event{Kind: domain.EventPost, Post: domain.Post{ID: "r1", ChannelID: "alerts", RootID: "root", Message: "reply"}}})
    got := updated.(Model)
    if got.channels[1].Unread == 0 || len(got.triageItems) == 0 {
        t.Fatalf("background reply should create coherent unread signal: %#v %#v", got.channels[1], got.triageItems)
    }
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestLiveReplyInVisibleThreadDoesNotCreateUnreadOrTriage|TestLiveReplyInBackgroundChannelCreatesCoherentUnreadSignal' -count=1
```

Expected: FAIL until websocket post mutation uses one contract.

**Step 3: Implement live-post normalization helper**

Add helper:

```go
func (m *Model) applyIncomingPost(post domain.Post, visibleThread bool, mentionActivity bool)
```

Use it from the `backendEventMsg` `EventPost` branch.

Rules:

- visible current-thread reply => append locally, no new unread counter/triage work;
- background reply => mark thread unread and reconcile channel importance coherently;
- current-channel non-thread post => visible append and read-state clear;
- background mention => mention + unread applied together.

This helper should own the mutation order instead of spreading it across the switch branch.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestLiveReplyInVisibleThreadDoesNotCreateUnreadOrTriage|TestLiveReplyInBackgroundChannelCreatesCoherentUnreadSignal|TestLivePostInCurrentChannelSendsViewChannel' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/model.go internal/app/activity_test.go internal/app/read_test.go internal/app/live_importance_test.go
git commit -m "feat: normalize live unread updates"
```

---

### Task 5: Align triage and unread navigation with normalized importance rules

**Files:**
- Modify: `internal/app/actions.go:42-66`
- Modify: `internal/app/model.go:1674-1705`
- Modify: `internal/app/triage.go:330-538`
- Modify: `internal/app/unread_navigation_test.go`
- Modify: `internal/app/triage_test.go`

**Step 1: Write the failing tests**

Add tests proving the same priority rules are used everywhere:

```go
func TestSelectRelativeImportantPostAndTriageAgreeOnRemainingWork(t *testing.T) {
    m := Model{
        posts: []domain.Post{
            {ID: "a"},
            {ID: "b", ThreadUnread: true},
            {ID: "c", Unread: true},
            {ID: "d", Mentioned: true},
        },
        channels: []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev", Unread: 2, Mentions: 1}},
        postsByChannel: map[string][]domain.Post{
            "dev": {
                {ID: "b", ChannelID: "dev", ThreadUnread: true},
                {ID: "c", ChannelID: "dev", Unread: true},
                {ID: "d", ChannelID: "dev", Mentioned: true},
            },
        },
    }

    items := buildTriageItems(m)
    if len(items) == 0 || items[0].Kind != triageMention {
        t.Fatalf("triage priority wrong: %#v", items)
    }
    if got := m.initialSelectedPost("dev"); got != 3 {
        t.Fatalf("initialSelectedPost priority wrong: %d", got)
    }
}
```

Also add a regression for multiple unread replies in one thread remaining one triage item and one navigation target class, not duplicated work.

**Step 2: Run tests to verify they fail or expose drift**

Run:

```bash
go test ./internal/app -run 'TestSelectRelativeImportantPostAndTriageAgreeOnRemainingWork|TestBuildTriageItemsDoesNotAddUnreadChannelForMultipleUnreadRepliesInSameThread' -count=1
```

Expected: FAIL or reveal inconsistent rule encoding.

**Step 3: Unify derivation helpers**

Add shared helpers such as:

```go
func importantPostKind(post domain.Post) triageKindLikePriority
func firstImportantPost(posts []domain.Post) int
```

Use them in:

- `initialSelectedPost`
- `selectRelativeImportantPost`
- triage unread/thread coverage logic where priority assumptions matter

Do not over-abstract; one shared priority contract is enough.

**Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/app -run 'TestSelectRelativeImportantPostAndTriageAgreeOnRemainingWork|TestInitialSelectedPostAndUnreadNavigationUseSamePriority|TestBuildTriageItemsDoesNotAddUnreadChannelForMultipleUnreadRepliesInSameThread' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/actions.go internal/app/model.go internal/app/triage.go internal/app/unread_navigation_test.go internal/app/triage_test.go
git commit -m "feat: align unread navigation with triage"
```

---

### Task 6: Final verification

**Files:**
- No new source files required.

**Step 1: Run focused unread correctness tests**

```bash
go test ./internal/app -run 'TestClearThreadReadSignal|TestMarkChannelRead|TestInitialSelectedPost|TestSelectRelativeImportantPost|TestBuildTriageItems|TestLiveReply' -count=1
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

**Step 5: Review final diff**

```bash
git diff --stat HEAD
```

Expected: only unread correctness hardening source/tests changes.

---

## Notes for implementer

- The objective is not to invent a new store. The objective is to stop mutating important state from five different places with five different rules.
- Prefer a small number of helpers that mutate existing state over new mirrored caches.
- If two functions need the same priority rule, encode it once.
- Triage is a projection. Fix the state beneath it first.
- Resist UI polish in this cycle unless a test forces it.
