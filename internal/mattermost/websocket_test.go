package mattermost

import (
	"errors"
	"net/http"
	"testing"

	"band-tui/internal/domain"
)

func TestMentionsCurrentUser(t *testing.T) {
	c := &Client{userID: "me"}
	if !c.mentionsCurrentUser(`["other","me"]`) {
		t.Fatal("expected user mention")
	}
	if c.mentionsCurrentUser(`["other"]`) {
		t.Fatal("unexpected mention")
	}
}

func TestWatchFailureStateFromBackendError(t *testing.T) {
	state, retryable := watchFailureState(&domain.BackendError{Kind: domain.BackendErrorAuth, Retryable: false})
	if state != domain.ConnectionAuthExpired || retryable {
		t.Fatalf("unexpected auth state: %q retryable=%v", state, retryable)
	}
}

func TestWrapWatchDialErrorClassifiesUnauthorizedUpgrade(t *testing.T) {
	err := wrapWatchDialError(&http.Response{StatusCode: http.StatusUnauthorized}, errors.New("bad handshake"))
	var backendErr *domain.BackendError
	if !errors.As(err, &backendErr) {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if backendErr.Kind != domain.BackendErrorAuth || backendErr.Retryable {
		t.Fatalf("unexpected websocket dial classification: %#v", backendErr)
	}
}
