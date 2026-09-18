package slabs

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go.sia.tech/core/types"
	"go.sia.tech/indexd/alerts"
)

// ObjectStats reports statistics about the objects tracked by this instance.
type ObjectStats struct {
	// UnpublishedEvents is the number of object events still waiting for a
	// position in the stream.
	UnpublishedEvents int64 `json:"unpublishedEvents"`
}

// ObjectStats reports statistics about the objects tracked by this instance.
func (m *SlabManager) ObjectStats() (ObjectStats, error) {
	return m.store.ObjectStats()
}

// SectorsStats reports statistics about the sectors and slabs stored in the
// database.
type SectorsStats struct {
	Slabs                    int64   `json:"slabs"`
	Migrated                 int64   `json:"migrated"`
	Pinned                   int64   `json:"pinned"`
	Unpinnable               int64   `json:"unpinnable"`
	Unpinned                 int64   `json:"unpinned"`
	Lost                     int64   `json:"lost"`
	Checked                  int64   `json:"checked"`
	CheckFailed              int64   `json:"checkFailed"`
	UnrecoverableSlabs       int64   `json:"unrecoverableSlabs"`
	StuckSlabs               int64   `json:"stuckSlabs"`
	RepairThreshold          int     `json:"repairThreshold"`
	DegradedSlabs            int64   `json:"degradedSlabs"`
	WaitingForThresholdSlabs int64   `json:"waitingForThresholdSlabs"`
	ToMigrateSlabs           int64   `json:"toMigrateSlabs"`
	ReadyToMigrateSlabs      int64   `json:"readyToMigrateSlabs"`
	DeferredMigrationSlabs   int64   `json:"deferredMigrationSlabs"`
	DegradedSectors          int64   `json:"degradedSectors"`
	RetryingSlabs            int64   `json:"retryingSlabs"`
	WorstSlabHealth          float64 `json:"worstSlabHealth"`
	WorstSlabGoodSectors     int64   `json:"worstSlabGoodSectors"`
	WorstSlabTotalSectors    int64   `json:"worstSlabTotalSectors"`
	WorstSlabMinShards       int64   `json:"worstSlabMinShards"`
	WorstSlabDegradedSectors int64   `json:"worstSlabDegradedSectors"`
	WorstSlabRecoveryMargin  int64   `json:"worstSlabRecoveryMargin"`
	WorstSlabID              string  `json:"worstSlabID,omitempty"`
}

// SectorStats reports statistics about the sectors and slabs stored in the
// database.
func (m *SlabManager) SectorStats() (SectorsStats, error) {
	return m.store.SectorStats(m.repairThreshold)
}

func slabHealthAlertID() types.Hash256 {
	return types.Hash256(sha256.Sum256([]byte("indexd:slab-health-below-threshold")))
}

func (m *SlabManager) updateSlabHealthAlert(_ context.Context) error {
	stats, err := m.store.SectorStats(m.repairThreshold)
	if err != nil {
		return fmt.Errorf("failed to load sector stats: %w", err)
	}

	id := slabHealthAlertID()
	if stats.WorstSlabTotalSectors == 0 || stats.WorstSlabHealth >= m.healthAlertThreshold {
		m.alerter.DismissAlerts(id)
		return nil
	}

	severity := alerts.SeverityWarning
	if stats.WorstSlabRecoveryMargin <= 0 {
		severity = alerts.SeverityCritical
	}
	return m.alerter.RegisterAlert(alerts.Alert{
		ID:       id,
		Severity: severity,
		Message:  "Slab health below configured threshold",
		Data: map[string]any{
			"slabID":         stats.WorstSlabID,
			"health":         stats.WorstSlabHealth,
			"threshold":      m.healthAlertThreshold,
			"goodSectors":    stats.WorstSlabGoodSectors,
			"totalSectors":   stats.WorstSlabTotalSectors,
			"minShards":      stats.WorstSlabMinShards,
			"recoveryMargin": stats.WorstSlabRecoveryMargin,
		},
		Timestamp: time.Now(),
	})
}
