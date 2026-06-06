# NEVEL Security Assurance Gates

Mainnet release is blocked until all security evidence is present and passes `security.ValidateReleaseEvidence`.

## Required evidence

1. **Independent security audit**: auditor name, public report URL, report SHA-256, scope, completion date, and zero open critical/high findings.
2. **Public testnet battle testing**: public testnet start/end timestamps covering the configured minimum duration plus spam, malformed P2P, reorg, and wallet-restore drills.
3. **Real P2P hardening**: bounded message size, message type allow-listing, peer scoring, ban durations, per-peer rate limiting, and per-IP peer caps.
4. **Difficulty/reorg protection**: deterministic retarget validation, accumulated-work fork choice, max-depth reorg policy, stale-tip alerts, and low-peer alerts.
5. **Wallet seed phrase standard audit**: wallet seed phrases are inspected with `wallet.AuditSeedPhrase`; NEVEL v1 seeds are marked as internal-compatible and explicitly not BIP-39 checksummed.
6. **Mining pool/hashrate security**: operators must document stratum/pool authentication, miner payout custody, hashrate-drop alerts, and pool concentration limits before evidence can be marked complete.
7. **DDoS protection**: public RPC/explorer/mining endpoints must be behind authenticated writes, edge rate limits, upstream DDoS filtering, and origin lockdown before evidence can be marked complete.
8. **Exchange-grade monitoring**: exchanges must receive `/metrics/security` health data with stale-tip and low-peer alerts plus independent block-height cross-checks.
9. **Formal supply/economic audit**: release evidence must include a signed review of max supply, halvings, block subsidy, fee math, treasury/foundation/team allocations, and unlock schedules.

## Mainnet rule

A release package that does not satisfy every field in `security.ReleaseEvidence` is not mainnet-ready, even if binaries build and tests pass.
