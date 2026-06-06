package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/nevel/nevel-core/params"
	"github.com/nevel/nevel-core/wallet"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	netName := fs.String("network", "regtest", "network")
	rpcURL := fs.String("rpc", "http://127.0.0.1:18443", "RPC base URL")
	_ = fs.Parse(os.Args[2:])
	net := params.ByName(*netName)
	switch os.Args[1] {
	case "wallet":
		args := fs.Args()
		if len(args) == 0 || args[0] != "create" {
			usage()
			return
		}
		w, err := wallet.New(net.AddressPrefix)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("address=%s\nprivate_key_hex=%s\n", w.Address, wallet.PrivateKeyHex(w.PrivateKey))
	case "status":
		printGET(*rpcURL + "/chain/tip")
	case "block":
		args := fs.Args()
		if len(args) != 1 {
			usage()
			return
		}
		printGET(*rpcURL + "/block/height/" + args[0])
	case "tx":
		args := fs.Args()
		if len(args) != 1 {
			usage()
			return
		}
		printGET(*rpcURL + "/tx/" + args[0])
	case "balance":
		args := fs.Args()
		if len(args) != 1 {
			usage()
			return
		}
		printGET(*rpcURL + "/address/" + args[0] + "/balance")
	case "invoice":
		args := fs.Args()
		if len(args) < 3 {
			usage()
			return
		}
		body := fmt.Sprintf(`{"merchant":%q,"address":%q,"amount":%s,"memo":%q}`, args[0], args[1], args[2], strings.Join(args[3:], " "))
		printPOST(*rpcURL+"/payments/invoice", body)
	default:
		usage()
	}
}

func printGET(url string) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	printResponse(resp)
}

func printPOST(url, body string) {
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	printResponse(resp)
}

func printResponse(resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Fatalf("rpc error %s: %s", resp.Status, string(body))
	}
	var v any
	if err := json.Unmarshal(body, &v); err == nil {
		pretty, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(pretty))
		return
	}
	fmt.Print(string(body))
}

func usage() {
	fmt.Println("usage: nevel-cli wallet create | status | block <height> | tx <hash> | balance <address> | invoice <merchant> <address> <neveloshi> [memo]")
}
