# UI UX Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the chat TUI read like a real conversation instead of a dense terminal log, while preserving the keyboard-first workflow and compact layout.

**Architecture:** Keep this as a render-layer improvement in `internal/app/view.go` with small pure helpers and targeted tests in `internal/app/*_test.go`. Do not change backend APIs, Mattermost models, persistence, or keybindings unless a task explicitly says so. Preserve stable pane heights: header remains two rows, composer remains the same height, and thread layout must not jump.

**Tech Stack:** Go, Bubble Tea, Bubbles viewport/textarea, Lip Gloss, existing `domain.Channel` and `domain.Post` types.

---

## Baseline from the screenshot and current code

The screenshot shows the right product direction: compact, keyboard-first, dark terminal UI. The main UX debt is visual hierarchy: repeated author headers, weak unread/count semantics in the sidebar, weak focus affordances, and a composer/status area that does not explain the current mode strongly enough.

Current relevant code:

- `internal/app/view.go:592-633` — sidebar shell and focus border.
- `internal/app/view.go:725-759` — sidebar channel row rendering.
- `internal/app/view.go:761-815` — sidebar crop/overflow labels.
- `internal/app/view.go:818-842` — main pane layout.
- `internal/app/view.go:854-895` — channel/DM header.
- `internal/app/view.go:978-992` — main composer.
- `internal/app/view.go:995-1018` — status bar.
- `internal/app/view.go:1040-1128` — timeline post rendering.
- `internal/app/view.go:420-448` — thread post rendering.

Current tests already cover header stability, unread separator, sidebar scope label, focus behavior, thread layout height, and thread composer behavior. Every task below must keep the existing tests passing.

## Acceptance criteria

- Consecutive messages from the same author are visually grouped in the main timeline.
- Important messages are never hidden inside a group: unread, mention, thread-unread, or reply-count-bearing posts still get their own visible header/indicator.
- Sidebar unread and mention state remains visible even in narrow sidebars; mentions take priority over unread counts.
- Sidebar crop labels tell the user how many items are hidden above/below instead of generic `… выше` / `… ниже`.
- Header shows stronger selected-context metadata, including loaded message count when available.
- Composer and status bar make the active focus/mode obvious without changing layout height.
- Thread rendering follows the same readability rules where it does not make thread interaction ambiguous.
- Verification covers render behavior with targeted unit tests and a final `go test ./...`.

---

### Task 1: Group consecutive timeline messages

**Files:**

- Modify: `internal/app/view.go:1040-1128`
- Test: create `internal/app/message_grouping_test.go`

**Step 1: Write failing tests**

Create `internal/app/message_grouping_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
)

func TestRenderPostsGroupsConsecutiveMessagesFromSameAuthor(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		posts: []domain.Post{
			{ID: "p1", UserID: "u1", Username: "Alice", Message: "first", CreateAt: base},
			{ID: "p2", UserID: "u1", Username: "Alice", Message: "second", CreateAt: base + 60_000},
			{ID: "p3", UserID: "u2", Username: "Bob", Message: "third", CreateAt: base + 120_000},
		},
		selectedPost: -1,
	}

	got, offsets := m.renderPosts()

	if strings.Count(got, "Alice") != 1 {
		t.Fatalf("Alice header count = %d, want 1 in:\n%s", strings.Count(got, "Alice"), got)
	}
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") || !strings.Contains(got, "Bob") {
		t.Fatalf("grouped timeline lost content:\n%s", got)
	}
	if len(offsets) != len(m.posts) || offsets[1] <= offsets[0] || offsets[2] <= offsets[1] {
		t.Fatalf("bad post offsets: %#v", offsets)
	}
}

func TestRenderPostsDoesNotGroupImportantMessages(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		posts: []domain.Post{
			{ID: "p1", UserID: "u1", Username: "Alice", Message: "old", CreateAt: base},
			{ID: "p2", UserID: "u1", Username: "Alice", Message: "new", CreateAt: base + 60_000, Unread: true},
			{ID: "p3", UserID: "u1", Username: "Alice", Message: "reply root", CreateAt: base + 120_000, ReplyCount: 2},
		},
		selectedPost: 1,
	}

	got, _ := m.renderPosts()

	if strings.Count(got, "Alice") != 3 {
		t.Fatalf("important messages should keep headers, got:\n%s", got)
	}
	if !strings.Contains(got, "new messages") || !strings.Contains(got, "↳ 2") {
		t.Fatalf("important indicators missing:\n%s", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderPostsGroupsConsecutiveMessagesFromSameAuthor|TestRenderPostsDoesNotGroupImportantMessages' -count=1
```

