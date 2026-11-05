package contracts

import "go.sia.tech/core/types"

// Stats contains statistics about the contracts in the database.
type Stats struct {
	Contracts             uint64         `json:"contracts"`
	BadContracts          uint64         `json:"badContracts"`
	HealthyContracts      uint64         `json:"healthyContracts"`
	QuarantineContracts   uint64         `json:"quarantineContracts"`
	PermanentBanContracts uint64         `json:"permanentBanContracts"`
	OtherBadContracts     uint64         `json:"otherBadContracts"`
	HealthyCapacity       uint64         `json:"healthyCapacity"`
	QuarantineCapacity    uint64         `json:"quarantineCapacity"`
	PermanentBanCapacity  uint64         `json:"permanentBanCapacity"`
	Renewing              uint64         `json:"renewing"`

	LockedAllowance       types.Currency `json:"lockedAllowance"`
	RemainingAllowance    types.Currency `json:"remainingAllowance"`
	TotalCapacity         uint64         `json:"totalCapacity"`
	TotalSize             uint64         `json:"totalSize"`
}

// ContractsStats returns statistics about the contracts in the database.
func (cm *ContractManager) ContractsStats() (Stats, error) {
	return cm.store.ContractsStats()
}
