package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"band-tui/internal/domain"
)

type Backend struct {
	mu       sync.Mutex
	channels []domain.Channel
	posts    map[string][]domain.Post
	watchers []chan<- domain.Event
	nextID   int
}

func New() *Backend {
	now := time.Now()
	channels := []domain.Channel{
		{ID: "town-square", TeamID: "wb", Name: "town-square", DisplayName: "town square", Type: "O"},
		{ID: "dev", TeamID: "wb", Name: "dev", DisplayName: "dev", Type: "O", Unread: 2, LastPostAt: now.Add(-6 * time.Minute).UnixMilli()},
		{ID: "random", TeamID: "wb", Name: "random", DisplayName: "random", Type: "O", Mentions: 1, Unread: 1, LastPostAt: now.Add(-5 * time.Minute).UnixMilli()},
		{ID: "dm-alisa", TeamID: "wb", Name: "alisa", DisplayName: "alisa", Type: "D", LastPostAt: now.Add(-8 * time.Minute).UnixMilli()},
	}
	posts := map[string][]domain.Post{}
	for _, ch := range channels {
		items := make([]domain.Post, 0, 40)
		for i := 40; i >= 1; i-- {
			items = append(items, domain.Post{
				ID:        fmt.Sprintf("%s-%02d", ch.ID, 41-i),
				ChannelID: ch.ID,
				Username:  "system",
				Message:   fmt.Sprintf("Historical mock message %d in %s", 41-i, ch.DisplayName),
				CreateAt:  now.Add(-time.Duration(i) * time.Hour).UnixMilli(),
			})
		}
		items = append(items,
			domain.Post{ID: ch.ID + "-latest-1", ChannelID: ch.ID, Username: "system", Message: "Welcome to " + ch.DisplayName + ". This is mock mode.", CreateAt: now.Add(-20 * time.Minute).UnixMilli()},
			domain.Post{ID: ch.ID + "-latest-2", ChannelID: ch.ID, Username: "alisa", Message: "Минималистичный TUI уже почти живой ✨", CreateAt: now.Add(-8 * time.Minute).UnixMilli()},
		)
		posts[ch.ID] = items
	}
	return &Backend{channels: channels, posts: posts, nextID: 100}
}

func (b *Backend) Connect(ctx context.Context) (*domain.Session, error) {
	return &domain.Session{
		ServerURL: "mock://band.wb.ru",
		User:      domain.User{ID: "me", Username: "you", Nickname: "You"},
		Teams:     []domain.Team{{ID: "wb", Name: "wb", DisplayName: "WB Band"}},
		Emojis:    []domain.Emoji{{Name: "party_blob"}, {Name: "ship_it"}, {Name: "this_is_fine"}},
	}, nil
}

func (b *Backend) LoadChannels(ctx context.Context, teamID string) ([]domain.Channel, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := append([]domain.Channel(nil), b.channels...)
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName) })
	return out, nil
}

func (b *Backend) LoadPosts(ctx context.Context, channelID string, limit int) ([]domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	posts := append([]domain.Post(nil), b.posts[channelID]...)
	if limit > 0 && len(posts) > limit {
		posts = posts[len(posts)-limit:]
	}
	return posts, nil
}

func (b *Backend) LoadPostsBefore(ctx context.Context, channelID, beforePostID string, limit int) ([]domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	posts := b.posts[channelID]
	end := len(posts)
	for i, post := range posts {
		if post.ID == beforePostID {
			end = i
			break
		}
	}
	start := 0
	if limit > 0 && end > limit {
		start = end - limit
	}
	return append([]domain.Post(nil), posts[start:end]...), nil
}

func (b *Backend) ViewChannel(ctx context.Context, channelID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.channels {
		if b.channels[i].ID == channelID {
			b.channels[i].Unread = 0
			b.channels[i].Mentions = 0
			return nil
		}
	}
	return nil
}

func (b *Backend) LoadThread(ctx context.Context, postID string) ([]domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, posts := range b.posts {
		for _, post := range posts {
			if post.ID == postID {
				return []domain.Post{
					post,
					{ID: postID + "-reply-1", ChannelID: post.ChannelID, RootID: post.ID, Username: "alisa", Message: "Mock thread reply", CreateAt: time.Now().Add(-3 * time.Minute).UnixMilli()},
				}, nil
			}
		}
	}
	return nil, nil
}

func (b *Backend) SendPost(ctx context.Context, channelID, message string) (domain.Post, error) {
	return b.sendPost(channelID, "", message)
}

func (b *Backend) SendReply(ctx context.Context, channelID, rootID, message string) (domain.Post, error) {
	return b.sendPost(channelID, rootID, message)
}

func (b *Backend) UpdatePost(ctx context.Context, postID, message string) (domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for channelID, posts := range b.posts {
		for i := range posts {
			if posts[i].ID != postID {
				continue
			}
			posts[i].Message = message
			b.posts[channelID][i] = posts[i]
			return posts[i], nil
		}
	}
	return domain.Post{}, fmt.Errorf("post %q not found", postID)
}

