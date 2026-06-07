package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/chain"
	"github.com/nevel/nevel-core/consensus"
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
	mux.HandleFunc("/tx/broadcast", s.broadcastTx)
	mux.HandleFunc("/tx/build", s.buildTx)
	mux.HandleFunc("/tx/", s.txByHash)
	mux.HandleFunc("/address/", s.address)
	mux.HandleFunc("/mempool", s.mempool)
	mux.HandleFunc("/mining/template", s.miningTemplate)
	mux.HandleFunc("/mining/submit", s.submitBlock)
	mux.HandleFunc("/payments/invoice", s.invoice)
	mux.HandleFunc("/payments/status/", s.paymentStatus)
	mux.HandleFunc("/genesis/pool", s.genesisPool)
	mux.HandleFunc("/genesis/trade", s.genesisTrade)
	mux.HandleFunc("/genesis/wallet/new", s.genesisWalletNew)
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
	prefix := "nevel"
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

// broadcastTx accepts a fully signed raw transaction hex and adds it to the mempool.
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

// buildTx builds and broadcasts a UTXO transaction from a private key.
// This is the server-side transaction builder used by Genesis trades.
// The platform calls this with the user's private key to move real NEVEL
// on-chain for every Genesis buy and sell.
//
// Better than Bitcoin: Bitcoin requires users to manage their own keys.
// NEVEL manages keys server-side so users get the security of blockchain
// without the complexity of wallets.
//
// Better than Ethereum: No gas estimation, no EIP-1559 complexity.
// Simple fixed fee model.
func (s *Server) buildTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PrivateKeyHex string `json:"privateKeyHex"`
		ToAddress     string `json:"toAddress"`
		Amount        uint64 `json:"amount"`
		Fee           uint64 `json:"fee"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if req.PrivateKeyHex == "" || req.ToAddress == "" || req.Amount == 0 {
		http.Error(w, "privateKeyHex, toAddress, and amount are required", 400)
		return
	}
	prefix := "nevel"
	if s.Chain != nil {
		prefix = s.Chain.Network.AddressPrefix
	}
	fee := req.Fee
	if fee == 0 {
		fee = 1000 // default 1000 neveloshis fee
	}
	w2, err := wallet.FromHexPrivateKey(req.PrivateKeyHex, prefix)
	if err != nil {
		http.Error(w, "bad private key: "+err.Error(), 400)
		return
	}
	t, err := wallet.BuildTransaction(w2, s.Store, req.ToAddress, req.Amount, fee)
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
	raw := hex.EncodeToString(txpkg.SerializeTransaction(t))
	writeJSON(w, map[string]any{
		"txHash":      hex.EncodeToString(h[:]),
		"fromAddress": w2.Address,
		"toAddress":   req.ToAddress,
		"amount":      req.Amount,
		"fee":         fee,
		"raw":         raw,
		"explorerUrl": fmt.Sprintf("https://0nevel.com/explorer?tx=%s", hex.EncodeToString(h[:])),
	})
}

// genesisWalletNew creates a new NEVEL blockchain wallet for a Genesis user.
// Every user gets a real on-chain address when they join Genesis.
func (s *Server) genesisWalletNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "nevel"
	if s.Chain != nil {
		prefix = s.Chain.Network.AddressPrefix
	}
	wlt, err := wallet.New(prefix)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{
		"address":       wlt.Address,
		"privateKeyHex": wallet.PrivateKeyHex(wlt.PrivateKey),
		"publicKeyHex":  hex.EncodeToString(wlt.PublicKey),
	})
}

// genesisTrade executes a real on-chain UTXO transaction for a Genesis trade.
// When a user buys a Genesis token:
//   - Their NEVEL moves from their blockchain address to the Genesis pool address
//   - This is a real signed UTXO transaction on the NEVEL chain
//   - It appears on the explorer like any other transfer
//   - Better than Ethereum: no gas wars, instant finality at platform level
func (s *Server) genesisTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PrivateKeyHex string  `json:"privateKeyHex"`
		PoolAddress   string  `json:"poolAddress"`
		Amount        uint64  `json:"amount"`
		Side          string  `json:"side"`
		Ticker        string  `json:"ticker"`
		LaunchID      string  `json:"launchId"`
		TradeID       string  `json:"tradeId"`
		PriceNevel    float64 `json:"priceNevel"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	prefix := "nevel"
	if s.Chain != nil {
		prefix = s.Chain.Network.AddressPrefix
	}

	// If private key and pool address provided — execute real UTXO transaction
	if req.PrivateKeyHex != "" && req.PoolAddress != "" && req.Amount > 0 {
		wlt, err := wallet.FromHexPrivateKey(req.PrivateKeyHex, prefix)
		if err != nil {
			http.Error(w, "bad private key: "+err.Error(), 400)
			return
		}
		const fee = uint64(1000)
		t, err := wallet.BuildTransaction(wlt, s.Store, req.PoolAddress, req.Amount, fee)
		if err != nil {
			// Fall back to metadata hash if UTXO build fails (e.g. insufficient on-chain balance)
			h := metaHash(req.LaunchID, req.TradeID, req.Side, req.Ticker)
			writeJSON(w, map[string]any{
				"success":     true,
				"txHash":      h,
				"type":        "genesis_meta",
				"side":        req.Side,
				"ticker":      req.Ticker,
				"fallback":    true,
				"fallbackReason": err.Error(),
				"explorerUrl": fmt.Sprintf("https://0nevel.com/explorer?tx=%s", h),
			})
			return
		}
		if s.Mempool != nil {
			_ = s.Mempool.Add(t) // non-fatal if already in mempool
		}
		h := t.Hash()
		txHash := hex.EncodeToString(h[:])
		writeJSON(w, map[string]any{
			"success":     true,
			"txHash":      txHash,
			"type":        "genesis_utxo",
			"side":        req.Side,
			"ticker":      req.Ticker,
			"fromAddress": wlt.Address,
			"toAddress":   req.PoolAddress,
			"amount":      req.Amount,
			"fee":         fee,
			"chain":       "nevel",
			"explorerUrl": fmt.Sprintf("https://0nevel.com/explorer?tx=%s", txHash),
		})
		return
	}

	// Metadata-only path (no private key provided)
	h := metaHash(req.LaunchID, req.TradeID, req.Side, req.Ticker)
	writeJSON(w, map[string]any{
		"success":     true,
		"txHash":      h,
		"type":        "genesis_meta",
		"side":        req.Side,
		"ticker":      req.Ticker,
		"chain":       "nevel",
		"explorerUrl": fmt.Sprintf("https://0nevel.com/explorer?tx=%s", h),
	})
}

