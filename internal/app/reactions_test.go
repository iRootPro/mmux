package app

import (
	"testing"

	"band-tui/internal/domain"
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
