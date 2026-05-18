package app

import (
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
