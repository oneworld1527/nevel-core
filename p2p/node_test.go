package p2p

import (
	"bufio"
	"net"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	want := Message{Type: MsgPing, Payload: []byte("hello")}
	go func() {
		_ = writeMessage(left, want)
	}()
	got, err := readMessage(bufio.NewReader(right))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != want.Type || string(got.Payload) != string(want.Payload) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
