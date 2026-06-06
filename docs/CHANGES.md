# NEVEL Blockchain — Production Hardening Changelog

## Fix 1 & 2 — Fork/Reorg Logic + Total-Work Fork Choice (`chain/chain.go`)

`ValidateAndApplyBlock` now:
- Detects competing blocks at the same height (side-chain blocks whose parent
  is NOT the current tip).
- Computes cumulative `total_work` for the incoming chain.
- Only performs a chain reorganization when `incomingWork > currentWork`
  (longest-chain / most-work rule, as in Bitcoin).
- The new `reorg()` function walks back to the fork point, disconnects the
  old main chain via `RollbackToHeight`, then applies the heavier side chain
  in order from oldest to newest.

## Fix 3 — Fully Atomic Storage (`storage/storage.go`)

`ApplyBlock` previously wrote the undo log and block data in one batch, then
separately mutated UTXOs. A crash between those two writes would leave the
database in a corrupt intermediate state.

All writes — undo log, block bytes, height index, transaction index, UTXO
spends/creates, and the tip pointer — now happen inside **a single Pebble
batch**. Nothing is visible on disk until `batch.Commit(pebble.Sync)` succeeds.

`DisconnectTip` received the same treatment: UTXO deletions, undo-set
restores, and the tip update are all in one atomic batch.

## Fix 4 — Mempool Cleanup After Mining (`mempool/mempool.go`, `chain/chain.go`)

Added `Mempool.RemoveMinedBlock(b blockchain.Block)` which iterates every
non-coinbase transaction in the block and removes it from the mempool map.
`ValidateAndApplyBlock` calls this after both normal block acceptance and
after every block applied during a reorg.

Added `Mempool.Remove(hash [32]byte)` for single-transaction removal.

## Fix 5 — Real P2P Node Wired into `neveld start` (`cmd/neveld/main.go`, `p2p/handler.go`)

`neveld start` previously only launched the RPC HTTP server. It now:
- Accepts a `--p2p <addr>` flag (e.g. `--p2p 0.0.0.0:8333`).
- Creates a `p2p.Node` with a `ChainAdapter` handler that processes:
  - `version` / `verack` handshakes
  - `block` messages (validate + apply + relay)
  - `tx` messages (mempool admission + relay)
  - `getheaders` / `headers` (block locator header sync, up to 2 000 headers)
  - `getaddr` / `addr` (peer address exchange)
  - `ping` / `pong`
- Listens for inbound connections in a background goroutine.
- Runs graceful shutdown via `context.WithCancel` + OS signal handling
  (SIGTERM / SIGINT).

## Fix 6 — Seed Nodes + Peer Discovery (`p2p/seeds.go`, `cmd/neveld/main.go`)

Added `p2p/seeds.go` with:
- `MainnetSeeds` and `TestnetSeeds` — built-in bootstrap peer lists.
- `Bootstrap(ctx)` — dials all seed nodes on startup and requests their peer
  lists via `getaddr`.
- `PeriodicDiscovery(ctx, interval)` — sends `getaddr` to all connected peers
  every 10 minutes to maintain connectivity.
- `HandleAddrMessage(payload)` — connects to newly announced peers.
- `AddrPayload()` — serializes the current peer list to send in `addr` replies.

`neveld start` now accepts `--seeds seed1:port,seed2:port` to override the
built-in seed list.

## Fix 7 — Production Wallet KDF (`wallet/store.go`, `wallet/pbkdf2_internal.go`)

- Wallet files are now **version 2**, using **PBKDF2-SHA256** with a random
  32-byte salt and **100 000 iterations** instead of a single raw SHA-256
  hash of the passphrase.
- Old version 1 wallet files are still loadable (backward compatible).
- Wrong passphrase now returns `"wrong passphrase or corrupt wallet"` without
  leaking timing information.
- `NewFromSeedPhrase` now uses PBKDF2 with a fixed `"nevel-seed"` salt for
  deterministic key derivation (same seed phrase → same key, always).
- The PBKDF2 implementation (`pbkdf2_internal.go`) is self-contained (no
  external dependencies) using standard-library `crypto/hmac` + `crypto/sha256`.

## Note on Fix 8 — Public Testnet / Independent Audit

The seed node DNS names in `p2p/seeds.go` are placeholders. Before mainnet
launch you must:
1. Stand up at least 3 geographically diverse seed nodes using `neveld start
   --network testnet --p2p 0.0.0.0:18333`.
2. Replace the placeholder FQDNs in `MainnetSeeds` / `TestnetSeeds`.
3. Commission an independent security audit of the consensus, storage, and
   cryptographic subsystems.
