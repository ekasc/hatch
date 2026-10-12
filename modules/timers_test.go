package modules

import (
	"reflect"
	"testing"
)

func TestAfterFiresInOrder(t *testing.T) {
	c := NewClock()
	c.After(100, "slow")
	c.After(10, "fast")
	c.After(50, "mid")
	if got := c.Advance(100); !reflect.DeepEqual(got, []string{"fast", "mid", "slow"}) {
		t.Fatalf("fired = %v", got)
	}
	if n := c.Pending(); n != 0 {
		t.Fatalf("pending = %d, want 0", n)
	}
}

func TestAdvanceIsPartial(t *testing.T) {
	c := NewClock()
	c.After(100, "slow")
	if got := c.Advance(30); len(got) != 0 {
		t.Fatalf("fired early: %v", got)
	}
	if got := c.Now(); got != 30 {
		t.Fatalf("now = %d, want 30", got)
	}
	if got := c.Advance(70); !reflect.DeepEqual(got, []string{"slow"}) {
		t.Fatalf("fired = %v", got)
	}
}

func TestEveryRepeats(t *testing.T) {
	c := NewClock()
	c.Every(25, "ping") // the room-store 25s heartbeat shape
	if got := c.Advance(100); !reflect.DeepEqual(got, []string{"ping", "ping", "ping", "ping"}) {
		t.Fatalf("fired = %v", got)
	}
	if n := c.Pending(); n != 1 {
		t.Fatalf("pending = %d, want repeating timer to survive", n)
	}
}

func TestClearCancels(t *testing.T) {
	c := NewClock()
	id := c.After(10, "cancelled")
	c.Clear(id)
	c.Clear(999) // unknown id is a no-op
	if got := c.Advance(100); len(got) != 0 {
		t.Fatalf("fired = %v, want none", got)
	}
}

func TestDebounceShape(t *testing.T) {
	// room-store invalidations reset an 80ms timer; only the last one fires.
	c := NewClock()
	var id int
	fire := func() { id = c.After(80, "refresh") }
	fire()
	c.Advance(50)
	c.Clear(id)
	fire()
	if got := c.Advance(79); len(got) != 0 {
		t.Fatalf("debounce fired early: %v", got)
	}
	if got := c.Advance(1); !reflect.DeepEqual(got, []string{"refresh"}) {
		t.Fatalf("fired = %v, want single refresh", got)
	}
	if got := c.Fired(); !reflect.DeepEqual(got, []string{"refresh"}) {
		t.Fatalf("history = %v", got)
	}
}
