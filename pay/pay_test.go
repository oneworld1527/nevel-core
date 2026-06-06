package pay

import (
	"testing"
	"time"
)

func TestPaymentLifecycle(t *testing.T) {
	store := NewStore()
	req, err := store.Create("merchant", "rnevel1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqh0pu8j", 100, "memo", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != StatusPending || req.ID == "" {
		t.Fatalf("bad request: %#v", req)
	}
	updated, err := store.MarkBroadcasted(req.ID, "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusBroadcasted || updated.TxHash != "abcd" {
		t.Fatalf("bad broadcast update: %#v", updated)
	}
	if sig := SignWebhook([]byte("secret"), []byte("body")); sig == "" {
		t.Fatal("empty webhook signature")
	}
}
