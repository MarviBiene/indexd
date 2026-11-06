package postgres

import (
	"context"
	"testing"
	"time"

	"go.sia.tech/core/types"
	"go.sia.tech/coreutils/rhp/v4"
	"go.sia.tech/indexd/contracts"
	"go.sia.tech/indexd/geoip"
	"go.sia.tech/indexd/hosts"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestPriceQuarantineDoesNotInvalidatePaidContract(t *testing.T) {
	db := initPostgres(t, zaptest.NewLogger(t))
	if err := db.UpdateMaintenanceSettings(contracts.MaintenanceSettings{
		Period: 2, RenewWindow: 1, WantedContracts: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateUsabilitySettings(hosts.UsabilitySettings{
		MaxEgressPrice: types.Siacoins(10), MaxIngressPrice: types.Siacoins(10),
		MaxStoragePrice: types.Siacoins(10), MinCollateral: types.ZeroCurrency,
		MinProtocolVersion: rhp.ProtocolVersion400,
	}); err != nil {
		t.Fatal(err)
	}
	hk := types.GeneratePrivateKey().PublicKey()
	db.addTestHost(t, hk)
	fcid := db.addTestContract(t, hk)
	settings := newTestHostSettings(hk)
	settings.Prices.ValidUntil = time.Now().Add(24 * time.Hour)
	settings.Prices.IngressPrice = types.Siacoins(5)
	settings.Prices.EgressPrice = types.Siacoins(5)
	if err := db.UpdateHostScan(hk, settings, geoip.Location{}, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if h, err := db.Host(hk); err != nil {
		t.Fatal(err)
	} else if !h.PriceQuarantineSince.IsZero() {
		t.Fatal("under-limit host must not be quarantined")
	}

	// A single overpriced scan pauses writes, without changing the paid
	// contract's goodness or producing a permanent blocklist entry.
	settings.Prices.IngressPrice = types.Siacoins(15)
	if err := db.UpdateHostScan(hk, settings, geoip.Location{}, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	h, err := db.Host(hk)
	if err != nil {
		t.Fatal(err)
	} else if h.PriceQuarantineSince.IsZero() || h.Usability.IngressPrice || h.IsGood() || !h.PricePaused() {
		t.Fatalf("expected ingress price quarantine, got %+v", h)
	} else if h.Blocked {
		t.Fatal("price glitch must not permanently block host")
	}
	c, err := db.Contract(fcid)
	if err != nil {
		t.Fatal(err)
	} else if !c.Good {
		t.Fatal("price glitch must not invalidate a paid contract")
	}

	// First good scan is deliberately insufficient to resume writes.
	settings.Prices.IngressPrice = types.Siacoins(5)
	if err := db.UpdateHostScan(hk, settings, geoip.Location{}, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	h, err = db.Host(hk)
	if err != nil {
		t.Fatal(err)
	} else if h.PriceQuarantineSince.IsZero() || h.PriceRecoveryScans != 1 || h.Usability.PriceRecovery {
		t.Fatalf("expected one pending price recovery scan, got %+v", h)
	}

	// Second consecutive good scan ends the price quarantine.
	if err := db.UpdateHostScan(hk, settings, geoip.Location{}, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	h, err = db.Host(hk)
	if err != nil {
		t.Fatal(err)
	} else if !h.PriceQuarantineSince.IsZero() || h.PriceRecoveryScans != 0 || !h.Usability.PriceRecovery {
		t.Fatalf("expected automatic price recovery, got %+v", h)
	}
}

func TestRestoreLegacyUsabilityBlocksLeavesManualBans(t *testing.T) {
	db := initPostgres(t, zaptest.NewLogger(t))
	autoHost := db.addTestHost(t, types.PublicKey{1})
	manualHost := db.addTestHost(t, types.PublicKey{2})
	autoContract := db.addTestContract(t, autoHost)
	manualContract := db.addTestContract(t, manualHost)

	if err := db.BlockHosts([]types.PublicKey{autoHost}, []string{"IngressPrice"}); err != nil {
		t.Fatal(err)
	}
	if err := db.BlockHosts([]types.PublicKey{manualHost}, []string{"UserPermanentBan"}); err != nil {
		t.Fatal(err)
	}

	if err := db.transaction(func(ctx context.Context, tx *txn) error {
		return restoreLegacyUsabilityBlocks(ctx, tx, zap.NewNop())
	}); err != nil {
		t.Fatal(err)
	}
	h, err := db.Host(autoHost)
	if err != nil {
		t.Fatal(err)
	} else if h.Blocked {
		t.Fatal("legacy automatic price block was not released")
	}
	c, err := db.Contract(autoContract)
	if err != nil {
		t.Fatal(err)
	} else if !c.Good {
		t.Fatal("old price-blocked active contract was not recovered")
	}

	h, err = db.Host(manualHost)
	if err != nil {
		t.Fatal(err)
	} else if !h.Blocked || !h.IsPermanentBan() {
		t.Fatal("manual ban must survive legacy cleanup")
	}
	c, err = db.Contract(manualContract)
	if err != nil {
		t.Fatal(err)
	} else if c.Good {
		t.Fatal("manual-banned contract must remain bad")
	}
}
