# NEVEL Security Checklist

NEVEL must be treated as money software. This checklist is part of the project baseline and should gate testnet/mainnet releases.

## Protocol and consensus

- Reject blocks with invalid previous hashes, proof-of-work, timestamps, merkle roots, difficulty targets, coinbase rewards, transaction signatures, UTXO spends, or supply math.
- Validate every spend against the local UTXO set; never trust peer-provided balances or transaction status.
- Detect double spends inside blocks and inside the mempool before relay.
- Check all amount additions/subtractions for unsigned integer overflow.
- Retarget difficulty only through deterministic consensus code with bounded adjustment.
- Prefer the valid chain with the most accumulated work once chain-work tracking and reorg support are implemented.

## Node and P2P hardening

- Treat every peer as malicious until all messages are parsed, bounded, and validated.
- Apply maximum sizes to messages, blocks, headers, inventory, and mempool payloads.
- Rate-limit expensive peer behavior and ban peers that repeatedly send invalid blocks or malformed messages.
- Use diverse seed nodes across hosting providers and regions to reduce eclipse and Sybil risk.
- Add reorg, deep-reorg, stale-tip, hashrate-drop, and difficulty-anomaly alerts before public networks.

## Wallet and key safety

- Do not store private keys or seed phrases in plaintext.
- Do not send private keys, seeds, or unsigned signing secrets to any server.
- Always display destination address, amount, and fee before signing.
- Use secp256k1 keys and a `SHA256 -> RIPEMD160 -> Bech32` address pipeline for Bitcoin-style compatibility.
- Add encrypted wallet files, seed phrases, backups, and hardware-wallet support before mainnet value is at risk.

## RPC, explorer, and NEVEL Pay

- Bind admin RPC to localhost by default; put any public RPC behind authentication, rate limits, and DDoS protection.
- Never expose private-key export, node shutdown, database deletion, or treasury-control endpoints publicly.
- Sign merchant webhooks with HMAC, require idempotency keys, and expire invoices.
- Use unique invoice addresses and confirmation thresholds based on payment value.
- Keep logs free of private keys, seed phrases, authorization headers, webhook secrets, and payment credentials.

## Operations and launch

- Require branch protection, mandatory reviews, secret scanning, and signed release artifacts.
- Run unit, fuzz, integration, race, static-analysis, database-corruption, reorg, spam, and double-spend tests before mainnet.
- Run private testnet first, then public testnet for 30-90 days, then independent audit and bug bounty before mainnet.
- Publish genesis parameters, binaries, source, supply schedule, treasury allocation, and any locked/team allocation transparently.
- Maintain an incident response plan with identify, protect, detect, respond, and recover stages.

## Current implementation gates

- Header-first sync code paths must be exercised against hostile peer fixtures before public testnet.
- Reorg handling must be reviewed with block-undo fixtures before mainnet.
- The local Pebble-compatible shim is for this build environment only; release builds should remove the `replace` directive and use audited upstream Pebble.
- Independent audit evidence must be attached to release artifacts before mainnet launch.
- Mainnet release evidence must pass `security.ValidateReleaseEvidence`, including independent audit, public testnet battle testing, P2P hardening, reorg/difficulty guards, wallet seed audit, mining/hashrate controls, DDoS controls, exchange monitoring, and supply/economic audit gates.
