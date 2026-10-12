package modules

import (
	"testing"
)

func TestBroadcastReachesSubscriber(t *testing.T) {
	h := NewHub()
	ch := h.Subscribe("AK8F")
	h.BroadcastInvalidation("AK8F", 2)
	select {
	case raw := <-ch:
		inv, err := DecodeInvalidation(raw)
		if err != nil {
			t.Fatalf("DecodeInvalidation: %v", err)
		}
		if inv.StateVersion != 2 {
			t.Fatalf("stateVersion = %d, want 2", inv.StateVersion)
		}
	default:
		t.Fatal("subscriber received nothing")
	}
}

func TestBroadcastIsolatedPerRoom(t *testing.T) {
	h := NewHub()
	other := h.Subscribe("ZZZZ")
	h.Subscribe("AK8F")
	h.BroadcastInvalidation("AK8F", 3)
	select {
	case msg := <-other:
		t.Fatalf("wrong room got message: %q", msg)
	default:
	}
}

func TestPingPong(t *testing.T) {
	if got := Ping(); got != "pong" {
		t.Fatalf("Ping() = %q, want pong", got)
	}
}

func TestDecodeRejectsForeignMessages(t *testing.T) {
	if _, err := DecodeInvalidation(`{"type":"hello"}`); err == nil {
		t.Fatal("expected error for foreign message type")
	}
	if _, err := DecodeInvalidation(`not json`); err == nil {
		t.Fatal("expected error for non-JSON")
	}
}
