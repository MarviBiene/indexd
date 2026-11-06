package postgres

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// legacyAutomaticReasons are historic Host Usability reasons. They were used
// directly as blocklist entries by older indexd versions even for one-time
// price spikes. Unknown and operator-supplied reasons are NEVER removed.
const legacyAutomaticReasons = `ARRAY[
	'Uptime', 'MaxContractDuration', 'MaxCollateral', 'ProtocolVersion',
	'PriceValidity', 'PriceRecovery', 'AcceptingContracts', 'QUIC', 'Siamux',
	'ContractPrice', 'Collateral', 'StoragePrice', 'IngressPrice',
	'EgressPrice', 'FreeSectorPrice'
]::text[]`

// restoreLegacyUsabilityBlocks is deliberately a one-time upgrade operation.
// It releases only legacy automatic blocklist entries and only revives
// unexpired, unrenewed contracts whose saved reason was exclusively a legacy
// host-usability block. Manual bans and independently bad contracts are left
// unchanged. After upgrading, host usability is governed by current scans and
// a price-only quarantine, never automatic permanent blocks.
func restoreLegacyUsabilityBlocks(ctx context.Context, tx *txn, log *zap.Logger) error {
	res, err := tx.Exec(ctx, `
DELETE FROM hosts_blocklist
WHERE CARDINALITY(reasons) > 0
  AND reasons <@ `+legacyAutomaticReasons)
	if err != nil {
		return fmt.Errorf("failed to remove legacy automatic block reasons: %w", err)
	}
	releasedHosts := res.RowsAffected()

	// Some very old databases (including the v1 migration fixture) have no
	// custom bad-reason metadata. In that case there is no safe way to infer
	// which contracts were disabled solely by an automatic usability block.
	// Release the legacy blocklist entries, but do not guess contract state.
	var hasBadReasonMetadata bool
	if err := tx.QueryRow(ctx,
		"SELECT to_regclass('public.custom_contract_bad_metadata') IS NOT NULL",
	).Scan(&hasBadReasonMetadata); err != nil {
		return fmt.Errorf("failed to check legacy contract metadata schema: %w", err)
	}
	if !hasBadReasonMetadata {
		log.Info("released legacy usability blocks; contract reason metadata unavailable",
			zap.Int64("hosts", releasedHosts))
		return nil
	}

	res, err = tx.Exec(ctx, `
WITH restored AS (
	UPDATE contracts AS c SET good = TRUE
	FROM hosts AS h, custom_contract_bad_metadata AS cbm
	WHERE c.host_id = h.id
	  AND cbm.contract_id = c.contract_id
	  AND NOT c.good
	  AND c.state IN (0,1)
	  AND c.renewed_to IS NULL
	  AND c.proof_height > (SELECT scanned_height FROM global_settings)
	  AND cbm.bad_reason LIKE 'host blocked: %'
	  AND CARDINALITY(string_to_array(SUBSTRING(cbm.bad_reason FROM LENGTH('host blocked: ') + 1), ', ')) > 0
	  AND string_to_array(SUBSTRING(cbm.bad_reason FROM LENGTH('host blocked: ') + 1), ', ') <@ `+legacyAutomaticReasons+`
	  AND NOT EXISTS (
		SELECT 1 FROM hosts_blocklist AS hb WHERE hb.public_key = h.public_key
	  )
	RETURNING c.contract_id
)
DELETE FROM custom_contract_bad_metadata
WHERE contract_id IN (SELECT contract_id FROM restored)`)
	if err != nil {
		return fmt.Errorf("failed to restore paid contracts blocked by legacy usability checks: %w", err)
	}
	log.Info("released legacy usability blocks", zap.Int64("hosts", releasedHosts), zap.Int64("contracts", res.RowsAffected()))
	return nil
}
