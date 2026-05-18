package app

import (
	"context"
	"strings"
	"testing"

	"band-tui/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func TestReactionStateFindsExistingReaction(t *testing.T) {
	post := domain.Post{Reactions: []domain.PostReaction{{Name: "+1", Count: 2, Reacted: true}}}
	reaction, ok := reactionState(post, "+1")
	if !ok {
		t.Fatal("expected reaction state")
	}
	if reaction.Name != "+1" || reaction.Count != 2 || !reaction.Reacted {
		t.Fatalf("reaction = %#v", reaction)
	}
}

func TestMergeAddedReactionAddsMissingEmoji(t *testing.T) {
	post := domain.Post{}
	updated := mergeAddedReaction(post, "+1")
	if len(updated.Reactions) != 1 || updated.Reactions[0].Name != "+1" || updated.Reactions[0].Count != 1 || !updated.Reactions[0].Reacted {
		t.Fatalf("reactions = %#v", updated.Reactions)
	}
}

func TestMergeRemovedReactionRemovesOwnReaction(t *testing.T) {
	post := domain.Post{Reactions: []domain.PostReaction{{Name: "+1", Count: 2, Reacted: true}}}
	updated := mergeRemovedReaction(post, "+1")
	if len(updated.Reactions) != 1 || updated.Reactions[0].Count != 1 || updated.Reactions[0].Reacted {
		t.Fatalf("reactions = %#v", updated.Reactions)
	}
}

func TestMergeRemovedReactionDropsZeroCountReaction(t *testing.T) {
	post := domain.Post{Reactions: []domain.PostReaction{{Name: "+1", Count: 1, Reacted: true}}}
	updated := mergeRemovedReaction(post, "+1")
	if len(updated.Reactions) != 0 {
		t.Fatalf("reactions = %#v", updated.Reactions)
	}
}

func TestHandleTimelineKeyROpensReactionPicker(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.focus = focusTimeline
	m.posts = []domain.Post{{ID: "p1", Message: "hello"}}
	m.selectedPost = 0

	updated, _ := m.handleKey(actionKey("R"))
	got := updated.(Model)
	if !got.reactionPickerOpen {
		t.Fatal("reaction picker should open")
	}
}

func TestHandleReactionPickerEscCloses(t *testing.T) {
	m := Model{reactionPickerOpen: true}
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.reactionPickerOpen {
		t.Fatal("reaction picker should close")
	}
}

func TestRenderReactionPickerShowsChoices(t *testing.T) {
	m := Model{reactionPickerOpen: true}
	got := m.renderReactionPicker(120, 30)
	if !strings.Contains(got, "👍") || !strings.Contains(got, "👀") || !strings.Contains(got, "✅") {
		t.Fatalf("picker render = %q", got)
	}
}

type reactionRecordingBackend struct {
	addPostID    string
	addEmoji     string
	removePostID string
	removeEmoji  string
	addErr       error
	removeErr    error
}

func (b *reactionRecordingBackend) Connect(context.Context) (*domain.Session, error) { return nil, nil }
func (b *reactionRecordingBackend) LoadChannels(context.Context, string) ([]domain.Channel, error) {
	return nil, nil
}
func (b *reactionRecordingBackend) LoadPosts(context.Context, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *reactionRecordingBackend) LoadPostsBefore(context.Context, string, string, int) ([]domain.Post, error) {
	return nil, nil
}
func (b *reactionRecordingBackend) ViewChannel(context.Context, string) error { return nil }
func (b *reactionRecordingBackend) LoadThread(context.Context, string) ([]domain.Post, error) {
	return nil, nil
}
func (b *reactionRecordingBackend) SendPost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *reactionRecordingBackend) SendReply(context.Context, string, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *reactionRecordingBackend) UpdatePost(context.Context, string, string) (domain.Post, error) {
	return domain.Post{}, nil
}
func (b *reactionRecordingBackend) DeletePost(context.Context, string) error { return nil }
func (b *reactionRecordingBackend) AddReaction(_ context.Context, postID, emojiName string) (domain.Post, error) {
	b.addPostID = postID
	b.addEmoji = emojiName
	if b.addErr != nil {
		return domain.Post{}, b.addErr
	}
	return domain.Post{ID: postID, ChannelID: "c1", Reactions: []domain.PostReaction{{Name: emojiName, Count: 1, Reacted: true}}}, nil
}
func (b *reactionRecordingBackend) RemoveReaction(_ context.Context, postID, emojiName string) (domain.Post, error) {
	b.removePostID = postID
	b.removeEmoji = emojiName
	if b.removeErr != nil {
		return domain.Post{}, b.removeErr
	}
	return domain.Post{ID: postID, ChannelID: "c1"}, nil
}
func (b *reactionRecordingBackend) WatchPosts(context.Context, chan<- domain.Event) error { return nil }
func (b *reactionRecordingBackend) Close() error                                          { return nil }

