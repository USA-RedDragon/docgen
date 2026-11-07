package gen

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlotsHoldInNestedPools(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{1, 2} {
		s := newSlots(limit)
		outer := newPool(t.Context(), s)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				inner := newPool(t.Context(), s)
				release := inner.hold(limit)
				release()
			})
		}
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Minute):
			t.Fatalf("limit %d: the holds of the pools did not finish", limit)
		}
		outer.wg.Wait()
	}
}

func TestSlotsLimitNestedWork(t *testing.T) {
	t.Parallel()
	const limit = 2
	s := newSlots(limit)
	var running, peak atomic.Int32
	leaf := func(context.Context) {
		n := running.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
	}
	outer := newPool(t.Context(), s)
	for range 4 {
		outer.run(func(ctx context.Context) {
			outer.withoutSlot(func() {
				inner := newPool(ctx, s)
				for range 8 {
					inner.run(leaf)
				}
				inner.wg.Wait()
			})
		})
	}
	outer.wg.Wait()
	if got := peak.Load(); got > limit {
		t.Errorf("%d functions at the same time, want at most %d", got, limit)
	}
}

func TestEachLimit(t *testing.T) {
	t.Parallel()
	const limit = 2
	g := &Generator{slots: newSlots(limit)}
	var running, peak, calls atomic.Int64
	g.each(t.Context(), 64, func(int) {
		calls.Add(1)
		if n := running.Add(1); n > limit {
			peak.Store(n)
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
	})
	if calls.Load() != 64 {
		t.Errorf("%d calls, want 64", calls.Load())
	}
	if n := peak.Load(); n != 0 {
		t.Errorf("%d calls at the same time, limit %d", n, limit)
	}
}

func TestEachStopsWithTheContext(t *testing.T) {
	t.Parallel()
	g := &Generator{slots: newSlots(1)}
	ctx, cancel := context.WithCancel(t.Context())
	var calls atomic.Int64
	g.each(ctx, 64, func(int) {
		calls.Add(1)
		cancel()
	})
	if calls.Load() != 1 {
		t.Errorf("%d calls after the end of the context, want 1", calls.Load())
	}
}
