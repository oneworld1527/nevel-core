package p2p

import (
	"errors"
	"net"
	"time"
)

var (
	ErrUnknownMessageType = errors.New("unknown message type")
	ErrPeerRateLimited    = errors.New("peer rate limited")
	ErrPeerIPLimit        = errors.New("peer ip limit reached")
)

const (
	DefaultPeerRateLimit  = 120
	DefaultPeerRateWindow = time.Minute
	DefaultBanScore       = -100
	DefaultBanDuration    = time.Hour
)

var knownMessageTypes = map[MessageType]struct{}{
	MsgVersion: {}, MsgVerAck: {}, MsgPing: {}, MsgPong: {}, MsgInv: {}, MsgGetData: {},
	MsgTx: {}, MsgBlock: {}, MsgHeaders: {}, MsgGetHeaders: {}, MsgMempool: {}, MsgReject: {},
}

type peerStats struct {
	WindowStart time.Time
	Messages    int
}

func validateMessage(msg Message) error {
	if _, ok := knownMessageTypes[msg.Type]; !ok {
		return ErrUnknownMessageType
	}
	return nil
}

func peerHost(id string) string {
	host, _, err := net.SplitHostPort(id)
	if err == nil {
		return host
	}
	return id
}