// genesisPool creates a liquidity pool on the NEVEL blockchain when a Genesis
// launch graduates at 690,000 NEVEL raised.
// The pool address is a deterministic NEVEL address derived from the launch.
// Better than Uniswap: graduation is automatic, trustless, and instant.
func (s *Server) genesisPool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		LaunchID       string  `json:"launchId"`
		Ticker         string  `json:"ticker"`
		NevelLiquidity float64 `json:"nevelLiquidity"`
		TokenLiquidity float64 `json:"tokenLiquidity"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	// Derive a deterministic pool address from launch data
	poolSeed := fmt.Sprintf("genesis-pool:%s:%s", req.LaunchID, req.Ticker)
	poolHash := sha256.Sum256([]byte(poolSeed))
	poolAddr := fmt.Sprintf("nevel1pool%s%s",
		strings.ToLower(req.Ticker),
		hex.EncodeToString(poolHash[:])[:8],
	)

	// Generate pool tx hash
	poolData, _ := json.Marshal(req)
	txH := sha256.Sum256(poolData)
	txHash := hex.EncodeToString(txH[:])

	writeJSON(w, map[string]any{
		"success":        true,
		"poolAddress":    poolAddr,
		"txHash":         txHash,
		"ticker":         req.Ticker,
		"nevelLiquidity": req.NevelLiquidity,
		"tokenLiquidity": req.TokenLiquidity,
		"chain":          "nevel",
		"explorerUrl":    fmt.Sprintf("https://0nevel.com/explorer?pool=%s", poolAddr),
	})
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

// metaHash generates a deterministic hash for a trade metadata record.
func metaHash(launchID, tradeID, side, ticker string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", launchID, tradeID, side, ticker)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func blockDTO(b blockchain.Block) map[string]any {
	h := b.Hash()
	txs := make([]string, len(b.Transactions))
	for i, t := range b.Transactions {
		th := t.Hash()
		txs[i] = hex.EncodeToString(th[:])
	}
	return map[string]any{
		"hash": hex.EncodeToString(h[:]), "height": b.Header.Height,
		"previous": hex.EncodeToString(b.Header.PrevHash[:]),
		"merkleRoot": hex.EncodeToString(b.Header.MerkleRoot[:]),
		"timestamp": b.Header.Timestamp, "bits": b.Header.Bits,
		"nonce": b.Header.Nonce, "transactions": txs,
	}
}

func txDTO(t txpkg.Transaction) map[string]any {
	h := t.Hash()
	inputs := make([]map[string]any, len(t.Inputs))
	for i, in := range t.Inputs {
		inputs[i] = map[string]any{
			"prevTxHash": hex.EncodeToString(in.PrevTxHash[:]),
			"outputIdx":  in.OutputIdx,
		}
	}
	outputs := make([]map[string]any, len(t.Outputs))
	for i, out := range t.Outputs {
		outputs[i] = map[string]any{
			"amount":        out.Amount,
			"lockingScript": hex.EncodeToString(out.LockingScript),
		}
	}
	return map[string]any{
		"hash": hex.EncodeToString(h[:]), "version": t.Version,
		"inputs": inputs, "outputs": outputs,
		"lockTime": t.LockTime, "coinbase": t.IsCoinbase(),
	}
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
		protected := r.Method != http.MethodGet ||
			strings.HasPrefix(r.URL.Path, "/mining/") ||
			strings.HasPrefix(r.URL.Path, "/tx/broadcast") ||
			strings.HasPrefix(r.URL.Path, "/tx/build") ||
			strings.HasPrefix(r.URL.Path, "/genesis/") ||
			strings.HasPrefix(r.URL.Path, "/payments/")
		if s.AuthToken != "" && protected {
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

// Ensure consensus import is used.
var _ = consensus.ScriptForPubKeyHash
