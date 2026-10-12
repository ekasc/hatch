package modules

import (
	"sync"
)

// Clock is a deterministic double of the JS timer surface (setTimeout,
// setInterval, and the room-store debounce/reconnect delays). Scenarios
// advance virtual time explicitly instead of sleeping, so runs are instant
// and repeatable. Real-device divergence: no event-loop jitter, no timer
// throttling under load; FIDELITY.md row 4.
type Clock struct {
	mu     sync.Mutex
	now    int64
	nextID int
	timers map[int]*timer
	fired  []string
}

type timer struct {
	id       int
	fireAt   int64
	interval int64 // 0 = one-shot
	name     string
}

// NewClock returns a clock starting at virtual millisecond 0.
func NewClock() *Clock {
	return &Clock{timers: make(map[int]*timer)}
}

// Now returns the current virtual time in milliseconds.
func (c *Clock) Now() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// After schedules a one-shot timer firing after delayMs, returning its id.
func (c *Clock) After(delayMs int64, name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	c.timers[id] = &timer{id: id, fireAt: c.now + delayMs, name: name}
	return id
}

// Every schedules a repeating timer firing first after intervalMs.
func (c *Clock) Every(intervalMs int64, name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	c.timers[id] = &timer{id: id, fireAt: c.now + intervalMs, interval: intervalMs, name: name}
	return id
}

// Clear removes a timer. Clearing an unknown id is a no-op.
func (c *Clock) Clear(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.timers, id)
}

// Advance moves virtual time forward by deltaMs, firing due timers in
// fire-time order. A repeating timer that lapses multiple times fires once
// per missed interval. Returns the names of fired timers in order.
func (c *Clock) Advance(deltaMs int64) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	target := c.now + deltaMs
	var order []string
	for {
		var due *timer
		for _, tm := range c.timers {
			if tm.fireAt <= target && (due == nil || tm.fireAt < due.fireAt || (tm.fireAt == due.fireAt && tm.id < due.id)) {
				due = tm
			}
		}
		if due == nil {
			break
		}
		c.now = due.fireAt
		order = append(order, due.name)
		c.fired = append(c.fired, due.name)
		if due.interval > 0 {
			due.fireAt += due.interval
		} else {
			delete(c.timers, due.id)
		}
	}
	c.now = target
	return order
}

// Fired returns every timer name fired so far, in order.
func (c *Clock) Fired() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.fired...)
}

// Pending returns the number of live timers.
func (c *Clock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}
