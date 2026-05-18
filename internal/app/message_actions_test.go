package app

import (
	"context"
	"strings"
	"testing"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func actionKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
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

	updated, _ := m.handleKey(actionKey("r"))
	got := updated.(Model)
	if got.focus != focusComposer || got.composer.Value() == "" {
		t.Fatalf("quote not inserted, focus=%v composer=%q", got.focus, got.composer.Value())
	}
}

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

func TestHelpTextMentionsQuoteAndPermalinkKeys(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "r") || !strings.Contains(got, "quote") || !strings.Contains(got, "p") || !strings.Contains(got, "permalink") {
		t.Fatalf("help text missing message action keys: %q", got)
	}
}

func TestEditSelectedOwnPostLoadsComposer(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{{ID: "p1", UserID: "me", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.editSelectedPost()
	got := updated.(Model)
	if got.focus != focusComposer {
		t.Fatalf("focus = %v, want composer", got.focus)
	}
	if got.editingPostID != "p1" {
		t.Fatalf("editingPostID = %q", got.editingPostID)
	}
	if got.composer.Value() != "Hello" {
		t.Fatalf("composer = %q", got.composer.Value())
	}
}

func TestCannotEditOtherUsersPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{{ID: "p1", UserID: "other", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.editSelectedPost()
	got := updated.(Model)
	if got.editingPostID != "" {
		t.Fatalf("unexpected edit mode: %q", got.editingPostID)
	}
	if got.status != "can only edit your own messages" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestHandleTimelineKeyEEntersEditMode(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{{ID: "p1", UserID: "me", Username: "Alice", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.handleKey(actionKey("e"))
	got := updated.(Model)
	if got.editingPostID != "p1" || got.focus != focusComposer {
		t.Fatalf("edit mode not entered: editing=%q focus=%v", got.editingPostID, got.focus)
	}
}

type updateRecordingBackend struct {
	updatedPostID string
	updatedText   string
	updateErr     error
}

func (b *updateRecordingBackend) Connect(context.Context) (*domain.Session, error) { return nil, nil }
func (b *updateRecordingBackend) LoadChannels(context.Context, string) ([]domain.Channel, error) {
	return nil, nil
}
func (b *updateRecordingBackend) LoadPosts(context.Context, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *updateRecordingBackend) LoadPostsBefore(context.Context, string, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *updateRecordingBackend) ViewChannel(context.Context, string) error { return nil }
func (b *updateRecordingBackend) LoadThread(context.Context, string) ([]domain.Post, error) {
	return nil, nil
}
func (b *updateRecordingBackend) SendPost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *updateRecordingBackend) SendReply(context.Context, string, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *updateRecordingBackend) UpdatePost(_ context.Context, postID, message string) (domain.Post, error) {
	b.updatedPostID = postID
	b.updatedText = message
	if b.updateErr != nil {
		return domain.Post{}, b.updateErr
	}
	return domain.Post{ID: postID, ChannelID: "c1", UserID: "me", Username: "You", Message: message}, nil
}
func (b *updateRecordingBackend) DeletePost(context.Context, string) error { return nil }
func (b *updateRecordingBackend) AddReaction(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *updateRecordingBackend) RemoveReaction(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *updateRecordingBackend) WatchPosts(context.Context, chan<- domain.Event) error { return nil }
func (b *updateRecordingBackend) Close() error                                          { return nil }

func TestEditSubmitUsesUpdatePostPath(t *testing.T) {
	backend := &updateRecordingBackend{}
	m := New(backend, testConfig(), false)
	m.focus = focusComposer
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.editingPostID = "p1"
	m.composer.SetValue("edited")

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.status != "updating…" {
		t.Fatalf("status = %q", got.status)
	}
	if cmd == nil {
		t.Fatal("expected update command")
	}
	msg := cmd()
	updatedMsg, ok := msg.(postUpdatedMsg)
	if !ok {
		t.Fatalf("command msg = %#v", msg)
	}
	if updatedMsg.post.ID != "p1" || backend.updatedPostID != "p1" || backend.updatedText != "edited" {
		t.Fatalf("update path not used: msg=%#v backend=%#v", updatedMsg, backend)
	}
}

func TestSuccessfulEditClearsEditModeAndUpdatesPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.editingPostID = "p1"
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "old"}}
	m.postsByChannel = map[string][]domain.Post{"c1": {{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "old"}}}
	m.threadPosts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "old"}}
	m.composer.SetValue("edited")

	updated, _ := m.Update(postUpdatedMsg{postID: "p1", post: domain.Post{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "edited"}})
	got := updated.(Model)
	if got.editingPostID != "" {
		t.Fatalf("editingPostID = %q", got.editingPostID)
	}
	if got.posts[0].Message != "edited" || got.postsByChannel["c1"][0].Message != "edited" || got.threadPosts[0].Message != "edited" {
		t.Fatalf("updated post not propagated: %#v %#v %#v", got.posts, got.postsByChannel["c1"], got.threadPosts)
	}
	if got.composer.Value() != "" {
		t.Fatalf("composer not cleared after successful edit: %q", got.composer.Value())
	}
}

