package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// newTest returns a limiter with a fake clock we can move forward by hand.
func newTest(limit int, window time.Duration) (*Limiter, *time.Time) {
	l := New(limit, window)
	clock := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }
	return l, &clock
}

func TestAllowUpToLimit(t *testing.T) {
	l, _ := newTest(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if ok, _ := l.Allow("ip1"); !ok {
			t.Fatalf("attempt %d refused, want allowed", i)
		}
	}
	if ok, _ := l.Allow("ip1"); ok {
		t.Fatal("attempt 4 allowed, want refused")
	}
}

func TestKeysAreSeparate(t *testing.T) {
	l, _ := newTest(1, time.Minute)
	l.Allow("ip1")
	if ok, _ := l.Allow("ip2"); !ok {
		t.Error("ip2 was blocked because of ip1")
	}
}

func TestRetryAfter(t *testing.T) {
	l, clock := newTest(1, time.Minute)
	l.Allow("ip1")
	*clock = clock.Add(20 * time.Second)

	ok, wait := l.Allow("ip1")
	if ok {
		t.Fatal("want refused")
	}
	if wait != 40*time.Second {
		t.Errorf("retryAfter = %v, want 40s", wait)
	}
}

func TestWindowResets(t *testing.T) {
	l, clock := newTest(1, time.Minute)
	l.Allow("ip1")
	if ok, _ := l.Allow("ip1"); ok {
		t.Fatal("want refused inside the window")
	}
	*clock = clock.Add(time.Minute)
	if ok, _ := l.Allow("ip1"); !ok {
		t.Error("still refused after the window ended")
	}
}

func TestSweepRemovesOldEntries(t *testing.T) {
	l, clock := newTest(5, time.Minute)
	l.Allow("ip1")
	l.Allow("ip2")
	*clock = clock.Add(2 * time.Minute)
	l.Allow("ip3") // triggers the sweep

	if len(l.entries) != 1 {
		t.Errorf("entries = %d, want 1 (old ones swept)", len(l.entries))
	}
}

// Run with `go test -race` to check the lock really protects the map.
func TestConcurrentUse(t *testing.T) {
	l := New(1000, time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				if ok, _ := l.Allow("same-ip"); ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	// 1500 attempts at the same time: exactly 1000 may pass, never more.
	if allowed != 1000 {
		t.Errorf("allowed = %d, want exactly 1000", allowed)
	}
}
