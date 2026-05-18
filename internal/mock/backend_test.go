package mock

import (
	"context"
	"testing"
)

func TestSendFailureTrigger(t *testing.T) {
	b := New()
	_, err := b.SendPost(context.Background(), "dev", "fail-send")
	if err == nil {
		t.Fatal("expected fail-send to trigger mock send failure")
	}
}
