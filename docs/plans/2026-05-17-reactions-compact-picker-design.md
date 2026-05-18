# Compact Timeline Reaction Picker Design

## Goal

Add a fast, keyboard-first way to react to the selected timeline message with a compact picker and compact reaction badges, without expanding scope into thread-viewport interactions or arbitrary emoji search.

## Scope

First slice only:

- timeline focus only
- compact centered reaction picker overlay
- fixed reaction set
- toggle semantics (add if absent, remove if already mine)
- compact reaction badge rendering on messages

Not in scope:

- thread viewport reactions
- arbitrary emoji search/input
- reaction user lists
- reaction management screen
- live websocket reaction sync if Mattermost does not already deliver it cheaply

## Key Constraint

Mattermost reactions are keyed by **emoji name**, not raw unicode glyph. The app therefore needs a tiny fixed catalog that maps API names to display glyphs.

Recommended fixed set:

- `+1` → 👍
- `eyes` → 👀
- `white_check_mark` → ✅
- `heart` → ❤️
- `tada` → 🎉

This gives us a useful reaction set without building emoji search.

## UX Contract

### Open picker

- `R` in timeline focus opens the picker for the selected post.
- no selected post => no-op.
- picker is centered and compact, in the same visual family as existing lightweight overlays.

### Navigate picker

- `j/k` or arrows move selection
- `enter` toggles the selected reaction
- `esc` closes the picker

### Toggle semantics

The app decides add vs remove locally:

- if I already reacted with the chosen emoji name => remove
- otherwise => add

After a successful toggle:
- close picker
- update local copies of the post immediately
- status `reaction added` or `reaction removed`

On failure:
- close picker
- leave local state unchanged
- status `reaction failed`

## Data Model

Reactions should live on `domain.Post`, not in a side map.

Recommended type:

```go
type PostReaction struct {
    Name    string
    Count   int
    Reacted bool
}
```

and in `domain.Post`:

```go
Reactions []PostReaction
```

Why `Name` instead of glyph:
- backend API uses names;
- stable matching/toggling is easier;
- rendering can map names to glyphs through a local lookup helper.

## Backend Shape

Extend `domain.Backend` with explicit reaction methods:

```go
AddReaction(ctx context.Context, postID, emojiName string) (Post, error)
RemoveReaction(ctx context.Context, postID, emojiName string) (Post, error)
```

Why explicit add/remove instead of backend toggle:
- UI already knows intent from current selected post state
- tests are simpler and backend remains dumb transport

### Mattermost mapping

Add reaction:
- `POST /api/v4/reactions`
- body shaped like Mattermost reaction payload with `user_id`, `post_id`, `emoji_name`

Remove reaction:
- `DELETE /api/v4/users/{userID}/posts/{postID}/reactions/{emojiName}`

After either mutation, fetch the updated post state needed for rendering reactions. If the normal post response already includes reaction metadata, use it; otherwise fetch reactions explicitly and merge them into the returned `domain.Post`.

### Mock backend

Mock backend should:
- mutate reactions on stored posts in memory
- support add/remove deterministically
- optionally expose explicit failure trigger strings later, but that is not required for the first slice if app failure tests use local stubs

## App State

Add minimal picker state to `Model`:

```go
reactionPickerOpen     bool
reactionPickerSelected int
```

And a local fixed catalog like:

```go
type reactionOption struct {
    Name  string
    Glyph string
}
```

No new text input or search state.

## Local Update Rules

On successful reaction mutation, update all local copies consistently:

- visible `m.posts`
- cached `postsByChannel`
- `threadPosts` if that message is present there

This should reuse the same replacement pattern already used for edit/update flows.

## Rendering

Messages with reactions should show compact badges, for example:

```text
👍 2  👀 1  ✅ 3
```

Rules:
- only render non-zero reactions
- preserve stable catalog order first
- visually distinguish `Reacted == true`
- keep density tight; badges should not dominate the message body

## Testing Strategy

### Pure helper tests

- detect whether a reaction is currently mine
- add missing reaction
- remove existing own reaction
- preserve unrelated reactions
- zero-count reactions disappear

### App behavior tests

- `R` opens picker
- picker `esc` closes
- picker `enter` dispatches add/remove correctly
- success updates visible/cached/thread copies
- failure leaves local state unchanged

### Render tests

- picker shows fixed reactions
- badges render compact counts
- my reaction is highlighted

## Non-Goals

Not in this slice:

- thread viewport reactions
- emoji search
- reaction user popovers
- emoji autocomplete
- server event sync redesign

## Recommended Follow-Up

After this slice, the next logical increment is either:

1. extend reactions to thread viewport once thread message selection exists, or
2. add a small message-action status hint / discoverability polish.
