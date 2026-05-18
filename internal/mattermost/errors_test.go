package mattermost

import (
	"context"
	"errors"
	"testing"

	"band-tui/internal/domain"
)

func TestWrapHTTPErrorClassifiesAuthFailures(t *testing.T) {
	err := wrapHTTPError("get current user", 401, "unauthorized")
	assertBackendError(t, err, domain.BackendErrorAuth, 401, false)
}

func TestWrapHTTPErrorClassifiesServerFailures(t *testing.T) {
	err := wrapHTTPError("get posts", 503, "unavailable")
	assertBackendError(t, err, domain.BackendErrorServer, 503, true)
}

func TestWrapRequestErrorClassifiesNetworkFailures(t *testing.T) {
	err := wrapRequestError("get current user", context.DeadlineExceeded)
	assertBackendError(t, err, domain.BackendErrorNetwork, 0, true)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped deadline exceeded, got %v", err)
	}
}

func TestWrapRequestErrorClassifiesCanceledContextAsNonRetryable(t *testing.T) {
	err := wrapRequestError("get current user", context.Canceled)
	assertBackendError(t, err, domain.BackendErrorNetwork, 0, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected wrapped context canceled, got %v", err)
	}
}

func TestWrapHTTPErrorClassifiesUnknownFailures(t *testing.T) {
	err := wrapHTTPError("get current user", 429, "rate limited")
	assertBackendError(t, err, domain.BackendErrorUnknown, 429, false)
}

func assertBackendError(t *testing.T, err error, wantKind domain.BackendErrorKind, wantStatus int, wantRetryable bool) {
	t.Helper()
	var be *domain.BackendError
	if !errors.As(err, &be) {
		t.Fatalf("expected BackendError, got %T", err)
	}
	if be.Kind != wantKind || be.StatusCode != wantStatus || be.Retryable != wantRetryable {
		t.Fatalf("unexpected backend error: %#v", be)
	}
}
