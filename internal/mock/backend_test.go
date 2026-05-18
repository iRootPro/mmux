package mock

import (
	"context"
	"testing"
	"time"

	"band-tui/internal/domain"
)

func TestMockControlMessageEmitsState(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    domain.ConnectionState
	}{
		{name: "offline", message: "mock:offline", want: domain.ConnectionOffline},
		{name: "auth expired", message: "mock:auth-expired", want: domain.ConnectionAuthExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New()
			events := make(chan domain.Event, 2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go b.WatchPosts(ctx, events)
			waitForWatcher(t, b)

			if _, err := b.SendPost(context.Background(), "dev", tt.message); err != nil {
				t.Fatal(err)
			}
			ev := <-events
			if ev.Kind != domain.EventState || ev.State != tt.want {
				t.Fatalf("unexpected event: %#v", ev)
			}
		})
	}
}

func waitForWatcher(t *testing.T, b *Backend) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		n := len(b.watchers)
		b.mu.Unlock()
		if n != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("watcher did not register")
}

func TestSendFailureTrigger(t *testing.T) {
	b := New()
	_, err := b.SendPost(context.Background(), "dev", "fail-send")
	if err == nil {
		t.Fatal("expected fail-send to trigger mock send failure")
	}
}

func TestMockOfflineStateIncludesMessage(t *testing.T) {
	b := New()
	events := make(chan domain.Event, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.WatchPosts(ctx, events)
	waitForWatcher(t, b)

	if _, err := b.SendPost(context.Background(), "dev", "mock:offline"); err != nil {
		t.Fatal(err)
	}
	first := <-events
	second := <-events
	if first.State != domain.ConnectionOffline || first.Message == "" {
		t.Fatalf("offline state missing message: %#v", first)
	}
	if second.State != domain.ConnectionReconnecting || second.RetryIn <= 0 || second.Attempt != 1 {
		t.Fatalf("offline should transition into reconnecting: %#v", second)
	}

}

func TestMockReconnectEmitsReconnectingThenConnected(t *testing.T) {
	b := New()
	events := make(chan domain.Event, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.WatchPosts(ctx, events)
	waitForWatcher(t, b)

	if _, err := b.SendPost(context.Background(), "dev", "mock:reconnect"); err != nil {
		t.Fatal(err)
	}
	ev := <-events
	if ev.State != domain.ConnectionConnected {
		t.Fatalf("reconnect should restore connected state: %#v", ev)
	}

}

func TestMockAuthExpiredIncludesActionMessage(t *testing.T) {
	b := New()
	events := make(chan domain.Event, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.WatchPosts(ctx, events)
	waitForWatcher(t, b)

	if _, err := b.SendPost(context.Background(), "dev", "mock:auth-expired"); err != nil {
		t.Fatal(err)
	}
	ev := <-events
	if ev.State != domain.ConnectionAuthExpired || ev.Message == "" {
		t.Fatalf("auth expired state missing action message: %#v", ev)
	}
}

func TestUpdatePostMutatesStoredMessage(t *testing.T) {
	b := New()
	posts, err := b.LoadPosts(context.Background(), "dev", 0)
	if err != nil {
		t.Fatal(err)
	}
	target := posts[len(posts)-1]

	updated, err := b.UpdatePost(context.Background(), target.ID, "edited")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Message != "edited" {
		t.Fatalf("updated message = %q", updated.Message)
	}

	posts, err = b.LoadPosts(context.Background(), "dev", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, post := range posts {
		if post.ID == target.ID {
			if post.Message != "edited" {
				t.Fatalf("stored message = %q", post.Message)
			}
			return
		}
	}
	t.Fatalf("post %q not found after update", target.ID)
}

func TestDeletePostRemovesStoredMessage(t *testing.T) {
	b := New()
	posts, err := b.LoadPosts(context.Background(), "dev", 0)
	if err != nil {
		t.Fatal(err)
	}
	target := posts[len(posts)-1]

	if err := b.DeletePost(context.Background(), target.ID); err != nil {
		t.Fatal(err)
	}

	posts, err = b.LoadPosts(context.Background(), "dev", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, post := range posts {
		if post.ID == target.ID {
			t.Fatalf("post %q still present after delete", target.ID)
		}
	}
}
