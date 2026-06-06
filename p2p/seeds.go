package p2p

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// MainnetSeeds are the well-known bootstrap peers for the NEVEL mainnet.
// Replace these with real seed node DNS names or IPs before public launch.
var MainnetSeeds = []string{
	"seed1.nevel.network:8333",
	"seed2.nevel.network:8333",
	"seed3.nevel.network:8333",
}

// TestnetSeeds are the bootstrap peers for the NEVEL testnet.
var TestnetSeeds = []string{
	"seed1.testnet.nevel.network:18333",
	"seed2.testnet.nevel.network:18333",
}

// Bootstrap dials all seed nodes, requests their peer lists, and connects to
// the resulting peers up to the node's MaxPeers limit. It runs in the
// background; errors are logged but do not stop the node.
func (n *Node) Bootstrap(ctx context.Context) {
	seeds := n.cfg.Seeds
	if len(seeds) == 0 {
		return
	}
	log.Printf("p2p: bootstrapping from %d seed(s)", len(seeds))
	for _, seed := range seeds {
		if ctx.Err() != nil {
			return
		}
		if err := n.Connect(seed); err != nil {
			log.Printf("p2p: seed %s unreachable: %v", seed, err)
			continue
		}
		// Request the seed's peer list.
		if err := n.Send(seed, Message{Type: MsgGetAddr}); err != nil {
			log.Printf("p2p: getaddr to %s failed: %v", seed, err)
		}
	}
}

// PeriodicDiscovery requests fresh peer lists from all connected peers on a
// regular schedule, helping the node stay well-connected over time.
func (n *Node) PeriodicDiscovery(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.mu.RLock()
			peers := make([]string, 0, len(n.peers))
			for id := range n.peers {
				peers = append(peers, id)
			}
			n.mu.RUnlock()
			for _, peer := range peers {
				_ = n.Send(peer, Message{Type: MsgGetAddr})
			}
		}
	}
}

// HandleAddrMessage processes an inbound MsgAddr payload by attempting to
// connect to any newly announced peers.
func (n *Node) HandleAddrMessage(payload []byte) {
	var addrs []string
	if err := json.Unmarshal(payload, &addrs); err != nil {
		return
	}
	for _, addr := range addrs {
		if n.PeerCount() >= n.cfg.MaxPeers {
			return
		}
		n.mu.RLock()
		_, known := n.peers[addr]
		n.mu.RUnlock()
		if known {
			continue
		}
		if err := n.Connect(addr); err != nil {
			log.Printf("p2p: addr connect %s failed: %v", addr, err)
		}
	}
}

// AddrPayload builds the JSON payload for a MsgAddr message containing the
// addresses of all currently connected peers.
func (n *Node) AddrPayload() ([]byte, error) {
	n.mu.RLock()
	addrs := make([]string, 0, len(n.peers))
	for id := range n.peers {
		addrs = append(addrs, id)
	}
	n.mu.RUnlock()
	return json.Marshal(addrs)
}
