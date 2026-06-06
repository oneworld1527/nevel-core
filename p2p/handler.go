package p2p

import (
	"encoding/json"
	"log"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/chain"
	txpkg "github.com/nevel/nevel-core/tx"
)

// ChainAdapter connects the P2P layer to the chain manager so that the node
// can process inbound blocks and transactions and relay them to peers.
type ChainAdapter struct {
	Node    *Node
	Manager *chain.Manager
}

// Handler returns a p2p.Handler that processes all inbound P2P messages.
func (ca *ChainAdapter) Handler() Handler {
	return func(peer PeerInfo, msg Message) error {
		switch msg.Type {
		case MsgVersion:
			// Acknowledge the peer's version handshake.
			return ca.Node.Send(peer.ID, Message{Type: MsgVerAck})

		case MsgGetAddr:
			// Respond with our known peer list.
			payload, err := ca.Node.AddrPayload()
			if err != nil {
				return err
			}
			return ca.Node.Send(peer.ID, Message{Type: MsgAddr, Payload: payload})

		case MsgAddr:
			// Attempt to connect to newly announced peers.
			ca.Node.HandleAddrMessage(msg.Payload)

		case MsgBlock:
			// Decode, validate and apply the block; relay to other peers if new.
			var b blockchain.Block
			if err := json.Unmarshal(msg.Payload, &b); err != nil {
				ca.Node.ReportPeer(peer.ID, -10)
				return err
			}
			if err := ca.Manager.ValidateAndApplyBlock(b); err != nil {
				// Not necessarily a ban-worthy event (e.g. orphan), just log.
				log.Printf("p2p: block from %s rejected: %v", peer.ID, err)
				return nil
			}
			// Relay to all peers except the sender.
			ca.Node.BroadcastExcept(msg, peer.ID)

		case MsgTx:
			// Decode and accept the transaction into the mempool; relay if new.
			var t txpkg.Transaction
			if err := json.Unmarshal(msg.Payload, &t); err != nil {
				ca.Node.ReportPeer(peer.ID, -10)
				return err
			}
			if err := ca.Manager.Mempool.Add(t); err != nil {
				return nil // already known or invalid — do not relay
			}
			ca.Node.BroadcastExcept(msg, peer.ID)

		case MsgGetHeaders:
			// Decode the block locator and send back a headers batch.
			var req GetHeadersRequest
			if err := json.Unmarshal(msg.Payload, &req); err != nil {
				return err
			}
			headers, err := ca.gatherHeaders(req.Locator, req.Stop)
			if err != nil {
				return err
			}
			payload, err := json.Marshal(headers)
			if err != nil {
				return err
			}
			return ca.Node.Send(peer.ID, Message{Type: MsgHeaders, Payload: payload})

		case MsgPing:
			return ca.Node.Send(peer.ID, Message{Type: MsgPong, Payload: msg.Payload})
		}
		return nil
	}
}

// GetHeadersRequest is the payload for MsgGetHeaders.
type GetHeadersRequest struct {
	Locator [][32]byte `json:"locator"`
	Stop    [32]byte   `json:"stop"`
}

// gatherHeaders finds the fork point using the block locator and returns up to
// 2000 sequential headers from that point.
func (ca *ChainAdapter) gatherHeaders(locator [][32]byte, stop [32]byte) ([]blockchain.BlockHeader, error) {
	tip, err := ca.Manager.TipBlock()
	if err != nil {
		return nil, err
	}
	startHeight := uint64(0)
	for _, hash := range locator {
		if _, err := ca.Manager.Store.GetBlock(hash); err == nil {
			b, err := ca.Manager.Store.GetBlock(hash)
			if err == nil {
				startHeight = b.Header.Height + 1
				break
			}
		}
	}
	const maxHeaders = 2000
	headers := make([]blockchain.BlockHeader, 0, maxHeaders)
	for h := startHeight; h <= tip.Header.Height && len(headers) < maxHeaders; h++ {
		bh, err := ca.Manager.HeaderByHeight(h)
		if err != nil {
			break
		}
		headers = append(headers, bh)
		if bh.Hash() == stop {
			break
		}
	}
	return headers, nil
}

// BroadcastExcept sends a message to all connected peers except the one
// identified by excludeID.
func (n *Node) BroadcastExcept(msg Message, excludeID string) {
	n.mu.RLock()
	peers := make([]string, 0, len(n.peers))
	for id := range n.peers {
		if id != excludeID {
			peers = append(peers, id)
		}
	}
	n.mu.RUnlock()
	for _, peer := range peers {
		_ = n.Send(peer, msg)
	}
}
