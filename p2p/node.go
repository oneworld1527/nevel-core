package p2p

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const MaxMessageBytes = 4 * 1024 * 1024

type Handler func(PeerInfo, Message) error

type Node struct {
	cfg     Config
	handler Handler
	mu      sync.RWMutex
	peers   map[string]net.Conn
	scores  map[string]int
	banned  map[string]time.Time
	stats   map[string]peerStats
}

func NewNode(cfg Config, handler Handler) *Node {
	if cfg.MaxPeers <= 0 {
		cfg.MaxPeers = 32
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = MaxMessageBytes
	}
	if cfg.PeerRateLimit <= 0 {
		cfg.PeerRateLimit = DefaultPeerRateLimit
	}
	if cfg.PeerRateWindow <= 0 {
		cfg.PeerRateWindow = DefaultPeerRateWindow
	}
	if cfg.BanScore == 0 {
		cfg.BanScore = DefaultBanScore
	}
	if cfg.BanDuration <= 0 {
		cfg.BanDuration = DefaultBanDuration
	}
	return &Node{cfg: cfg, handler: handler, peers: map[string]net.Conn{}, scores: map[string]int{}, banned: map[string]time.Time{}, stats: map[string]peerStats{}}
}

func (n *Node) Listen(ctx context.Context, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !n.addPeer(conn.RemoteAddr().String(), conn) {
			_ = conn.Close()
			continue
		}
		go n.readLoop(conn)
	}
}

func (n *Node) Connect(address string) error {
	if n.IsBanned(address) {
		return errors.New("peer is banned")
	}
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return err
	}
	if !n.addPeer(address, conn) {
		_ = conn.Close()
		return errors.New("peer limit reached")
	}
	go n.readLoop(conn)
	return n.Send(address, Message{Type: MsgVersion, Payload: []byte(n.cfg.NetworkID)})
}

func (n *Node) Send(peer string, msg Message) error {
	n.mu.RLock()
	conn := n.peers[peer]
	n.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("peer not connected: %s", peer)
	}
	return writeMessage(conn, msg)
}

func (n *Node) Broadcast(msg Message) {
	n.mu.RLock()
	peers := make([]string, 0, len(n.peers))
	for peer := range n.peers {
		peers = append(peers, peer)
	}
	n.mu.RUnlock()
	for _, peer := range peers {
		_ = n.Send(peer, msg)
	}
}

func (n *Node) PeerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.peers)
}

func (n *Node) addPeer(id string, conn net.Conn) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if until, banned := n.banned[id]; banned && time.Now().Before(until) {
		return false
	}
	if len(n.peers) >= n.cfg.MaxPeers {
		return false
	}
	if n.cfg.MaxPeersPerIP > 0 {
		host := peerHost(id)
		count := 0
		for peer := range n.peers {
			if peerHost(peer) == host {
				count++
			}
		}
		if count >= n.cfg.MaxPeersPerIP {
			return false
		}
	}
	n.peers[id] = conn
	return true
}

func (n *Node) removePeer(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if conn := n.peers[id]; conn != nil {
		_ = conn.Close()
	}
	delete(n.peers, id)
}

func (n *Node) ReportPeer(peer string, delta int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.scores[peer] += delta
	if n.scores[peer] <= n.cfg.BanScore {
		n.banned[peer] = time.Now().Add(n.cfg.BanDuration)
		if conn := n.peers[peer]; conn != nil {
			_ = conn.Close()
		}
		delete(n.peers, peer)
	}
}

func (n *Node) Score(peer string) int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.scores[peer]
}

func (n *Node) IsBanned(peer string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	until, ok := n.banned[peer]
	return ok && time.Now().Before(until)
}

func (n *Node) readLoop(conn net.Conn) {
	id := conn.RemoteAddr().String()
	defer n.removePeer(id)
	reader := bufio.NewReader(conn)
	for {
		msg, err := n.readPeerMessage(id, reader)
		if err != nil {
			n.ReportPeer(id, -25)
			return
		}
		if n.handler != nil {
			if err := n.handler(PeerInfo{ID: id, Address: id, LastSeen: time.Now()}, msg); err != nil {
				n.ReportPeer(id, -10)
			}
		}
	}
}

func (n *Node) readPeerMessage(id string, reader *bufio.Reader) (Message, error) {
	if !n.allowPeerMessage(id, time.Now()) {
		return Message{}, ErrPeerRateLimited
	}
	msg, err := readMessageLimit(reader, n.cfg.MaxMessageBytes)
	if err != nil {
		return Message{}, err
	}
	if err := validateMessage(msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (n *Node) allowPeerMessage(id string, now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := n.stats[id]
	if st.WindowStart.IsZero() || now.Sub(st.WindowStart) >= n.cfg.PeerRateWindow {
		st = peerStats{WindowStart: now}
	}
	st.Messages++
	n.stats[id] = st
	return st.Messages <= n.cfg.PeerRateLimit
}

func writeMessage(conn net.Conn, msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(payload) > MaxMessageBytes {
		return errors.New("message too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := conn.Write(header[:]); err != nil {
		return err
	}
	_, err = conn.Write(payload)
	return err
}

func readMessage(reader *bufio.Reader) (Message, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Message{}, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > MaxMessageBytes {
		return Message{}, errors.New("invalid message size")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return Message{}, err
	}
	var msg Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return Message{}, err
	}
	if err := validateMessage(msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func readMessageLimit(reader *bufio.Reader, maxBytes int) (Message, error) {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Message{}, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if maxBytes <= 0 {
		maxBytes = MaxMessageBytes
	}
	if size == 0 || size > uint32(maxBytes) {
		return Message{}, errors.New("invalid message size")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return Message{}, err
	}
	var msg Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return Message{}, err
	}
	if err := validateMessage(msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}
