# NEVEL Blockchain

A proof-of-work blockchain powering the NEVEL 
financial platform at 0nevel.com

## Mine NEVEL

### Requirements
- Go 1.22+
- Linux or Mac

### Step 1 — Clone and build
git clone https://github.com/oneworld1527/nevel-core
cd nevel-core
go build ./cmd/neveld/

### Step 2 — Initialize
./neveld init --network mainnet --datadir ~/.nevel

### Step 3 — Start your node
./neveld start --network mainnet \
  --datadir ~/.nevel \
  --rpc 127.0.0.1:8332 \
  --p2p 0.0.0.0:8333

### Step 4 — Mine NEVEL
./neveld mine --network mainnet \
  --datadir ~/.nevel \
  --blocks 100 \
  --address YOUR_NEVEL_ADDRESS

## Network
- Block reward: 500 NEVEL
- Block time: 60 seconds  
- Total supply: 21,000,000,000
- Algorithm: SHA-256 PoW
- Explorer: 0nevel.com/explorer
- Platform: 0nevel.com
- Whitepaper: 0nevel.com/whitepaper