func TestFailedEditKeepsComposerTextAndEditMode(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.editingPostID = "p1"
	m.composer.SetValue("edited")

	updated, _ := m.Update(postUpdatedMsg{postID: "p1", err: assertErr{}})
	got := updated.(Model)
	if got.editingPostID != "p1" {
		t.Fatalf("editingPostID = %q", got.editingPostID)
	}
	if got.composer.Value() != "edited" {
		t.Fatalf("composer = %q", got.composer.Value())
	}
	if got.status != "update failed" {
		t.Fatalf("status = %q", got.status)
	}
}

type deleteRecordingBackend struct {
	deletedPostID string
	deleteErr     error
}

func (b *deleteRecordingBackend) Connect(context.Context) (*domain.Session, error) { return nil, nil }
func (b *deleteRecordingBackend) LoadChannels(context.Context, string) ([]domain.Channel, error) {
	return nil, nil
}
func (b *deleteRecordingBackend) LoadPosts(context.Context, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *deleteRecordingBackend) LoadPostsBefore(context.Context, string, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *deleteRecordingBackend) ViewChannel(context.Context, string) error { return nil }
func (b *deleteRecordingBackend) LoadThread(context.Context, string) ([]domain.Post, error) {
	return nil, nil
}
func (b *deleteRecordingBackend) SendPost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *deleteRecordingBackend) SendReply(context.Context, string, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *deleteRecordingBackend) UpdatePost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *deleteRecordingBackend) DeletePost(_ context.Context, postID string) error {
	b.deletedPostID = postID
	return b.deleteErr
}
func (b *deleteRecordingBackend) AddReaction(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *deleteRecordingBackend) RemoveReaction(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *deleteRecordingBackend) WatchPosts(context.Context, chan<- domain.Event) error { return nil }
func (b *deleteRecordingBackend) Close() error                                          { return nil }

func TestDeleteOwnPostRequiresConfirmation(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{{ID: "p1", UserID: "me", Message: "Hello"}}
	m.selectedPost = 0

	updated, cmd := m.deleteSelectedPost()
	got := updated.(Model)
	if got.pendingDeletePostID != "p1" {
		t.Fatalf("pendingDeletePostID = %q", got.pendingDeletePostID)
	}
	if got.status != "press D again to delete" {
		t.Fatalf("status = %q", got.status)
	}
	if cmd != nil {
		t.Fatal("first delete press should not dispatch command")
	}
}

func TestDeleteConfirmationClearsOnSelectionChange(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{
		{ID: "p1", UserID: "me", Message: "Hello"},
		{ID: "p2", UserID: "me", Message: "Other"},
	}
	m.selectedPost = 0
	m.pendingDeletePostID = "p1"

	updated, _ := m.selectPost(1)
	got := updated.(Model)
	if got.pendingDeletePostID != "" {
		t.Fatalf("pending delete not cleared: %q", got.pendingDeletePostID)
	}
}

func TestCannotDeleteOtherUsersPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.posts = []domain.Post{{ID: "p1", UserID: "other", Message: "Hello"}}
	m.selectedPost = 0

	updated, _ := m.deleteSelectedPost()
	got := updated.(Model)
	if got.pendingDeletePostID != "" {
		t.Fatalf("unexpected pending delete: %q", got.pendingDeletePostID)
	}
	if got.status != "can only delete your own messages" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestSuccessfulDeleteRemovesPostAndClampsSelection(t *testing.T) {
	backend := &deleteRecordingBackend{}
	m := New(backend, testConfig(), false)
	m.focus = focusTimeline
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.posts = []domain.Post{
		{ID: "p1", ChannelID: "c1", UserID: "me", Message: "Hello"},
		{ID: "p2", ChannelID: "c1", UserID: "me", Message: "Other"},
	}
	m.postsByChannel = map[string][]domain.Post{"c1": {
		{ID: "p1", ChannelID: "c1", UserID: "me", Message: "Hello"},
		{ID: "p2", ChannelID: "c1", UserID: "me", Message: "Other"},
	}}
	m.threadPosts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Message: "Hello"}}
	m.selectedPost = 0
	m.pendingDeletePostID = "p1"

	updated, cmd := m.deleteSelectedPost()
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	msg := cmd()
	deletedMsg, ok := msg.(postDeletedMsg)
	if !ok {
		t.Fatalf("command msg = %#v", msg)
	}
	if deletedMsg.postID != "p1" || backend.deletedPostID != "p1" {
		t.Fatalf("delete path not used: msg=%#v backend=%#v", deletedMsg, backend)
	}
	updated, _ = got.Update(postDeletedMsg{postID: "p1"})
	got = updated.(Model)
	if len(got.posts) != 1 || got.posts[0].ID != "p2" {
		t.Fatalf("post not removed from timeline: %#v", got.posts)
	}
	if len(got.postsByChannel["c1"]) != 1 || got.postsByChannel["c1"][0].ID != "p2" {
		t.Fatalf("post not removed from cache: %#v", got.postsByChannel["c1"])
	}
	if len(got.threadPosts) != 0 {
		t.Fatalf("post not removed from thread view: %#v", got.threadPosts)
	}
	if got.selectedPost != 0 {
		t.Fatalf("selectedPost = %d", got.selectedPost)
	}
}

