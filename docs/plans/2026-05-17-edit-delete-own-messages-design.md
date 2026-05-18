# Edit & Delete Own Messages Design

## Goal

Add the first server-mutating message power tools to `band-tui`: edit and delete for the user’s own messages from timeline focus, without expanding scope into thread-viewport actions or a general message action framework.

## Chosen Slice

Scope is deliberately narrow:

- **timeline focus only**
- **own messages only**
- **edit + delete together**
- **no thread viewport support yet**

This is the smallest slice that produces meaningful feature parity with the web client while keeping interaction and state management comprehensible.

## Why This Slice

Edit/delete is the first message-actions slice that requires backend mutation and local cache updates. That makes it the right next milestone after quote/permalink, but also a place to be conservative.

Supporting thread viewport immediately would force a second message-selection model, extra key routing, and more update paths through `threadPosts`. A timeline-only first cut keeps the mutation semantics and tests focused on one message list.

## UX Contract

### Edit (`e`)

`e` works only in **timeline focus** and only when the selected post belongs to the current user.

Behavior:

1. read the selected post;
2. if it is not the user’s own post, set status `can only edit your own messages`;
3. if it is own post:
   - enter edit mode with `editingPostID` set to the post ID;
   - load the post text into the composer;
   - focus the composer;
   - status `editing message`.

While in edit mode:
- composer `enter` performs an update mutation instead of creating a new post;
- sending keeps the same draft-safety expectations as normal send:
  - success clears edit mode;
  - failure keeps the edited text in the composer and leaves edit mode intact.

Cancellation should be lightweight. The first slice can use `esc` from composer to clear edit mode only if that does not conflict with the current compose model; otherwise a dedicated `status` hint plus overwrite-by-reentry is acceptable. Keep this boring.

### Delete (`D`)

`D` works only in **timeline focus** and only for the user’s own post.

Behavior:

1. first press on an own post arms delete confirmation for that specific post ID and sets status `press D again to delete`;
2. second press on the same selected post performs delete;
3. selecting another post or taking another navigation action disarms the pending delete.

If the post is not owned by the user, status should be `can only delete your own messages`.

No modal dialog for the first slice.

## State Model

Add small explicit state to `Model`:

```go
editingPostID      string
pendingDeletePostID string
```

This is enough for the first cut.

### Edit mode rules

- `editingPostID != ""` means composer submit should call update, not create.
- only timeline posts participate in this state.
- successful edit updates local `posts`, `postsByChannel`, and `threadPosts` if the edited post is also present there.
- edit does not affect unread/mention/thread-important state; only the message body changes.

### Delete mode rules

- `pendingDeletePostID` is armed only from timeline focus;
- navigating selection, opening thread, switching focus, or starting edit should clear pending delete;
- after successful delete, remove the post from all local lists and clamp selection.

## Backend/API Shape

This slice requires extending `domain.Backend` with two methods:

```go
UpdatePost(ctx context.Context, postID, message string) (Post, error)
DeletePost(ctx context.Context, postID string) error
```

### Mattermost mapping

- edit: `PUT /api/v4/posts/{postID}` with `{ "message": ... }`
- delete: `DELETE /api/v4/posts/{postID}`

Mattermost update returns the updated post object; delete can be treated as success/no payload.

### Mock backend

Mock must support both operations deterministically:

- update post in its in-memory store;
- delete post from its in-memory store;
- return an error for explicit trigger strings if we need failure-path tests later.

## App Flow

### Edit send path

Current composer submit already switches between channel send and thread reply based on context. Extend that dispatch:

- if `editingPostID != ""` => call update command
- else current behavior

This keeps edit mode integrated with the same composer and draft-safety machinery instead of inventing a separate mini-editor.

### Delete path

Delete never touches the composer.

Success path:
- remove the post from `m.posts`
- remove it from `postsByChannel[channelID]`
- remove it from `threadPosts` if present
- clamp selection
- rebuild triage if needed
- refresh viewport
- status `message deleted`

## Testing Strategy

This slice should be tested in three layers.

### Pure/local model behavior

- cannot edit another user’s post
- cannot delete another user’s post
- first `D` arms confirmation
- second `D` on same post triggers delete path
- navigation clears pending delete
- entering edit mode loads composer and focus

### Backend client/mock

- Mattermost edit request shape is correct
- Mattermost delete request path/method is correct
- mock update actually changes stored post
- mock delete actually removes post

### Integration behavior in app

- successful edit updates local timeline/cache/thread copies
- failed edit keeps composer text and edit mode
- successful delete removes post and clamps selection
- delete does not leave stale pending state

## Non-Goals

Not in this slice:

- thread viewport edit/delete
- edit history
- delete undo
- reaction support
- batch message actions
- soft-delete placeholder rendering changes beyond whatever the backend already implies

## Recommended Follow-Up

Once timeline-only edit/delete is solid, the next obvious increment is either:

1. extend the same actions to thread viewport once thread message selection exists, or
2. add reactions if we want breadth before deepening thread interactions.
