# Experimental contract preservation and host lifecycle

**Do not deploy directly to production without a full PostgreSQL backup and a staging restore.**
This changes contract-formation and slab-migration decisions. It has not been
validated against a live indexd instance.

## Rules

- A price gouging check (ingress, egress, storage, contract, free-sector) that
  fails pauses **new uploads, funding, refreshes and contract renewals**.
  The existing paid contract remains good, with its previously stored data
  available for reads. A price pause never automatically adds a permanent block
  or causes an immediate migration.
- Price pauses are persisted per host with a start timestamp. When the prices
  fall back under limits, **two consecutive successful full host scans** are
  required to resume new paid work. Another high quote resets the recovery
  counter. Direct price-RPC updates and tightened USD pins can immediately
  enter quarantine but cannot bypass scan-based recovery.
- Explicit **manual blocks** are written with an additional `ManualBan`
  reason. Their contracts become bad and are migrated as before.
- On the first upgrade only, existing blocks with exclusively known legacy
  usability reasons (such as `IngressPrice`) are removed, even if their price
  is *still* too high. This leaves them paused, not permanently blocked.
  Corresponding unexpired/unrenewed, price-blocked paid contracts are revived
  only when their recorded bad reason consists exclusively of those historical
  usability reasons. Expired, renewed, manually blocked or independently bad
  contracts are not silently recovered.
- Future manual bans contain `ManualBan` and are never automatically released.
- **All paid price-only paused contracts count toward the wanted-contract
  target** until the safety migration deadline (half the renew window); temporary non-price outages keep the
  conservative 20% buffer. This minimizes double-payment, but may reduce
  available hosts for *new* uploads until prices return. Monitor the healthy
  (writable) host count and configure enough redundancy for the encoding scheme.
- Migration retains sectors for price-paused hosts until the contract becomes
  unrenewable near its proof height. The existing maintenance deadline marks
  those contracts bad **before** expiration, so their sectors can be moved in
  time to preserve redundancy. This is not a promise that migration can be
  deferred *past* expiration without risking data loss.
- A host with at least six failed scans and no successful scan for more than
  72 hours can be migrated earlier; manual permanent bans still cause immediate
  replacement. Existing independently bad or expired contracts retain their
  existing safety handling.
- The `lifecycle` field reflects current host usability and block reasons.
  `activeContract` is exposed separately; no-contract hosts are gray in the UI.
- Dashboard statistics separate healthy, quarantine, permanent-ban and
  other-bad contracts, as well as their contracted capacity.
- Currency pins now compare the current smoothed rate against the last
  **applied** exchange rate, not the immediately preceding sample.

## Known limitations / follow-up work

- This change does **not** implement a migration-vs-renewal price cost
  optimizer, automatic evidence-driven permanent bans for hostile behavior,
  or a historically reliable `avoided migrations` counter. Those require
  separate evidence storage and integration tests.
- Legacy manual bans entered using *only* a built-in usability reason cannot be
  distinguished from old automatic blocks. Such entries are released once on
  upgrade. Back up the database and review historical block reasons first.
- Existing bad contracts without a recorded reason cannot safely be restored
  automatically; they require manual forensic review.
- Host lifecycle fields for contracts currently require per-request host lookups
  (cached per distinct host within the API request).
- The companion frontend work lives in `MarviBiene/web`, and requires a built
  web asset module before the running indexd daemon can display the new UI.
- `quarantine` means a temporary usage restriction, **not** that a paid
  contract's data is physically lost. Pinning/upload destinations still use
  strict host usability checks.
