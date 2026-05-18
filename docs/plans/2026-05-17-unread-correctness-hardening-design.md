# Unread Correctness Hardening Design

## Goal

Make unread channels, mentions, thread replies, navigation, and triage behave as one consistent system: the same underlying signal should appear once, clear once, and never resurrect after the user has already seen or opened it.

## Problem

Today the app mostly works, but the “important state” contract is still spread across several partially-overlapping mechanisms:

- channel counters live in `channels[].Unread` and `channels[].Mentions`;
- per-post importance lives in `Post.Unread`, `Post.Mentioned`, and `Post.ThreadUnread`;
- live mention activity also lives in `recentEvents`;
- triage rebuilds from all of the above;
- `n/N`, `initialSelectedPost`, message markers, sidebar badges, and thread opening all depend on slightly different combinations of those flags.

That creates classic reliability bugs:

- clearing one representation but not another;
- live websocket updates incrementing counters one way and post flags another way;
- thread opens clearing thread state differently from channel opens;
- triage or unread navigation resurrecting a signal the user already consumed.

The root problem is not missing if-statements. It is the absence of a single local contract for “important state”.

## Chosen Direction

Use **one normalization layer inside `internal/app`**.

Not a new backend pipeline, not “reload from server on every step”, and not a full new store. The app keeps the existing state shape (`channels`, `posts`, `postsByChannel`, `threadPosts`, `recentEvents`, triage), but all read/unread/thread mutations must flow through a small set of normalization helpers.

This is the least risky option because:

- it preserves the current fast keyboard-first local UX;
- it does not invent new persistent state;
- it gives us a single place to reason about invariants and test them.

## Core Contract

A signal is important if it represents one of three things:

1. **mention** — personal or broadcast attention directed at the user;
2. **unread message** — unread channel-level work not already represented by a stronger signal;
3. **thread unread** — unread work inside a thread.

The contract must satisfy these invariants:

### Invariant 1: one source mutation path

Any code path that changes important state must call a normalization helper, not hand-edit counters/flags directly.

That includes:

- opening a channel;
- opening a thread;
- receiving a live websocket post;
- loading posts/threads from backend;
- sending a reply that changes local thread state.

### Invariant 2: stronger signal dominates weaker signal, but only for the same work

Mention > thread reply > unread channel for the same underlying work.

But a stronger signal may only suppress a weaker one if it covers the same unread work. Different unread work must remain visible.

### Invariant 3: once consumed, consumed everywhere

If the user opens a channel or thread and consumes that work, every representation of the same work must clear together:

- per-post flags;
- channel counters;
- thread root signal;
- triage item;
- unread navigation target.

### Invariant 4: local navigation and visual markers agree

`initialSelectedPost`, `n/N`, selected unread jump, timeline markers, sidebar badges, and triage must all point at the same remaining important posts.

## Proposed Architecture

Keep existing stored fields, but add a narrow set of mutation/derivation helpers.

### Mutation helpers

Introduce helpers in `model.go` or a new `important_state.go`:

```go
func (m *Model) applyChannelRead(channelID string)
func (m *Model) applyThreadRead(channelID, rootID string)
func (m *Model) applyIncomingPost(post domain.Post, source incomingPostSource)
func (m *Model) reconcileChannelImportance(channelID string)
func (m *Model) reconcileAllImportance()
```

Optional enum:

```go
type incomingPostSource int
```

with values like:
- backend load
- websocket current channel
- websocket background channel
- local send
- local reply

The point is not abstraction for its own sake. The point is to route all important-state mutation through a tiny, testable API.

### Derivation helpers

Add read-only helpers to answer “what is important now?” consistently:

```go
func importantPosts(posts []domain.Post) []int
func firstImportantPost(posts []domain.Post) int
func postImportance(post domain.Post) importantKind
func threadUnreadWork(posts []domain.Post, rootID string) int
```

`initialSelectedPost`, `selectRelativeImportantPost`, timeline markers, and triage coverage logic should use these helpers instead of re-encoding priority rules in multiple places.

## What Changes Conceptually

### Channel read

Opening a channel should not just zero counters. It should mean:

- all visible/current channel unread and mention post flags clear;
- cached channel post flags clear consistently;
- any thread root signals already represented in that channel clear if the user is now looking at the relevant work;
- channel counters are then reconciled from the remaining important posts, not guessed.

### Thread read

Opening a thread should not perform ad hoc partial clearing. It should mean:

- all important flags for that thread clear in `threadPosts`, `posts`, and `postsByChannel`;
- the containing channel counters are reconciled from what remains outside that consumed thread;
- triage for that thread disappears unless newer unread work arrives.

### Incoming websocket post

A live post should be applied once, then normalized:

- background channel post => mark unread;
- mention activity => mark mention + unread consistently;
- thread reply outside visible thread => mark thread unread and update channel unread consistently;
- reply inside open thread => visible local append, no phantom unread resurrection.

This is where many current edge cases originate, so centralizing it is the highest-value change.

## Triage Relationship

Triage should remain a pure projection, not become the source of truth.

That means the builder in `triage.go` may stay pure, but it must read normalized post/channel state whose invariants are already enforced. The hardening cycle should reduce triage-specific patches because the underlying state becomes coherent.

In practice:

- `buildTriageItems` continues to derive mention/thread/unread rows;
- dedupe rules stay, but they operate over a cleaner contract;
- fewer “special-case because markChannelRead forgot X” bugs survive.

## Testing Strategy

This cycle should be more test-heavy than UI-heavy.

### Regression tests first

Before refactoring helpers, encode the bugs we care about:

- opening a thread clears the same work from channel counters and triage;
- live reply in visible thread does not create phantom unread;
- live reply in background channel does create one coherent thread/unread signal;
- `n/N` and `initialSelectedPost` pick the same next important post as triage would imply;
- clearing a channel or thread never allows old cached flags to resurrect after reload.

### Invariant tests

Then add explicit invariant tests such as:

- channel counters equal the remaining normalized important work;
- consumed thread does not still contribute unread/mention counts;
- stronger-vs-weaker triage suppression only removes same-work duplicates.

## Non-Goals

Not in this cycle:

- backend mark-read protocol changes;
- server-side thread-state redesign;
- durable triage dismissals;
- attachment/file unread semantics;
- new inbox UX.

## Recommended Next Slice

Implement this hardening in four moves:

1. add regression tests around current unread/thread/triage inconsistencies;
2. introduce normalization helpers and route read paths through them;
3. route websocket/live-post mutations through one helper;
4. align triage and unread navigation to the normalized contract.

That sequence attacks the root problem without turning this into a full architecture rewrite.