Expected: first test fails because current `renderPosts` renders `Alice` for both consecutive messages.

**Step 3: Add grouping helpers**

In `internal/app/view.go`, near `renderPosts`, add pure helpers:

```go
const messageGroupWindowMillis int64 = 5 * 60 * 1000

func shouldGroupTimelinePost(prev, current domain.Post) bool {
	if isImportantPost(current) || current.ReplyCount > 0 {
		return false
	}
	if prev.CreateAt <= 0 || current.CreateAt <= 0 {
		return false
	}
	if formatDate(prev.CreateAt) != formatDate(current.CreateAt) {
		return false
	}
	delta := current.CreateAt - prev.CreateAt
	if delta < 0 {
		delta = -delta
	}
	if delta > messageGroupWindowMillis {
		return false
	}
	return samePostAuthor(prev, current)
}

func samePostAuthor(a, b domain.Post) bool {
	if a.UserID != "" || b.UserID != "" {
		return a.UserID != "" && a.UserID == b.UserID
	}
	return a.Username != "" && a.Username == b.Username
}
```

Do not import `time`; the millisecond constant is enough.

**Step 4: Use grouping in `renderPosts`**

In the `for i, p := range m.posts` loop:

- Compute `grouped := i > 0 && shouldGroupTimelinePost(m.posts[i-1], p)` after date/unread divider handling.
- Set `offsets[i] = lineNo` before rendering either the compact continuation or normal header.
- If `grouped` is false, render the existing header exactly as today.
- If `grouped` is true, skip the author header and render only the message body, indented the same way as a normal body line.
- Keep blank-line spacing between posts; do not collapse messages into one paragraph.
- Keep `selected := i == m.selectedPost && m.focus == focusTimeline`; selected grouped messages must still show the `▌` marker on their body lines.

Minimal shape:

```go
grouped := i > 0 && shouldGroupTimelinePost(m.posts[i-1], p)
offsets[i] = lineNo
selected := i == m.selectedPost && m.focus == focusTimeline
if !grouped {
	header := renderTimelinePostHeader(p)
	writeLine(m.renderPostLine(header, selected))
}
body := renderMarkdownMessage(p.Message, max(20, m.viewport.Width-6))
for _, line := range strings.Split(body, "\n") {
	writeLine(m.renderPostLine(baseStyle.Render(line), selected))
}
```

Extract `renderTimelinePostHeader(p domain.Post) string` only if it keeps `renderPosts` simpler; do not add an abstraction that is used once and makes the loop harder to read.

**Step 5: Run tests**

Run:

```bash
go test ./internal/app -run 'TestRenderPostsGroupsConsecutiveMessagesFromSameAuthor|TestRenderPostsDoesNotGroupImportantMessages|TestRenderPostsShowsNewMessagesSeparator' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/app/view.go internal/app/message_grouping_test.go
git commit -m "feat: group consecutive timeline messages"
```

---

### Task 2: Make sidebar unread and mention badges robust

**Files:**

- Modify: `internal/app/view.go:725-759`
- Test: modify `internal/app/sidebar_test.go`

**Step 1: Write failing tests**

Append to `internal/app/sidebar_test.go`:

```go
func TestRenderSidebarChannelLineKeepsMentionBadgeVisible(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Very Long Channel Name That Must Truncate", Mentions: 12, Unread: 99}},
	}

	got := m.renderSidebarChannelLine(0, 24)

	if !strings.Contains(got, "@12") {
		t.Fatalf("mention badge not visible in narrow row: %q", got)
	}
	if strings.Contains(got, "99") {
		t.Fatalf("mentions must take priority over unread count: %q", got)
	}
}

func TestRenderSidebarChannelLineShowsUnreadCount(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "General", Unread: 7}},
	}

	got := m.renderSidebarChannelLine(0, 24)

	if !strings.Contains(got, "7") {
		t.Fatalf("unread count not visible: %q", got)
	}
}
```

