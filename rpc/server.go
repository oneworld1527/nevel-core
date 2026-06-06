package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/chain"
	"github.com/nevel/nevel-core/mempool"
	"github.com/nevel/nevel-core/pay"
	"github.com/nevel/nevel-core/storage"
	txpkg "github.com/nevel/nevel-core/tx"
	"github.com/nevel/nevel-core/wallet"
)

type Server struct {
	Store     *storage.DB
	Mempool   *mempool.Mempool
	Chain     *chain.Manager
	Payments  *pay.Store
	AuthToken string
	Peers     func() int
}

func New(store *storage.DB, mp *mempool.Mempool) *Server {
	return &Server{Store: store, Mempool: mp, Payments: pay.NewStore()}
}
func NewWithChain(manager *chain.Manager) *Server {
	return &Server{Store: manager.Store, Mempool: manager.Mempool, Chain: manager, Payments: pay.NewStore()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/metrics/security", s.securityMetrics)
	mux.HandleFunc("/chain/tip", s.tip)
	mux.HandleFunc("/block/height/", s.blockHeight)
	mux.HandleFunc("/block/", s.blockHash)
	mux.HandleFunc("/tx/", s.txByHash)
	mux.HandleFunc("/address/", s.address)
	mux.HandleFunc("/tx/broadcast", s.broadcastTx)
	mux.HandleFunc("/mempool", s.mempool)
	mux.HandleFunc("/mining/template", s.miningTemplate)
	mux.HandleFunc("/mining/submit", s.submitBlock)
	mux.HandleFunc("/payments/invoice", s.invoice)
	mux.HandleFunc("/payments/status/", s.paymentStatus)
	return securityHeaders(s.auth(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) securityMetrics(w http.ResponseWriter, _ *http.Request) {
	peers := 0
	if s.Peers != nil {
		peers = s.Peers()
	}
	if s.Chain == nil {
		writeJSON(w, map[string]any{"peerCount": peers, "alerts": []string{"chain_manager_unavailable"}})
		return
	}
	tip, err := s.Chain.Tip()
	if err != nil {
		writeJSON(w, map[string]any{"peerCount": peers, "alerts": []string{"tip_unavailable"}})
		return
	}
	writeJSON(w, chain.AssessHealth(tip, peers, chain.HealthThresholds{MaxTipAge: 5 * time.Minute, MinPeerCount: 1}, time.Now()))
}

func (s *Server) tip(w http.ResponseWriter, _ *http.Request) {
	if s.Chain != nil {
		tip, err := s.Chain.Tip()
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, tip)
		return
	}
	h, err := s.Store.Tip()
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	b, err := s.Store.GetBlock(h)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]any{"hash": hex.EncodeToString(h[:]), "height": b.Header.Height, "bits": b.Header.Bits})
}
func (s *Server) blockHeight(w http.ResponseWriter, r *http.Request) {
	height, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/block/height/"), 10, 64)
	if err != nil {
		http.Error(w, "bad height", 400)
		return
	}
	h, err := s.Store.BlockHashByHeight(height)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	s.writeBlock(w, h)
}
func (s *Server) blockHash(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/block/")
	b, err := hex.DecodeString(raw)
	if err != nil || len(b) != 32 {
		http.Error(w, "bad hash", 400)
		return
	}
	var h [32]byte
	copy(h[:], b)
	s.writeBlock(w, h)
}
func (s *Server) writeBlock(w http.ResponseWriter, h [32]byte) {
	b, err := s.Store.GetBlock(h)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, blockDTO(b))
}
func (s *Server) txByHash(w http.ResponseWriter, r *http.Request) {
	h, err := parseHash(strings.TrimPrefix(r.URL.Path, "/tx/"))
	if err != nil {
		http.Error(w, "bad hash", 400)
		return
	}
	t, err := s.Store.GetTx(h)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, txDTO(t))
}
func (s *Server) address(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/address/"), "/")
	if len(parts) != 2 || (parts[1] != "balance" && parts[1] != "utxos") {
		http.Error(w, "expected /address/{address}/balance or /address/{address}/utxos", 404)
		return
	}
	prefix := "rnevel"
	if s.Chain != nil {
		prefix = s.Chain.Network.AddressPrefix
	}
	script, err := wallet.LockingScriptForAddress(parts[0], prefix)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if parts[1] == "balance" {
		balance, err := s.Store.BalanceByScript(script)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"address": parts[0], "balance": balance})
		return
	}
	utxos, err := s.Store.ListUTXOsByScript(script)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, utxos)
}
func (s *Server) broadcastTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Hex string `json:"hex"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	data, err := hex.DecodeString(req.Hex)
	if err != nil {
		http.Error(w, "bad transaction hex", 400)
		return
	}
	t, err := txpkg.DeserializeTransaction(data)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if s.Mempool == nil {
		http.Error(w, "mempool unavailable", 503)
		return
	}
	if err := s.Mempool.Add(t); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	h := t.Hash()
	writeJSON(w, map[string]string{"txHash": hex.EncodeToString(h[:])})
}
func (s *Server) mempool(w http.ResponseWriter, _ *http.Request) {
	if s.Mempool == nil {
		writeJSON(w, []string{})
		return
	}
	txs := s.Mempool.Snapshot()
	hashes := make([]string, len(txs))
	for i, t := range txs {
		h := t.Hash()
		hashes[i] = hex.EncodeToString(h[:])
	}
	writeJSON(w, hashes)
}
func (s *Server) miningTemplate(w http.ResponseWriter, r *http.Request) {
	if s.Chain == nil {
		http.Error(w, "chain manager unavailable", 503)
		return
	}
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "address query parameter required", 400)
		return
	}
	script, err := wallet.LockingScriptForAddress(address, s.Chain.Network.AddressPrefix)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_, tmpl, err := s.Chain.BuildTemplate(script)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, tmpl)
}
func (s *Server) submitBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Chain == nil {
		http.Error(w, "chain manager unavailable", 503)
		return
	}
	var req struct {
		Hex string `json:"hex"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	data, err := hex.DecodeString(req.Hex)
	if err != nil {
		http.Error(w, "bad block hex", 400)
		return
	}
	block, err := blockchain.DeserializeBlock(data)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := s.Chain.ValidateAndApplyBlock(block); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	h := block.Hash()
	writeJSON(w, map[string]any{"hash": hex.EncodeToString(h[:]), "height": block.Header.Height})
}
func (s *Server) invoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Merchant string `json:"merchant"`
		Address  string `json:"address"`
		Amount   uint64 `json:"amount"`
		Memo     string `json:"memo"`
		TTL      int64  `json:"ttlSeconds"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	invoice, err := s.Payments.Create(req.Merchant, req.Address, req.Amount, req.Memo, time.Duration(req.TTL)*time.Second)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, invoice)
}
func (s *Server) paymentStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/payments/status/")
	invoice, ok := s.Payments.Get(id)
	if !ok {
		http.Error(w, "payment not found", 404)
		return
	}
	writeJSON(w, invoice)
}

func blockDTO(b blockchain.Block) map[string]any {
	h := b.Hash()
	txs := make([]string, len(b.Transactions))
	for i, t := range b.Transactions {
		th := t.Hash()
		txs[i] = hex.EncodeToString(th[:])
	}
	return map[string]any{"hash": hex.EncodeToString(h[:]), "height": b.Header.Height, "previous": hex.EncodeToString(b.Header.PrevHash[:]), "merkleRoot": hex.EncodeToString(b.Header.MerkleRoot[:]), "timestamp": b.Header.Timestamp, "bits": b.Header.Bits, "nonce": b.Header.Nonce, "transactions": txs}
}
func txDTO(t txpkg.Transaction) map[string]any {
	h := t.Hash()
	return map[string]any{"hash": hex.EncodeToString(h[:]), "version": t.Version, "inputs": t.Inputs, "outputs": t.Outputs, "lockTime": t.LockTime, "coinbase": t.IsCoinbase()}
}
func parseHash(raw string) ([32]byte, error) {
	var h [32]byte
	b, err := hex.DecodeString(raw)
	if err != nil || len(b) != 32 {
		return h, fmt.Errorf("bad hash")
	}
	copy(h[:], b)
	return h, nil
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AuthToken != "" && (r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/mining/") || strings.HasPrefix(r.URL.Path, "/tx/broadcast") || strings.HasPrefix(r.URL.Path, "/payments/")) {
			if r.Header.Get("Authorization") != "Bearer "+s.AuthToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-content-type-options", "nosniff")
		w.Header().Set("referrer-policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