func TestReactionPickerEnterAddsReaction(t *testing.T) {
	backend := &reactionRecordingBackend{}
	m := New(backend, testConfig(), false)
	m.focus = focusTimeline
	m.reactionPickerOpen = true
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
	m.selectedPost = 0

	updated, cmd := m.handleReactionPickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.status != "toggling reaction…" {
		t.Fatalf("status = %q", got.status)
	}
	if cmd == nil {
		t.Fatal("expected toggle command")
	}
	msg := cmd()
	toggled, ok := msg.(reactionToggledMsg)
	if !ok {
		t.Fatalf("command msg = %#v", msg)
	}
	if !toggled.added || backend.addPostID != "p1" || backend.addEmoji != defaultReactions[0].Name {
		t.Fatalf("add path not used: msg=%#v backend=%#v", toggled, backend)
	}
}

func TestReactionPickerEnterRemovesExistingReaction(t *testing.T) {
	backend := &reactionRecordingBackend{}
	m := New(backend, testConfig(), false)
	m.focus = focusTimeline
	m.reactionPickerOpen = true
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello", Reactions: []domain.PostReaction{{Name: defaultReactions[0].Name, Count: 1, Reacted: true}}}}
	m.selectedPost = 0

	updated, cmd := m.handleReactionPickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.status != "toggling reaction…" {
		t.Fatalf("status = %q", got.status)
	}
	if cmd == nil {
		t.Fatal("expected toggle command")
	}
	msg := cmd()
	toggled, ok := msg.(reactionToggledMsg)
	if !ok {
		t.Fatalf("command msg = %#v", msg)
	}
	if toggled.added || backend.removePostID != "p1" || backend.removeEmoji != defaultReactions[0].Name {
		t.Fatalf("remove path not used: msg=%#v backend=%#v", toggled, backend)
	}
}

func TestSuccessfulReactionUpdateReplacesVisibleCachedAndThreadCopies(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
	m.postsByChannel = map[string][]domain.Post{"c1": {{ID: "p1", ChannelID: "c1", Message: "hello"}}}
	m.threadPosts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
	m.reactionPickerOpen = true

	updated, _ := m.Update(reactionToggledMsg{post: domain.Post{ID: "p1", ChannelID: "c1", Message: "hello", Reactions: []domain.PostReaction{{Name: "+1", Count: 1, Reacted: true}}}, added: true})
	got := updated.(Model)
	if !got.posts[0].Reactions[0].Reacted || !got.postsByChannel["c1"][0].Reactions[0].Reacted || !got.threadPosts[0].Reactions[0].Reacted {
		t.Fatalf("reaction update not propagated: %#v %#v %#v", got.posts, got.postsByChannel["c1"], got.threadPosts)
	}
	if got.reactionPickerOpen {
		t.Fatal("picker should close after success")
	}
	if got.status != "reaction added" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestFailedReactionToggleLeavesStateUnchanged(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "hello"}}
	m.reactionPickerOpen = true

	updated, _ := m.Update(reactionToggledMsg{err: assertErr{}})
	got := updated.(Model)
	if len(got.posts[0].Reactions) != 0 {
		t.Fatalf("state changed on failure: %#v", got.posts[0].Reactions)
	}
	if got.reactionPickerOpen {
		t.Fatal("picker should close on failure")
	}
	if got.status != "reaction failed" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestRenderPostShowsReactionBadges(t *testing.T) {
	m := Model{
		posts: []domain.Post{{ID: "p1", Username: "Alice", Message: "hello", Reactions: []domain.PostReaction{
			{Name: "+1", Count: 2, Reacted: true},
			{Name: "eyes", Count: 1},
		}}},
		selectedPost: -1,
	}
	got, _ := m.renderPosts()
	if !strings.Contains(got, "👍 2") || !strings.Contains(got, "👀 1") {
		t.Fatalf("rendered posts missing reactions: %q", got)
	}
}

func TestHelpTextMentionsReactionKey(t *testing.T) {
	m := Model{}
	got := m.helpText()
	if !strings.Contains(got, "R") || !strings.Contains(strings.ToLower(got), "reaction") {
		t.Fatalf("help text missing reaction key: %q", got)
	}
}

func TestHandleThreadKeyROpensReactionPickerForSelectedThreadPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.threadOpen = true
	m.threadPosts = []domain.Post{
		{ID: "root", ChannelID: "c1", Message: "root"},
		{ID: "r1", ChannelID: "c1", RootID: "root", Message: "reply"},
	}
	m.threadSelected = 1

	updated, _ := m.handleThreadKey(actionKey("R"))
	got := updated.(Model)
	if !got.reactionPickerOpen {
		t.Fatal("reaction picker should open from thread messages")
	}
	if got.reactionTargetPostID != "r1" {
		t.Fatalf("reactionTargetPostID = %q", got.reactionTargetPostID)
	}
}

func TestReactionPickerTargetsSelectedThreadPost(t *testing.T) {
	m := New(noopBackend{}, testConfig(), false)
	m.reactionPickerOpen = true
	m.reactionTargetKind = reactionTargetThread
	m.reactionTargetPostID = "r1"
	m.threadPosts = []domain.Post{
		{ID: "root", ChannelID: "c1", Message: "root"},
		{ID: "r1", ChannelID: "c1", RootID: "root", Message: "reply"},
	}
	m.posts = []domain.Post{{ID: "p1", ChannelID: "c1", Message: "timeline"}}
	target, ok := m.selectedReactionTarget()
	if !ok || target.ID != "r1" {
		t.Fatalf("selected reaction target = %#v ok=%v", target, ok)
	}
}
