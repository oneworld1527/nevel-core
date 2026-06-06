# NEVEL Launch Runbook

This repository now contains code hooks for MVP testnet operations, but mainnet must not launch until the checklist below is complete.

## Private testnet infrastructure

- Run at least five nodes: `seed1`, `seed2`, `rpc1`, `explorer`, and `faucet`.
- Start nodes with RPC auth: `neveld start --network testnet --rpctoken <secret>`.
- Keep admin RPC on private networks or localhost-only tunnels.
- Monitor block height, tip hash, peer count, mempool size, deep reorgs, stale tips, and disk usage.

## Public testnet infrastructure

- Publish binaries, genesis hash, seed node addresses, explorer URL, faucet URL, and miner guide.
- Run public testnet for 30-90 days before mainnet.
- Run spam, malformed P2P, invalid block, low-fee, double-spend, reorg, wallet restore, restart, and database-corruption drills.

## Mainnet finalization

- Freeze consensus code and tag release candidates.
- Mine and publish final mainnet genesis block and hash.
- Publish supply schedule and any treasury/foundation/team allocations with lockups.
- Require independent audit sign-off, bug bounty completion, and incident-response rehearsals.

## Independent security audit scope

- Consensus validation, proof-of-work, difficulty adjustment, block rewards, fee math, UTXO spends, reorgs, serialization, P2P parsing, RPC auth, wallet encryption, seed handling, payment webhooks, and operational key management.
