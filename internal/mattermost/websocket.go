package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"band-tui/internal/domain"

	"github.com/gorilla/websocket"
)

func (c *Client) WatchPosts(ctx context.Context, events chan<- domain.Event) error {
	if !sendEvent(ctx, events, domain.Event{Kind: domain.EventState, State: domain.ConnectionConnecting}) {
		return ctx.Err()
	}
	backoff := time.Second
	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.watchOnce(ctx, events)
		if err == nil || ctx.Err() != nil {
			return ctx.Err()
		}
		sendEvent(ctx, events, domain.Event{Kind: domain.EventError, Err: fmt.Errorf("websocket: %w", err)})
		state, retryable := watchFailureState(err)
		if !retryable {
			sendEvent(ctx, events, domain.Event{
				Kind:    domain.EventState,
				State:   state,
				Err:     err,
				Message: "refresh token and restart",
			})
			return err
		}
		attempt++
		if !sendEvent(ctx, events, domain.Event{
			Kind:    domain.EventState,
			State:   state,
			Attempt: attempt,
			RetryIn: backoff,
			Err:     err,
			Message: err.Error(),
		}) {
			return ctx.Err()
		}
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
		return &domain.BackendError{
			Op:        "websocket auth",
			Kind:      domain.BackendErrorAuth,
			Retryable: false,
			Err:       fmt.Errorf("missing token"),
		}
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
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return wrapWatchDialError(resp, err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data": map[string]string{
			"token": token,
		},
	}); err != nil {
		return wrapRequestError("websocket auth", err)
	}
	stopPing := make(chan struct{})
	defer close(stopPing)
	startWebsocketHeartbeat(ctx, conn, stopPing)
	authenticated := false
	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			return wrapRequestError("websocket read", err)
		}
		var msg wsMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			continue
		}
		if !authenticated && msg.SeqReply == 1 {
			if !strings.EqualFold(msg.Status, "OK") {
				reason := strings.TrimSpace(msg.Error)
				if reason == "" {
					reason = "websocket authentication failed"
				}
				return &domain.BackendError{
					Op:        "websocket auth",
					Kind:      domain.BackendErrorAuth,
					Retryable: false,
					Err:       errors.New(reason),
				}
			}
			authenticated = true
			if !sendEvent(ctx, events, domain.Event{Kind: domain.EventState, State: domain.ConnectionConnected}) {
				return ctx.Err()
			}
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
		case "reaction_added", "reaction_removed":
			reaction, ok := parseWSReaction(msg.Data.Reaction)
			if !ok {
				continue
			}
			kind := domain.EventReactionAdded
			if msg.Event == "reaction_removed" {
				kind = domain.EventReactionRemoved
			}
			if !sendEvent(ctx, events, domain.Event{Kind: kind, PostID: reaction.PostID, EmojiName: reaction.EmojiName, UserID: reaction.UserID}) {
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

const (
	websocketPongWait   = 75 * time.Second
	websocketPingPeriod = 25 * time.Second
)

func startWebsocketHeartbeat(ctx context.Context, conn *websocket.Conn, stop <-chan struct{}) {
	_ = conn.SetReadDeadline(time.Now().Add(websocketPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(websocketPongWait))
	})
	go func() {
		ticker := time.NewTicker(websocketPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				deadline := time.Now().Add(10 * time.Second)
				if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
					return
				}
			}
		}
	}()
}

func watchFailureState(err error) (domain.ConnectionState, bool) {
	var backendErr *domain.BackendError
	if errors.As(err, &backendErr) && backendErr.Kind == domain.BackendErrorAuth && !backendErr.Retryable {
		return domain.ConnectionAuthExpired, false
	}
	return domain.ConnectionReconnecting, true
}

func wrapWatchDialError(resp *http.Response, err error) error {
	if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		return wrapHTTPError("websocket dial", resp.StatusCode, "websocket authentication failed")
	}
	return wrapRequestError("websocket dial", err)
}

func sendEvent(ctx context.Context, events chan<- domain.Event, ev domain.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- ev:
		return true
	}
}

func parseWSReaction(raw string) (mmReaction, bool) {
	if raw == "" {
		return mmReaction{}, false
	}
	var reaction mmReaction
	if err := json.Unmarshal([]byte(raw), &reaction); err != nil {
		return mmReaction{}, false
	}
	if reaction.PostID == "" || reaction.EmojiName == "" {
		return mmReaction{}, false
	}
	return reaction, true
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
	Event    string `json:"event"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	SeqReply int    `json:"seq_reply"`
	Data     struct {
		Post     string `json:"post"`
		Reaction string `json:"reaction"`
		Mentions string `json:"mentions"`
		UserID   string `json:"user_id"`
		Status   string `json:"status"`
	} `json:"data"`
}
