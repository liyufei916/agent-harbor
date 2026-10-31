package kits

import (
	"context"
	"testing"
)

func TestDockerGuestConstantsAndProbeFailure(t *testing.T) {
	if GuestHome == "" || GuestNode == "" || GuestClaude == "" || GuestCodex == "" || GuestDSHScript == "" {
		t.Fatal("guest paths incomplete")
	}
	if _, err := ProbeDockerGuest(context.Background(), "relative", nil); err == nil {
		t.Fatal("relative guest command accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ProbeDockerGuest(ctx, "/bin/true", nil); err == nil {
		t.Fatal("cancelled guest probe succeeded")
	}
	_ = DockerGuest()
}
