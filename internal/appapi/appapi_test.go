package appapi

import "testing"

func TestMakePayload(t *testing.T) {
	payload, err := makePayload(170001, 279786, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 6 {
		t.Fatalf("payload too short: %d", len(payload))
	}
	if payload[0] != 1 {
		t.Fatalf("payload should be gzip framed")
	}
	msg, err := readMessage(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) == 0 {
		t.Fatal("empty protobuf payload")
	}
}
