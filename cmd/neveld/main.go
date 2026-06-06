package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nevel/nevel-core/chain"
	"github.com/nevel/nevel-core/mempool"
	"github.com/nevel/nevel-core/params"
	"github.com/nevel/nevel-core/rpc"
	"github.com/nevel/nevel-core/storage"
	"github.com/nevel/nevel-core/wallet"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	netName := fs.String("network", "regtest", "mainnet, testnet, or regtest")
	dataDir := fs.String("datadir", filepath.Join(os.TempDir(), "nevel"), "data directory")
	rpcAddr := fs.String("rpc", "127.0.0.1:18443", "rpc listen address")
	mineAddress := fs.String("address", "", "miner payout address")
	blocks := fs.Int("blocks", 1, "number of blocks to mine")
	rpcToken := fs.String("rpctoken", "", "bearer token required for protected RPC endpoints")
	_ = fs.Parse(os.Args[2:])
	net := params.ByName(*netName)
	dbPath := filepath.Join(*dataDir, *netName, "chain.pebble")
	switch os.Args[1] {
	case "init":
		manager := mustManager(dbPath, net)
		b, err := manager.InitGenesis(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		h := b.Hash()
		fmt.Printf("initialized %s genesis %x height %d\n", net.ID, h, b.Header.Height)
	case "start":
		manager := mustManager(dbPath, net)
		if _, err := manager.InitGenesis(context.Background()); err != nil {
			log.Fatal(err)
		}
		start(manager, *rpcAddr, *rpcToken)
	case "mine":
		manager := mustManager(dbPath, net)
		if _, err := manager.InitGenesis(context.Background()); err != nil {
			log.Fatal(err)
		}
		address := *mineAddress
		if address == "" {
			w, err := wallet.New(net.AddressPrefix)
			if err != nil {
				log.Fatal(err)
			}
			address = w.Address
			fmt.Printf("generated miner address=%s private_key_hex=%s\n", w.Address, wallet.PrivateKeyHex(w.PrivateKey))
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		verbose := *blocks <= 100
		summary, err := manager.MineBlocksStreaming(ctx, *blocks, address, func(tip chain.Tip) {
			if verbose || tip.Height%100000 == 0 {
				fmt.Printf("mined height=%d hash=%s\n", tip.Height, tip.Hash)
			}
		})
		if err != nil {
			log.Fatal(err)
		}
		if !verbose {
			fmt.Printf("mined blocks=%d first_height=%d last_height=%d last_hash=%s\n", summary.Blocks, summary.First.Height, summary.Last.Height, summary.Last.Hash)
		}
	case "status":
		manager := mustManager(dbPath, net)
		tip, err := manager.Tip()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("network=%s height=%d hash=%s bits=%08x\n", net.ID, tip.Height, tip.Hash, tip.Bits)
	default:
		usage()
	}
}

func mustManager(path string, net params.Network) *chain.Manager {
	db, err := storage.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	verifier := wallet.Verifier{}
	mp := mempool.New(params.DefaultMempoolMaxByte, db, verifier)
	return chain.New(db, net, mp, verifier)
}

func start(manager *chain.Manager, addr, token string) {
	defer manager.Store.Close()
	srv := rpc.NewWithChain(manager)
	srv.AuthToken = token
	log.Printf("neveld rpc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}

func usage() {
	fmt.Println("usage: neveld <init|start|mine|status> [--network regtest] [--datadir path] [--rpc addr] [--rpctoken token] [--blocks n] [--address addr]")
}
