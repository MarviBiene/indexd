package hosts

import "testing"

func TestLifecycleStatus(t *testing.T) {
	h := Host{Usability: GoodUsability}
	if got := h.LifecycleStatus(); got != "healthy" {
		t.Fatalf("healthy host: got %q", got)
	}
	h.Usability.IngressPrice = false
	if !h.PricePaused() {
		t.Fatal("isolated ingress-price failure should pause only new writes")
	}
	if got := h.LifecycleStatus(); got != "quarantine" {
		t.Fatalf("price excursion: got %q", got)
	}
	h.Usability.Uptime = false
	if h.PricePaused() {
		t.Fatal("offline host is not a price-only quarantine")
	}
	h.Usability.Uptime = true
	h.Blocked = true
	if h.PricePaused() {
		t.Fatal("blocked host must not count as price-only paused")
	}
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
