# Experimental contract preservation and host lifecycle

**Do not deploy directly to production without a full PostgreSQL backup and a staging restore.**
This changes contract-formation and slab-migration decisions. It has not been
validated against a live indexd instance.

## Rules

- A failed price/uptime/availability usability check makes the host temporarily
  ineligible for **new uploads, account funding and new contract formation**.
  It does not automatically write a permanent blocklist row or mark an existing
  contract bad.
- Explicit **manual blocks** are written with an additional `ManualBan`
  reason. Their contracts become bad and are migrated as before.
- Legacy automatic reasons (e.g. `IngressPrice`) can be removed after the host
  becomes fully usable again, **including while it still has active contracts**.
  Restoration only reactivates revisable contracts outside the short renewal
  deadline. Any unrelated bad reason must not be silently overwritten.
- Contract formation retains at most **20% of the wanted-contract target** as
  temporarily quarantined paid contracts. This is a risk/cost tradeoff, not a
  universal Sia redundancy setting. Configure enough healthy contracts for
  your application's erasure-code geometry.
- Migration retains sectors while hosts are temporarily quarantined; a host
  with at least six consecutive failed scans and no successful scan for more
  than 72 hours is again eligible for migration. Existing expired/bad
  contracts remain subject to existing migration rules.
- The `lifecycle` field reflects current host usability and block reasons.
  `activeContract` is exposed separately; no-contract hosts are gray in the UI.
- Dashboard statistics separate healthy, quarantine, permanent-ban and
  other-bad contracts, as well as their contracted capacity.
- Currency pins now compare the current smoothed rate against the last
  **applied** exchange rate, not the immediately preceding sample.

## Known limitations / follow-up work

- This first change does **not** implement a cost-estimating scheduler for
  migration vs. renewal, a durable 2/3-scan hysteresis ledger, automatic
  evidence-driven permanent bans or a historically reliable `avoided
  migrations` counter. Those need explicit event storage and test cases.
- Existing manual bans that used **only** a builtin usability reason are not
  distinguishable from legacy automatic blocks. Review such entries before
  running the automatic scan-based cleanup.
- Existing bad contracts without a recorded reason cannot safely be restored
  automatically; they require manual forensic review.
- Host lifecycle fields for contracts currently require per-request host lookups
  (cached per distinct host within the API request).
- The companion frontend work lives in `MarviBiene/web`, and requires a built
  web asset module before the running indexd daemon can display the new UI.
- `quarantine` means a temporary usage restriction, **not** that a paid
  contract's data is physically lost. Pinning/upload destinations still use
  strict host usability checks.
