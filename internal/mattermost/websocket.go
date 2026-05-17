package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"band-tui/internal/domain"

	"github.com/gorilla/websocket"
)

func (c *Client) WatchPosts(ctx context.Context, events chan<- domain.Event) error {
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.watchOnce(ctx, events)
		if err == nil || ctx.Err() != nil {
			return ctx.Err()
		}
		sendEvent(ctx, events, domain.Event{Kind: domain.EventError, Err: fmt.Errorf("websocket: %w", err)})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) watchOnce(ctx context.Context, events chan<- domain.Event) error {
	token := c.currentToken()
	if token == "" {
		return fmt.Errorf("missing token")
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v4/websocket"
	u.RawQuery = ""

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.WriteJSON(map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data": map[string]string{
			"token": token,
		},
	})

	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg wsMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			continue
		}
		switch msg.Event {
		case "posted":
			rawPost := msg.Data.Post
			if rawPost == "" {
				continue
			}
			var post mmPost
			if err := json.Unmarshal([]byte(rawPost), &post); err != nil {
				continue
			}
			if post.DeleteAt != 0 {
				continue
			}
			out := post.toDomain(post.ChannelID)
			out.Username = c.usernameFor(out.UserID)
			out.Mentioned = c.mentionsCurrentUser(msg.Data.Mentions)
			if !sendEvent(ctx, events, domain.Event{Kind: domain.EventPost, Post: out}) {
				return ctx.Err()
			}
		case "status_change":
			if msg.Data.UserID != "" && msg.Data.Status != "" {
				if !sendEvent(ctx, events, domain.Event{Kind: domain.EventStatus, UserID: msg.Data.UserID, Status: msg.Data.Status}) {
					return ctx.Err()
				}
			}
		}
	}
}

func sendEvent(ctx context.Context, events chan<- domain.Event, ev domain.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- ev:
		return true
	}
}

func (c *Client) mentionsCurrentUser(raw string) bool {
	userID := c.currentUserID()
	if userID == "" || raw == "" {
		return false
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		for _, id := range ids {
			if id == userID {
				return true
			}
		}
		return false
	}
	return strings.Contains(raw, userID)
}

type wsMessage struct {
	Event string `json:"event"`
	Data  struct {
		Post     string `json:"post"`
		Mentions string `json:"mentions"`
		UserID   string `json:"user_id"`
		Status   string `json:"status"`
	} `json:"data"`
}
