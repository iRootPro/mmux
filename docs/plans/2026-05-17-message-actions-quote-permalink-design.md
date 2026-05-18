# Quote Reply & Permalink Actions Design

## Goal

Add fast, keyboard-first message actions for quoting a selected message into the composer and copying a permalink to the selected message, without expanding backend scope or risking the reliability work already completed.

## Problem

`band-tui` already has the basic read/send loop and a few per-message actions:

- `o` / `enter` on timeline opens the first link in the selected message;
- `y` copies selected message text;
- `t` opens thread view for the selected message.

What is still missing is the “I want to respond to this specific thing” workflow. In practice that means two very common operations:

1. take the selected message, quote it into the current composer, then continue typing;
2. copy a stable link to the selected message for sharing or bookmarking outside the app.

These are high-value actions because they make the client materially more useful in daily work, but they do not require server-side mutations, cache invalidation, or new unread logic. That makes them a good first “message power tools” slice.

## Chosen Slice

Implement exactly two new timeline actions:

- `r` — quote selected message into the current composer
- `p` — copy permalink for selected message

Do **not** bundle this slice with:

- edit/delete own message
- reactions
- thread reply mode changes
- quote from thread viewport
- a generic message action menu

The goal is to add two extremely useful actions with minimal surface area and minimal risk.

## UX Contract

### Quote reply (`r`)

`r` only works in **timeline focus**.

Behavior:

1. read the currently selected post;
2. build a compact quote block;
3. insert it into the existing composer;
4. move logical focus to composer.

Recommended quote format:

```text
> Alice:
> original message line 1
> original message line 2

```

Rules:

- if the selected message text is empty after trimming, do not insert anything; set status like `selected message is empty`;
- if the composer already contains text, insert a separating newline before the quote when needed;
- after insertion, leave the cursor below the quote so the user can immediately type the response;
- do not automatically open a thread or change message destination.

This is intentionally conservative: quote is a text operation, not a transport or navigation operation.

### Permalink copy (`p`)

`p` also works only in **timeline focus**.

Behavior:

1. build a permalink for the selected message;
2. copy it via clipboard;
3. show a status like `permalink copied`.

Best-effort permalink format:

- preferred: `<server>/<team-name>/pl/<post-id>`
- fallback: `<server>/pl/<post-id>` if a team slug cannot be resolved cleanly.

This should never block the action if team lookup is ambiguous. A useful fallback link is better than no link.

## Architecture

Keep this slice local to `internal/app`.

No backend interface changes are required.

Recommended additions in `internal/app/actions.go`:

```go
func (m Model) quoteSelectedPost() (tea.Model, tea.Cmd)
func (m Model) copySelectedPostPermalink() (tea.Model, tea.Cmd)
func formatQuotedReply(post domain.Post) string
func (m Model) selectedPostPermalink() (string, bool)
```

### Quote data flow

- `selectedPostIndex()` already gives the active post.
- `formatQuotedReply(post)` is pure and easy to test.
- `quoteSelectedPost()` mutates only local composer/focus/status state.
- draft safety already exists, so quoting into the composer automatically composes well with current draft handling.

### Permalink data flow

- selected `domain.Post` provides `Post.ID` and `ChannelID`.
- `Model.session.Teams` plus the selected channel’s `TeamID` can supply a team slug/name for scoped links.
- clipboard behavior should follow the existing `copySelectedPostText()` pattern.

## Edge Cases

1. **No selected post**
   - no-op.

2. **Empty selected message**
   - quote action returns status `selected message is empty`.

3. **Composer already has text**
   - quote appends cleanly with one separating newline when necessary.

4. **Multi-line messages**
   - prefix each line with `> `;
   - preserve line order;
   - no attempt to re-render markdown.

5. **Unknown team slug**
   - fallback to `/pl/<post-id>` on the server root.

6. **DM/group channels**
   - permalink builder should still work if `Channel.TeamID` is empty by using the root fallback.

## Testing Strategy

This slice should be test-heavy but simple.

### Pure-format tests

- single-line quote
- multi-line quote
- quote formatting includes author header
- permalink builder with known team slug
- permalink builder fallback when team slug unavailable

### Behavior tests

- `r` inserts quote into empty composer
- `r` appends below existing draft text
- `r` moves focus to composer
- `p` copies permalink to clipboard command path and sets correct status
- existing `y`, `o`, `t` behavior remains unaffected

### Documentation

Update help and README with:

- `r` — quote selected message into composer
- `p` — copy selected message permalink

## Non-Goals

Not in this slice:

- thread-viewport message actions
- edit/delete own message
- reactions
- server-side message mutations
- quote-specific markdown rendering beyond raw text prefixing

## Recommended Next Slice After This

After quote/permalink, the next logical message-actions slice is **edit/delete own message**, because that is the most important parity feature and the next point where backend/API work becomes necessary.
