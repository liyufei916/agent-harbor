package desktop

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestHostsForSecurityTier(t *testing.T) {
	if n := len(hostsForSecurityTier("strict")); n != 0 {
		t.Fatalf("strict want 0 got %d", n)
	}
	if n := len(hostsForSecurityTier("standard")); n < 5 {
		t.Fatalf("standard too few: %d", n)
	}
	if n := len(hostsForSecurityTier("relaxed")); n <= len(hostsForSecurityTier("standard")) {
		t.Fatalf("relaxed should include more hosts")
	}
}

func TestMergeRememberedWithTier(t *testing.T) {
	existing := []contracts.RememberedHost{{Host: "registry.npmjs.org", Workspace: "/p", Scope: "project"}}
	got := mergeRememberedWithTier(existing, "/p", "standard")
	if len(got) < 2 {
		t.Fatalf("want tier hosts merged, got %#v", got)
	}
	count := 0
	for _, h := range got {
		if h.Host == "registry.npmjs.org" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("should not duplicate npmjs, count=%d", count)
	}
}
