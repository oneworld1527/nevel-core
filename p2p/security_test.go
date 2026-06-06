package p2p

import (
	"net"
	"testing"
	"time"
)

func TestPeerScoringBansBadPeer(t *testing.T) {
	n := NewNode(Config{MaxPeers: 2}, nil)
	n.ReportPeer("peer", -100)
	if !n.IsBanned("peer") {
		t.Fatal("peer was not banned")
	}
	if n.Score("peer") != -100 {
		t.Fatal("score not tracked")
	}
}

func TestUnknownMessageTypeRejected(t *testing.T) {
	if err := validateMessage(Message{Type: MessageType("evil")}); err != ErrUnknownMessageType {
		t.Fatalf("expected unknown type rejection, got %v", err)
	}
}

func TestPeerRateLimit(t *testing.T) {
	n := NewNode(Config{PeerRateLimit: 1}, nil)
	if !n.allowPeerMessage("peer", time.Now()) {
		t.Fatal("first message should be allowed")
	}
	if n.allowPeerMessage("peer", time.Now()) {
		t.Fatal("second message in same window should be rate limited")
	}
}

func TestMaxPeersPerIP(t *testing.T) {
	n := NewNode(Config{MaxPeers: 4, MaxPeersPerIP: 1}, nil)
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	if !n.addPeer("127.0.0.1:1000", left) {
		t.Fatal("first peer should be accepted")
	}
	if n.addPeer("127.0.0.1:1001", right) {
		t.Fatal("second peer from same IP should be rejected")
	}
}
