package p2p

import "time"

type MessageType string

const (
	MsgVersion    MessageType = "version"
	MsgVerAck                 = "verack"
	MsgPing                   = "ping"
	MsgPong                   = "pong"
	MsgInv                    = "inv"
	MsgGetData                = "getdata"
	MsgTx                     = "tx"
	MsgBlock                  = "block"
	MsgHeaders                = "headers"
	MsgGetHeaders             = "getheaders"
	MsgGetAddr                = "getaddr"
	MsgAddr                   = "addr"
	MsgMempool                = "mempool"
	MsgReject                 = "reject"
	MsgPeers                  = "peers"
)

type Message struct {
	Type    MessageType `json:"type"`
	Payload []byte      `json:"payload"`
}
type PeerInfo struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	LastSeen time.Time `json:"lastSeen"`
}
type Config struct {
	NetworkID       string
	ListenAddrs     []string
	Seeds           []string
	MaxPeers        int
	MaxMessageBytes int
	MaxPeersPerIP   int
	PeerRateLimit   int
	PeerRateWindow  time.Duration
	BanScore        int
	BanDuration     time.Duration
}
