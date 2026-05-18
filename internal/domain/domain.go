package domain

import (
	"context"
	"strconv"
	"time"
)

// User is the authenticated Mattermost user.
type User struct {
	ID          string
	Username    string
	Nickname    string
	FirstName   string
	LastName    string
	DisplayName string
	Email       string
}

// Team is a Mattermost team visible to the user.
type Team struct {
	ID          string
	Name        string
	DisplayName string
}

// Channel is a public/private channel or a direct/group message channel.
type Channel struct {
	ID           string
	TeamID       string
	Name         string
	DisplayName  string
	Type         string
	Unread       int
	Mentions     int
	LastPostAt   int64
	LastViewedAt int64
	Header       string
	Purpose      string
	MemberCount  int
	UserIDs      []string
	Status       string
}

type PostReaction struct {
	Name    string
	Count   int
	Reacted bool
}

// Post is a normalized chat message.
type Post struct {
	ID           string
	ChannelID    string
	RootID       string
	UserID       string
	Username     string
	Message      string
	Mentioned    bool
	Unread       bool
	ThreadUnread bool
	CreateAt     int64
	UpdateAt     int64
	ReplyCount   int
	Reactions    []PostReaction
}

// Session contains initial data returned after connecting.
type Session struct {
	ServerURL string
	User      User
	Teams     []Team
}

type ConnectionState string

const (
	ConnectionConnecting   ConnectionState = "connecting"
	ConnectionConnected    ConnectionState = "connected"
	ConnectionReconnecting ConnectionState = "reconnecting"
	ConnectionOffline      ConnectionState = "offline"
	ConnectionAuthExpired  ConnectionState = "auth_expired"
)

type BackendErrorKind string

const (
	BackendErrorAuth    BackendErrorKind = "auth"
	BackendErrorNetwork BackendErrorKind = "network"
	BackendErrorServer  BackendErrorKind = "server"
	BackendErrorUnknown BackendErrorKind = "unknown"
)

type BackendError struct {
	Op         string
	Kind       BackendErrorKind
	StatusCode int
	Retryable  bool
	Err        error
}

func (e *BackendError) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := e.Op
	if msg == "" {
		msg = "backend error"
	}
	if e.StatusCode != 0 {
		msg += ": status " + strconv.Itoa(e.StatusCode)
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *BackendError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Event is emitted by a backend watcher.
type Event struct {
	Kind    string
	Post    Post
	UserID  string
	Status  string
	State   ConnectionState
	Attempt int
	RetryIn time.Duration
	Message string
	Err     error
}

const (
	EventPost   = "post"
	EventStatus = "status"
	EventError  = "error"
	EventState  = "state"
)

// Backend is the minimal chat backend the TUI needs. Mattermost and mock
// implementations both satisfy it, keeping the UI independently testable.
type Backend interface {
	Connect(ctx context.Context) (*Session, error)
	LoadChannels(ctx context.Context, teamID string) ([]Channel, error)
	LoadPosts(ctx context.Context, channelID string, limit int) ([]Post, error)
	LoadPostsBefore(ctx context.Context, channelID, beforePostID string, limit int) ([]Post, error)
	ViewChannel(ctx context.Context, channelID string) error
	LoadThread(ctx context.Context, postID string) ([]Post, error)
	SendPost(ctx context.Context, channelID, message string) (Post, error)
	SendReply(ctx context.Context, channelID, rootID, message string) (Post, error)
	UpdatePost(ctx context.Context, postID, message string) (Post, error)
	DeletePost(ctx context.Context, postID string) error
	AddReaction(ctx context.Context, postID, emojiName string) (Post, error)
	RemoveReaction(ctx context.Context, postID, emojiName string) (Post, error)
	WatchPosts(ctx context.Context, events chan<- Event) error
	Close() error
}