Ensure `sidebar_test.go` imports both `strings` and `band-tui/internal/domain`; they are already used in this file today.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderSidebarChannelLineKeepsMentionBadgeVisible|TestRenderSidebarChannelLineShowsUnreadCount' -count=1
```

Expected: at least the narrow mention-badge test fails because the current row appends badges before truncation.

**Step 3: Add badge helpers**

Near `renderSidebarChannelLine`, add:

```go
func channelBadge(ch domain.Channel) string {
	if ch.Mentions > 0 {
		return fmt.Sprintf("@%d", ch.Mentions)
	}
	if ch.Unread > 0 {
		return fmt.Sprintf("%d", ch.Unread)
	}
	return ""
}

func joinLabelAndBadge(label, badge string, width int) string {
	if width <= 0 {
		return ""
	}
	if badge == "" {
		return truncate(label, width)
	}
	badgeWidth := lipgloss.Width(badge)
	if badgeWidth >= width {
		return truncate(badge, width)
	}
	labelWidth := max(0, width-badgeWidth-1)
	left := truncate(label, labelWidth)
	spaces := max(1, width-lipgloss.Width(left)-badgeWidth)
	return left + strings.Repeat(" ", spaces) + badge
}
```

This uses existing imports: `fmt`, `strings`, and `lipgloss` are already imported in `view.go`.

**Step 4: Use helpers in `renderSidebarChannelLine`**

Change the row construction so the badge is reserved at the right edge before styling:

```go
label := marker + presenceGlyphPlain(ch.Status) + prefix + " " + name
if m.favoriteChannels[ch.ID] {
	label += " ★"
}
if scope := m.scopeSuffix(ch); scope != "" {
	label += " · " + scope
}
line := joinLabelAndBadge(label, channelBadge(ch), width)
```

Then keep the selected/non-selected styling:

```go
if index == m.selectedChannel {
	return pillStyle.Width(width).Render(truncate(line, width))
}
return style.Render(line)
```

Do not color badge differently in this task. Shape and count are the reliable affordances; color refinements can come later.

**Step 5: Run tests**

Run:

```bash
go test ./internal/app -run 'TestRenderSidebarChannelLineKeepsMentionBadgeVisible|TestRenderSidebarChannelLineShowsUnreadCount|TestRenderSidebarShowsScopeLabel' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/app/view.go internal/app/sidebar_test.go
git commit -m "feat: reserve sidebar unread badges"
```

---

### Task 3: Replace vague sidebar crop markers with hidden counts

**Files:**

- Modify: `internal/app/view.go:761-815`
- Test: modify `internal/app/sidebar_test.go`

**Step 1: Write failing test**

Append to `internal/app/sidebar_test.go`:

```go
func TestCropSidebarLinesShowsHiddenCounts(t *testing.T) {
	items := []sidebarLine{
		{Text: "▾ ЛИЧНЫЕ", Section: "ЛИЧНЫЕ", Header: true},
		{Text: "one", Section: "ЛИЧНЫЕ"},
		{Text: "two", Section: "ЛИЧНЫЕ"},
		{Text: "three", Section: "ЛИЧНЫЕ"},
		{Text: "four", Section: "ЛИЧНЫЕ"},
		{Text: "five", Section: "ЛИЧНЫЕ"},
		{Text: "six", Section: "ЛИЧНЫЕ"},
	}

	got := strings.Join(cropSidebarLines(items, 5, 4), "\n")

	if !strings.Contains(got, "ещё") || !strings.Contains(got, "ЛИЧНЫЕ") {
		t.Fatalf("crop labels should include hidden count and section: %q", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/app -run TestCropSidebarLinesShowsHiddenCounts -count=1
```

Expected: FAIL because current labels are generic `… выше` / `… ниже` without counts.

**Step 3: Add crop label helper**

Near `sectionSuffix`, add:

```go
func hiddenItemsLabel(direction string, count int, section string) string {
	if count <= 0 {
		return ""
	}
	label := direction + " ещё " + fmt.Sprint(count)
	if section != "" {
		label += " · " + strings.ToLower(section)
	}
	return label
}
```

Use arrows as `↑` and `↓` in callers. The symbols are safe because the existing UI already uses Unicode glyphs.

**Step 4: Use counts in `cropSidebarLines`**

When `start > 0`, compute hidden count as `start`. Replace fallback labels with:

```go
cropped[0] = sidebarLine{Text: muted.Render(hiddenItemsLabel("↑", start, section)), Section: section}
```

When `start+height < len(items)`, compute hidden count as `len(items) - (start + height)`. Replace the last row with:

```go
hiddenBelow := len(items) - (start + height)
cropped[len(cropped)-1] = sidebarLine{Text: muted.Render(hiddenItemsLabel("↓", hiddenBelow, section)), Section: section}
```

Keep the existing behavior that restores a section header when cropping starts inside a section, but make the second row count-based instead of `… выше`.

**Step 5: Run tests**

Run:

```bash
go test ./internal/app -run 'TestCropSidebarLinesShowsHiddenCounts|TestRenderSidebarShowsScopeLabel' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/app/view.go internal/app/sidebar_test.go
git commit -m "feat: show sidebar hidden counts"
```

---

### Task 4: Strengthen selected-channel header metadata

**Files:**

- Modify: `internal/app/view.go:854-951`
- Test: modify `internal/app/header_test.go`

**Step 1: Write failing tests**

Append to `internal/app/header_test.go`:

```go
func TestRenderHeaderShowsLoadedMessageCount(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town", MemberCount: 42}},
		posts: []domain.Post{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}},
		selectedChannel: 0,
	}

	got := m.renderHeader(120)

	if !strings.Contains(got, "42 members") || !strings.Contains(got, "3 messages") {
		t.Fatalf("header missing metadata: %q", got)
	}
}

func TestRenderHeaderKeepsDMStatusAndMessageCount(t *testing.T) {
	m := Model{
		channels: []domain.Channel{{ID: "d1", Type: "D", DisplayName: "Alice", Status: "offline"}},
		posts: []domain.Post{{ID: "p1"}},
		selectedChannel: 0,
	}

	got := m.renderHeader(120)

	if !strings.Contains(got, "offline") || !strings.Contains(got, "1 message") {
		t.Fatalf("DM header missing status/count: %q", got)
	}
}
```

`header_test.go` already imports `strings`, `testing`, and `domain`; extend only if needed.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderHeaderShowsLoadedMessageCount|TestRenderHeaderKeepsDMStatusAndMessageCount' -count=1
```

Expected: FAIL because `channelMeta` does not include loaded message count.

**Step 3: Add message count to header metadata**

Change `channelMeta(ch domain.Channel)` to include `len(m.posts)` after member/status metadata and before unread/mention metadata:

```go
if len(m.posts) > 0 {
	parts = append(parts, fmt.Sprintf("%d %s", len(m.posts), plural(len(m.posts), "message", "messages")))
}
```

Keep existing mention/unread logic after this so urgent state stays visible at the end of the metadata chain.

**Step 4: Preserve header height and truncation**

Do not remove the two-row workaround in `renderHeader`. Tests `TestRenderHeaderHeightIsStableWithoutTopic` and `TestRenderHeaderDoesNotExpandHugeMarkdownTopic` must continue to pass.

**Step 5: Run tests**

Run:

```bash
go test ./internal/app -run 'TestRenderHeader' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/app/view.go internal/app/header_test.go
git commit -m "feat: show message count in header"
```

---

### Task 5: Make composer and status communicate active mode

**Files:**

- Modify: `internal/app/view.go:978-1018`
- Test: modify `internal/app/focus_test.go` and `internal/app/status_test.go`

**Step 1: Write failing composer test**

Append to `internal/app/focus_test.go`:

```go
func TestRenderComposerShowsDestinationWithoutChangingHeight(t *testing.T) {
	m := New(nil, config.Config{}, true)
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "Town"}}
	m.selectedChannel = 0
	m.focus = focusComposer

	got := m.renderComposer(80)

	if !strings.Contains(got, "message # Town") || !strings.Contains(got, "enter send") {
		t.Fatalf("composer label missing destination/help: %q", got)
	}
	if h := lipgloss.Height(got); h != 4 {
		t.Fatalf("composer height = %d, want 4", h)
	}
}
```

Add imports if missing:

```go
import (
	"strings"
	"testing"

	"band-tui/internal/config"
	"band-tui/internal/domain"
	"github.com/charmbracelet/lipgloss"
)
```

If `focus_test.go` already has some of these imports, merge without duplicating.

**Step 2: Write failing status tests**

Append to `internal/app/status_test.go`:

```go
func TestRenderStatusShowsFocusHint(t *testing.T) {
	m := Model{status: "ready", focus: focusTimeline}
	got := m.renderStatus(120)
	if !strings.Contains(got, "t thread") || !strings.Contains(got, "n unread") {
		t.Fatalf("timeline status hint missing: %q", got)
	}

	m.focus = focusSidebar
	got = m.renderStatus(120)
	if !strings.Contains(got, "/ filter") || !strings.Contains(got, "enter open") {
		t.Fatalf("sidebar status hint missing: %q", got)
	}
}
```

**Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/app -run 'TestRenderComposerShowsDestinationWithoutChangingHeight|TestRenderStatusShowsFocusHint' -count=1
```

Expected: FAIL because current composer label has no destination and status has no focus-specific hints.

**Step 4: Add composer label helper**

Near `renderComposer`, add:

```go
func (m Model) composerLabel(width int) string {
	target := "current channel"
	if len(m.channels) > 0 && m.selectedChannel >= 0 && m.selectedChannel < len(m.channels) {
		ch := m.channels[m.selectedChannel]
		name := sanitizeTerminal(ch.DisplayName)
		if name == "" {
			name = sanitizeTerminal(ch.Name)
		}
		prefix := "#"
		switch ch.Type {
		case "D":
			prefix = "@"
		case "G":
			prefix = "◦"
		}
		if name != "" {
			target = prefix + " " + name
		}
	}
	return truncate("message "+target+" · enter send · ctrl+j newline · tab nav", width)
}
```

In `renderComposer`, replace the hard-coded label with:

```go
label := muted.Render(m.composerLabel(max(10, width-4)))
```

Do not change `composer.SetHeight(3)` or the border; existing height tests must stay green.

**Step 5: Add status focus hints**

Near `renderStatus`, add:

```go
func (m Model) focusStatusHint() string {
	if m.threadOpen {
		return "tab thread · esc close"
	}
	switch m.focus {
	case focusSidebar:
		return "enter open · / filter · tab timeline"
	case focusTimeline:
		return "j/k select · t thread · y copy · n unread"
	case focusComposer:
		return "enter send · ctrl+j newline · tab nav"
	default:
		return "? help"
	}
}
```

In `renderStatus`, append the hint after the base status and before errors/notification badge:

```go
if hint := m.focusStatusHint(); hint != "" {
	status += "  " + muted.Render(hint)
}
```

If `threadOpen` already appends `thread open · reply right · esc close`, remove the duplicate later append or keep only the clearer one. Do not show contradictory thread hints.

**Step 6: Run tests**

Run:

```bash
go test ./internal/app -run 'TestRenderComposerShowsDestinationWithoutChangingHeight|TestRenderStatusShowsFocusHint|TestComposerHeightStable|TestRenderStatusShowsScope' -count=1
```

Expected: PASS.

**Step 7: Commit**

```bash
git add internal/app/view.go internal/app/focus_test.go internal/app/status_test.go
git commit -m "feat: clarify composer and status focus"
```

---

### Task 6: Bring thread reply rendering in line with timeline readability

**Files:**

- Modify: `internal/app/view.go:420-448`
- Test: modify `internal/app/thread_polish_test.go`

**Step 1: Write failing test**

Append to `internal/app/thread_polish_test.go`:

```go
func TestRenderThreadPostsGroupsConsecutiveReplies(t *testing.T) {
	base := int64(1770000000000)
	m := Model{
		threadRootID: "root",
		threadPosts: []domain.Post{
			{ID: "root", Username: "Alice", Message: "Root"},
			{ID: "r1", RootID: "root", UserID: "u2", Username: "Bob", Message: "one", CreateAt: base},
			{ID: "r2", RootID: "root", UserID: "u2", Username: "Bob", Message: "two", CreateAt: base + 60_000},
		},
	}

	got := m.renderThreadPosts(80)

	if strings.Count(got, "Bob") != 1 {
		t.Fatalf("thread replies should group same author, got:\n%s", got)
	}
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("thread grouping lost content:\n%s", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/app -run TestRenderThreadPostsGroupsConsecutiveReplies -count=1
```

Expected: FAIL because thread replies currently repeat the author header.

**Step 3: Reuse timeline grouping logic**

In `renderThreadPosts`, track the previous rendered reply:

```go
var prevReply domain.Post
hasPrevReply := false
```

For each non-root reply:

```go
grouped := hasPrevReply && shouldGroupTimelinePost(prevReply, post)
if !grouped {
	// existing header rendering
}
// existing body rendering
prevReply = post
hasPrevReply = true
```

Keep important replies visible: because `shouldGroupTimelinePost` rejects unread/mentioned/thread-unread and reply-count-bearing posts, unread thread replies still get `●` and a header.

**Step 4: Run tests**

Run:

```bash
go test ./internal/app -run 'TestRenderThreadPostsGroupsConsecutiveReplies|TestRenderThreadPostsSkipsRoot|TestRenderThreadHeader' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/app/view.go internal/app/thread_polish_test.go
git commit -m "feat: group consecutive thread replies"
```

---

### Task 7: Update documented UX hints and run final verification

**Files:**

- Modify: `README.md:94-118`
- No new tests unless README tooling is added later.

**Step 1: Update README keys section**

In `README.md`, update the Keys list to reflect visible hints and grouping behavior:

```markdown
- consecutive messages from the same author are visually grouped; unread/mentioned messages keep their own marker/header
- sidebar rows reserve the right edge for mention/unread counts, with mentions shown as `@N`
- status bar changes hints based on active focus: sidebar, timeline, composer, or thread
```

Keep the existing keybinding list; do not rewrite the README into a product brochure.

**Step 2: Run targeted app tests**

Run:

```bash
go test ./internal/app -count=1
```

Expected: PASS.

**Step 3: Run full repository tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

**Step 4: Manual visual QA in mock mode**

Run:

```bash
go run ./cmd/band-tui --mock
```

Check these scenarios manually:

- Sidebar selected row still fits and mention/unread badge remains visible at the right edge.
- Sidebar crop rows say how many items are hidden above/below.
- Main timeline groups repeated author messages but keeps unread and thread-reply posts visible.
- Header still occupies exactly two rows and does not jump when moving between channel/DM/topic-heavy rows.
- Composer still occupies the same height and says where the message will be sent.
- Status hints change when pressing `tab` / `shift+tab` between sidebar, timeline, and composer.
- Thread panel groups same-author replies without hiding unread reply markers.

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document tui ux affordances"
```

---

## Implementation notes and constraints

- Do not change `domain.Channel` or `domain.Post` for these tasks. Existing fields are enough.
- Do not add a theme system. Add semantic colors only if a task needs them; the current palette can carry this iteration.
- Do not replace Bubble Tea components. This is a rendering pass, not a framework rewrite.
- Do not remove the header first-row workaround unless a failing test proves it is no longer needed.
- Avoid allocations in hot render loops where simple string building works. `renderPosts` already uses `strings.Builder`; keep that pattern.
- Prefer pure helper functions for render decisions so tests do not need a running Bubble Tea program.
- If any test becomes brittle because of ANSI styling, assert stable semantic substrings (`@12`, `new messages`, `message # Town`) rather than exact rendered lines.

## Final verification checklist

Run these before considering the UI/UX improvement branch complete:

```bash
go test ./internal/app -count=1
go test ./... -count=1
go build ./cmd/band-tui
go run ./cmd/band-tui --mock
```

Observed expected result for the first three commands must be success. The mock run must be manually inspected against the visual QA list above.
