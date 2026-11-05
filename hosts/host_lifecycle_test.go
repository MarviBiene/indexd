package hosts

import "testing"

func TestLifecycleStatus(t *testing.T) {
	h := Host{Usability: GoodUsability}
	if got := h.LifecycleStatus(); got != "healthy" {
		t.Fatalf("healthy host: got %q", got)
	}
	h.Usability.IngressPrice = false
	if got := h.LifecycleStatus(); got != "quarantine" {
		t.Fatalf("price excursion: got %q", got)
	}
	h.Blocked = true
	h.BlockedReasons = []string{"IngressPrice"}
	if got := h.LifecycleStatus(); got != "quarantine" {
		t.Fatalf("legacy automatic block: got %q", got)
	}
	h.BlockedReasons = []string{"IngressPrice", "ManualBan"}
	if got := h.LifecycleStatus(); got != "permanentBan" {
		t.Fatalf("manual ban: got %q", got)
	}
	h.BlockedReasons = []string{"malicious integrity failure"}
	if got := h.LifecycleStatus(); got != "permanentBan" {
		t.Fatalf("unknown manual reason: got %q", got)
	}
	h.Blocked = false
	h.BlockedReasons = nil
	h.Usability = GoodUsability
	if got := h.LifecycleStatus(); got != "healthy" {
		t.Fatalf("recovered host: got %q", got)
	}
}