func (b *Backend) DeletePost(ctx context.Context, postID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for channelID, posts := range b.posts {
		for i := range posts {
			if posts[i].ID != postID {
				continue
			}
			b.posts[channelID] = append(posts[:i], posts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("post %q not found", postID)
}

func (b *Backend) AddReaction(ctx context.Context, postID, emojiName string) (domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for channelID, posts := range b.posts {
		for i := range posts {
			if posts[i].ID != postID {
				continue
			}
			posts[i] = mergeAddedMockReaction(posts[i], emojiName)
			b.posts[channelID][i] = posts[i]
			return posts[i], nil
		}
	}
	return domain.Post{}, fmt.Errorf("post %q not found", postID)
}

func (b *Backend) RemoveReaction(ctx context.Context, postID, emojiName string) (domain.Post, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for channelID, posts := range b.posts {
		for i := range posts {
			if posts[i].ID != postID {
				continue
			}
			posts[i] = mergeRemovedMockReaction(posts[i], emojiName)
			b.posts[channelID][i] = posts[i]
			return posts[i], nil
		}
	}
	return domain.Post{}, fmt.Errorf("post %q not found", postID)
}

func (b *Backend) sendPost(channelID, rootID, message string) (domain.Post, error) {
	message = strings.TrimSpace(message)
	if message == "fail-send" {
		return domain.Post{}, fmt.Errorf("mock send failure")
	}
	if events, ok := mockControlEvents(message); ok {
		for _, ev := range events {
			b.broadcast(ev)
		}
		return domain.Post{}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	post := domain.Post{
		ID:        fmt.Sprintf("mock-%d", b.nextID),
		ChannelID: channelID,
		RootID:    rootID,
		UserID:    "me",
		Username:  "you",
		Message:   message,
		CreateAt:  time.Now().UnixMilli(),
	}
	b.posts[channelID] = append(b.posts[channelID], post)
	return post, nil
}

func (b *Backend) WatchPosts(ctx context.Context, events chan<- domain.Event) error {
	b.addWatcher(events)
	defer b.removeWatcher(events)
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			post := domain.Post{ID: fmt.Sprintf("tick-%d", t.Unix()), ChannelID: "dev", Username: "bot", Message: "mock live message", CreateAt: t.UnixMilli()}
			b.mu.Lock()
			b.posts[post.ChannelID] = append(b.posts[post.ChannelID], post)
			b.mu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case events <- domain.Event{Kind: domain.EventPost, Post: post}:
			}
		}
	}
}
func (b *Backend) addWatcher(events chan<- domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.watchers = append(b.watchers, events)
}

func (b *Backend) removeWatcher(events chan<- domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.watchers {
		if b.watchers[i] == events {
			b.watchers = append(b.watchers[:i], b.watchers[i+1:]...)
			return
		}
	}
}

func (b *Backend) broadcast(ev domain.Event) {
	b.mu.Lock()
	watchers := append([]chan<- domain.Event(nil), b.watchers...)
	b.mu.Unlock()
	for _, watcher := range watchers {
		watcher <- ev
	}
}

func mergeAddedMockReaction(post domain.Post, emojiName string) domain.Post {
	for i := range post.Reactions {
		if post.Reactions[i].Name != emojiName {
			continue
		}
		post.Reactions[i].Count++
		post.Reactions[i].Reacted = true
		return post
	}
	post.Reactions = append(post.Reactions, domain.PostReaction{Name: emojiName, Count: 1, Reacted: true})
	return post
}

func mergeRemovedMockReaction(post domain.Post, emojiName string) domain.Post {
	for i := range post.Reactions {
		if post.Reactions[i].Name != emojiName {
			continue
		}
		if post.Reactions[i].Count <= 1 {
			post.Reactions = append(post.Reactions[:i], post.Reactions[i+1:]...)
			return post
		}
		post.Reactions[i].Count--
		post.Reactions[i].Reacted = false
		return post
	}
	return post
}

func mockControlEvents(message string) ([]domain.Event, bool) {
	switch message {
	case "mock:offline":
		return []domain.Event{
			{
				Kind:    domain.EventState,
				State:   domain.ConnectionOffline,
				Message: "showing cached messages",
			},
			{
				Kind:    domain.EventState,
				State:   domain.ConnectionReconnecting,
				Attempt: 1,
				RetryIn: time.Second,
				Message: "mock reconnect",
			},
		}, true
	case "mock:reconnect":
		return []domain.Event{{
			Kind:  domain.EventState,
			State: domain.ConnectionConnected,
		}}, true
	case "mock:auth-expired":
		return []domain.Event{{
			Kind:    domain.EventState,
			State:   domain.ConnectionAuthExpired,
			Message: "refresh token and restart",
		}}, true
	default:
		return nil, false
	}
}

func (b *Backend) Close() error { return nil }
