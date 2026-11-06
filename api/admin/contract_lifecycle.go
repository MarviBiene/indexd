package admin

import (
	"context"
	"fmt"

	"go.sia.tech/core/types"
	"go.sia.tech/indexd/contracts"
)

// contractLifecycleResponse includes the current host health independently of
// the contract's historic good/bad state. UI clients must display both.
type contractLifecycleResponse struct {
	contracts.Contract
	HostLifecycle string `json:"hostLifecycle"`
}

func (a *admin) decorateContractLifecycle(ctx context.Context, list []contracts.Contract) ([]contractLifecycleResponse, error) {
	result := make([]contractLifecycleResponse, 0, len(list))
	cache := make(map[types.PublicKey]string)
	for _, contract := range list {
		lifecycle, ok := cache[contract.HostKey]
		if !ok {
			host, err := a.hosts.Host(ctx, contract.HostKey)
			if err != nil {
				return nil, fmt.Errorf("failed to check lifecycle for contract host %v: %w", contract.HostKey, err)
			}
			lifecycle = host.LifecycleStatus()
			cache[contract.HostKey] = lifecycle
		}
		result = append(result, contractLifecycleResponse{Contract: contract, HostLifecycle: lifecycle})
	}
	return result, nil
}
