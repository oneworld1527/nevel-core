package pay

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const (
	StatusPending     = "pending"
	StatusBroadcasted = "broadcasted"
	StatusConfirmed   = "confirmed"
	StatusExpired     = "expired"
	StatusRefunded    = "refunded"
)

type PaymentRequest struct {
	ID        string `json:"id"`
	Merchant  string `json:"merchant"`
	Address   string `json:"address"`
	Amount    uint64 `json:"amount"`
	Memo      string `json:"memo"`
	ExpiresAt int64  `json:"expiresAt"`
	Status    string `json:"status"`
	TxHash    string `json:"txHash,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	requests map[string]PaymentRequest
}

func NewStore() *Store { return &Store{requests: map[string]PaymentRequest{}} }

func (s *Store) Create(merchant, address string, amount uint64, memo string, ttl time.Duration) (PaymentRequest, error) {
	if merchant == "" || address == "" || amount == 0 {
		return PaymentRequest{}, errors.New("merchant, address, and positive amount are required")
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return PaymentRequest{}, err
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	req := PaymentRequest{ID: hex.EncodeToString(idBytes), Merchant: merchant, Address: address, Amount: amount, Memo: memo, ExpiresAt: time.Now().Add(ttl).Unix(), Status: StatusPending}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[req.ID] = req
	return req, nil
}

func (s *Store) Get(id string) (PaymentRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[id]
	if ok && req.Status == StatusPending && time.Now().Unix() > req.ExpiresAt {
		req.Status = StatusExpired
	}
	return req, ok
}

func (s *Store) MarkBroadcasted(id, txHash string) (PaymentRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[id]
	if !ok {
		return PaymentRequest{}, errors.New("payment not found")
	}
	if time.Now().Unix() > req.ExpiresAt {
		req.Status = StatusExpired
	} else {
		req.Status = StatusBroadcasted
		req.TxHash = txHash
	}
	s.requests[id] = req
	return req, nil
}

func SignWebhook(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type WebhookEvent struct {
	Type      string         `json:"type"`
	Payment   PaymentRequest `json:"payment"`
	CreatedAt int64          `json:"createdAt"`
}

func VerifyWebhook(secret []byte, body []byte, signature string) bool {
	expected := SignWebhook(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}
func Event(eventType string, payment PaymentRequest) WebhookEvent {
	return WebhookEvent{Type: eventType, Payment: payment, CreatedAt: time.Now().Unix()}
}
