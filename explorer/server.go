package explorer

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/nevel/nevel-core/chain"
	"github.com/nevel/nevel-core/storage"
)

type Server struct {
	Chain *chain.Manager
	Store *storage.DB
}

func NewServer(manager *chain.Manager) *Server { return &Server{Chain: manager, Store: manager.Store} }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/mempool", s.mempool)
	return mux
}
func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	tip, err := s.Chain.Tip()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	stats := ChainStats{Height: tip.Height, Difficulty: tip.Bits}
	if s.Chain.Mempool != nil {
		stats.MempoolCount = len(s.Chain.Mempool.Snapshot())
	}
	_ = indexTemplate.Execute(w, map[string]any{"Tip": tip, "Stats": stats})
}
func (s *Server) mempool(w http.ResponseWriter, _ *http.Request) {
	count := 0
	if s.Chain.Mempool != nil {
		count = len(s.Chain.Mempool.Snapshot())
	}
	_, _ = fmt.Fprintf(w, "NEVEL mempool transactions: %d\n", count)
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html><html><head><title>NEVEL Explorer</title></head><body><h1>NEVEL Explorer</h1><p>Height: {{.Stats.Height}}</p><p>Tip: {{.Tip.Hash}}</p><p>Difficulty bits: {{printf "%08x" .Stats.Difficulty}}</p><p>Mempool: {{.Stats.MempoolCount}}</p></body></html>`))
