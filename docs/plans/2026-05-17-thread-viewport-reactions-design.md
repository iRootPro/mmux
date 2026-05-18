# Thread Viewport Reactions Design

## Goal

Extend reactions into the thread panel by adding message selection inside the thread viewport and reusing the compact reaction picker for the selected thread message.

## Problem

We now support reactions on timeline messages, but the thread panel still treats messages as a scroll-only viewport. That creates an obvious inconsistency:

- timeline messages can be reacted to;
- thread replies cannot;
- there is no way to target a specific thread message for any future action because the thread panel has no selected-message model.

The real missing primitive is not “reaction support” by itself. It is **thread message selection**.

## Chosen Slice

Scope for this step:

- add selected-message navigation inside thread messages viewport
- reuse the existing compact reaction picker on the selected thread message
- render reaction badges in thread posts too
- keep composer reply flow intact

Not in scope:

- quote/edit/delete from thread viewport
- emoji search
- user list popovers for reactions
- multi-action thread context menu

## UX Contract

### Thread messages mode

When `threadOpen == true` and `threadFocusComposer == false`, the thread panel becomes a selectable message list, not only a raw scroller.

Behavior:
- `j/k` or arrows move the selected thread message
- `home/end` jump to first/last thread message
- viewport scroll follows the selection
- selected thread message gets a visible marker, parallel to timeline selection

The root post should be selectable too.

### Reaction action

On a selected thread message:
- `R` opens the same compact reaction picker used in timeline
- picker behavior stays identical:
  - `j/k`, arrows
  - `enter` toggle
  - `esc` close

The only difference is the **reaction target source**:
- timeline picker targets `m.posts[m.selectedPost]`
- thread picker targets `m.threadPosts[m.threadSelected]`

### Composer interaction

Reply composer stays as-is:
- `tab` switches between thread messages and thread composer
- `enter` in composer still sends a reply
- reactions only work while in thread messages mode, not while composing

## New State

Add minimal selection state to `Model`:

```go
threadSelected int
```

This should represent an index into `m.threadPosts` when `threadOpen` is true.

Rules:
- default selection after loading a thread should be the last visible reply if there are replies, otherwise the root post;
- clamp when thread posts change;
- reset when thread closes.

## Architecture

Do **not** fork the reaction picker. Reuse the existing picker model and command path.

The only addition needed is a notion of “current reaction target”:

```go
func (m Model) selectedReactionTarget() (domain.Post, bool)
```

Behavior:
- if reaction picker was opened from thread messages, return `m.threadPosts[m.threadSelected]`
- otherwise return timeline selected post

You can model this either by:
1. explicit picker context enum (`reactionPickerTimeline`, `reactionPickerThread`), or
2. storing the target post ID when opening the picker.

I recommend **store target post ID + source kind**. It is more robust if lists reorder.

Example:

```go
type reactionTargetKind int

const (
    reactionTargetTimeline reactionTargetKind = iota
    reactionTargetThread
)

reactionTargetKind reactionTargetKind
reactionTargetPostID string
```

This prevents the picker from depending on mutable indexes after opening.

## Local Update Rules

On successful reaction toggle from thread viewport:
- update `threadPosts`
- update `postsByChannel`
- if the reacted post is also present in `m.posts` (for example a thread root visible in timeline), update `m.posts`

This symmetry matters: thread and timeline should not diverge.

## Rendering

Thread posts should render reaction badges in the same compact style as timeline posts.

Selection should be visually similar to timeline selection, but adapted to the thread panel’s narrower layout.

Recommended behavior:
- selected thread post gets the same leading marker style as timeline selected post
- reaction badges render immediately below the message body when present

## Testing Strategy

### Selection behavior

- initial thread selection after load
- `j/k` moves selected thread post
- selection clamps at edges
- selection resets/closes with thread

### Reaction behavior

- `R` from thread messages opens picker
- picker targets selected thread post, not timeline post
- successful toggle updates `threadPosts` and any mirrored copies
- failure leaves local state unchanged

### Render behavior

- selected thread post is visibly marked
- thread post reaction badges render

## Non-Goals

Not in this slice:

- quote/edit/delete from thread viewport
- arbitrary emoji input
- thread message permalink actions
- generalized thread action framework

## Recommended Follow-Up

Once thread selection exists, the next natural upgrades are:

1. quote selected thread message into thread reply composer or channel composer
2. edit/delete own thread replies
3. other per-thread message actions using the same selection primitive
