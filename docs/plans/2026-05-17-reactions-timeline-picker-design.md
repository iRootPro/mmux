# Timeline Reaction Picker Design

## Goal

Add a fast, keyboard-first way to react to the selected timeline message with a compact picker, while keeping the first reaction slice small, local, and consistent with the rest of the TUI.

## Chosen Slice

Scope is intentionally narrow:

- **timeline focus only**
- **compact centered reaction picker overlay**
- **fixed reaction set** for v1
- **toggle semantics** (add if absent, remove if already reacted)
- **compact reaction badges rendered on messages**

Not in scope for this first slice:

- thread viewport reactions
- arbitrary emoji search
- full reaction user lists
- reaction management screens
- live websocket reaction sync if the existing event stream does not already provide it cleanly

## Product Shape

The user flow should feel like the rest of `band-tui`:

1. move to a message in the timeline;
2. press `R`;
3. choose an emoji with `j/k` or arrows;
4. press `enter` to toggle it;
5. see the message update immediately.

The picker closes immediately after a successful toggle. This keeps the action fast and minimizes modal friction.

### Fixed reaction set

For the first slice, I would use a short boring set:

- 👍
- 👀
- ✅
- ❤️
- 🎉

This avoids the complexity of emoji search/input and still covers the overwhelming majority of practical reactions.

## Data Model

Reactions should live on `domain.Post`, not in a global side map.

Recommended types:

```go
type PostReaction struct {
    Emoji   string
    Count   int
    Reacted bool
}
```

and in `domain.Post`:

```go
Reactions []PostReaction
```

Why this shape:

- timeline rendering becomes local and obvious;
- update/replace helpers can keep posts, cached channel copies, and thread copies consistent;
- no second reaction store needs to be synchronized with every post collection.

`Reacted` means the current user has already applied that emoji. That is enough for toggle behavior and UI highlighting.

## Backend Shape

Keep the backend transport layer explicit, not magical.

Add two methods to `domain.Backend`:

```go
AddReaction(ctx context.Context, postID, emoji string) (Post, error)
RemoveReaction(ctx context.Context, postID, emoji string) (Post, error)
```

The app layer decides whether a picker selection means add or remove.

Why not `ToggleReaction` in backend:

- backend should not guess UI intent;
- tests are simpler when add/remove are separate;
- the app already knows whether the selected post has `Reacted` for the chosen emoji.

## App State

Add minimal overlay state to `Model`:

```go
reactionPickerOpen     bool
reactionPickerSelected int
```

You do **not** need a full separate model or input widget. The picker is a small array selection overlay like the other lightweight popups.

Optional fixed reaction list helper:

```go
var defaultReactions = []string{"👍", "👀", "✅", "❤️", "🎉"}
```

## Toggle Flow

### Open

`R` in timeline focus:
- if no selected post => no-op;
- otherwise open picker and preselect index 0.

### Confirm

On `enter` in picker:
1. inspect selected post reactions;
2. if current user already reacted with chosen emoji => remove;
3. else => add.

### Success

On successful mutation:
- replace/update the post in `m.posts`;
- replace/update the cached copy in `postsByChannel`;
- replace/update the copy in `threadPosts` if present;
- close picker;
- status `reaction added` or `reaction removed`.

### Failure

On failure:
- close picker;
- leave local state unchanged;
- status `reaction failed`.

## Rendering

Messages with reactions should show a compact badge row, e.g.:

```text
👍 2  👀 1  ✅ 3
```

My own reactions should be visually distinct. For the first slice, a subtle accent/pill difference is enough.

Rules:
- do not render empty reactions;
- preserve stable reaction order based on the fixed picker list first, then any extra reactions if later needed;
- keep rendering compact so it does not blow up timeline density.

## Testing Strategy

### Pure helper tests

- add reaction to empty post
- remove reaction when `Reacted == true`
- count increments/decrements correctly
- zero-count reactions disappear after remove

### App behavior tests

- `R` opens picker from timeline focus
- picker `enter` dispatches add/remove intent correctly
- success updates visible/cached/thread copies
- failure keeps local copies unchanged
- `esc` closes picker

### Render tests

- reactions render compact count badges
- my reaction is visually distinct

## Non-Goals

Not in this slice:

- thread viewport reaction actions
- emoji search
- reaction autocomplete
- live presence/user list per reaction
- server-side notification semantics beyond existing backend behavior

## Recommended Follow-Up

After this slice, the next best increment is either:

1. extend reactions to thread viewport once thread message selection exists, or
2. add lightweight “message action hint” polish in the status/help layer if we want faster discoverability.