func TestHelpTextMentionsEditAndDeleteKeys(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "  e                 edit selected own message") {
		t.Fatalf("help text missing edit row: %q", got)
	}
	if !strings.Contains(got, "  D                 delete selected own message (press twice)") {
		t.Fatalf("help text missing delete row: %q", got)
	}
}

func TestSuccessfulStaleEditDoesNotClearNewEditSession(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.editingPostID = "p2"
	m.composer.SetValue("editing p2")

	updated, _ := m.Update(postUpdatedMsg{postID: "p1", post: domain.Post{ID: "p1", ChannelID: "c1", Message: "edited p1"}})
	got := updated.(Model)
	if got.editingPostID != "p2" {
		t.Fatalf("stale edit cleared active edit session: %q", got.editingPostID)
	}
	if got.composer.Value() != "editing p2" {
		t.Fatalf("stale edit cleared composer: %q", got.composer.Value())
	}
}

func TestOpenCurrentChannelClearsEditMode(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.editingPostID = "p1"
	m.composer.SetValue("editing text")

	updated, _ := m.openCurrentChannel()
	got := updated.(Model)
	if got.editingPostID != "" {
		t.Fatalf("editingPostID not cleared on channel open: %q", got.editingPostID)
	}
}

func TestDeleteConfirmationClearsOnFocusSwitch(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.pendingDeletePostID = "p1"

	updated, _ := m.handleKey(actionKey("tab"))
	got := updated.(Model)
	if got.pendingDeletePostID != "" {
		t.Fatalf("pending delete not cleared on focus switch: %q", got.pendingDeletePostID)
	}
}

func TestSuccessfulEditPreservesLocalUnreadThreadFlags(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.editingPostID = "p1"
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Message: "old", Unread: true, Mentioned: true, ThreadUnread: true, ReplyCount: 7}}

	updated, _ := m.Update(postUpdatedMsg{postID: "p1", post: domain.Post{ID: "p1", ChannelID: "c1", UserID: "me", Message: "edited", UpdateAt: 9}})
	got := updated.(Model)
	post := got.posts[0]
	if !post.Unread || !post.Mentioned || !post.ThreadUnread || post.ReplyCount != 7 {
		t.Fatalf("local importance flags not preserved: %#v", post)
	}
}

func TestSuccessfulEditRestoresSuspendedDraft(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.loadDraft(channelDraftKey("c1"))
	m.composer.SetValue("draft text")
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "old"}}
	m.selectedPost = 0

	updated, _ := m.editSelectedPost()
	m = updated.(Model)
	updated, _ = m.Update(postUpdatedMsg{postID: "p1", post: domain.Post{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "edited"}})
	got := updated.(Model)
	if got.composer.Value() != "draft text" {
		t.Fatalf("suspended draft not restored: %q", got.composer.Value())
	}
	if got.activeDraftKey != channelDraftKey("c1") {
		t.Fatalf("activeDraftKey = %q", got.activeDraftKey)
	}
}

func TestChannelOpenDuringEditRestoresDraftInsteadOfSavingEditedText(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.session = &domain.Session{User: domain.User{ID: "me"}}
	m.channels = []domain.Channel{{ID: "c1", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	key := channelDraftKey("c1")
	m.loadDraft(key)
	m.composer.SetValue("draft text")
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", UserID: "me", Username: "You", Message: "old"}}
	m.selectedPost = 0

	updated, _ := m.editSelectedPost()
	m = updated.(Model)
	m.composer.SetValue("edited text")
	updated, _ = m.openCurrentChannel()
	got := updated.(Model)
	if got.editingPostID != "" {
		t.Fatalf("editingPostID not cleared: %q", got.editingPostID)
	}
	if got.composer.Value() != "draft text" {
		t.Fatalf("draft not restored on navigation: %q", got.composer.Value())
	}
}
