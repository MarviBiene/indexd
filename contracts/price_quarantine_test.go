package contracts

import "testing"

func TestPricePausedContractsCountTowardPaidTarget(t *testing.T) {
	tests := []struct {
		name                              string
		healthy, pricePaused, otherPaused int
		wanted                            uint64
		expected                          int
	}{
		{"price-only pauses never force replacements", 30, 58, 0, 88, 88},
		{"mixed pause keeps full price budget", 30, 40, 30, 88, 87},
		{"unrelated outage budget is capped", 30, 0, 58, 88, 47},
		{"healthy pool", 88, 0, 0, 88, 88},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contractTargetCount(tc.healthy, tc.pricePaused, tc.otherPaused, tc.wanted); got != tc.expected {
				t.Fatalf("expected %d contract equivalents, got %d", tc.expected, got)
			}
		})
	}
}
