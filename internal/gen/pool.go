package gen

import (
	"context"
	"sync"
	"sync/atomic"
)

// slots limits the work of a run and of its nested runs. A hold of several slots
// takes the mutex, so that two holds cannot each wait for a slot of the other.
type slots struct {
	sem chan struct{}
	mu  sync.Mutex
}

func newSlots(limit int) *slots {
	return &slots{sem: make(chan struct{}, limit)}
}

// A pool runs functions at the same time, each in a slot. A function that has
// not started when ctx ends does not run.
type pool struct {
	ctx   context.Context //nolint:containedctx // the context of the functions of one call of prepare
	slots *slots
	wg    sync.WaitGroup
}

func newPool(ctx context.Context, s *slots) *pool {
	return &pool{ctx: ctx, slots: s}
}

// hold takes n slots and returns the function that releases them. It waits for
// slots that other pools hold.
func (w *pool) hold(n int) func() {
	w.slots.mu.Lock()
	defer w.slots.mu.Unlock()
	for range n {
		w.slots.sem <- struct{}{}
	}
	return func() {
		for range n {
			<-w.slots.sem
		}
	}
}

func (w *pool) run(fn func(ctx context.Context)) {
	w.wg.Go(func() {
		select {
		case w.slots.sem <- struct{}{}:
		case <-w.ctx.Done():
			return
		}
		defer func() { <-w.slots.sem }()
		if w.ctx.Err() == nil {
			fn(w.ctx)
		}
	})
}

// withoutSlot calls fn from a function of run, with the slot of that function
// free for the work of fn.
func (w *pool) withoutSlot(fn func()) {
	<-w.slots.sem
	defer func() { w.slots.sem <- struct{}{} }()
	fn()
}

// each calls fn with each index below n, in the slots of g, at most one call for
// each slot at the same time. It returns when the calls end. After ctx ends, no
// call starts.
func (g *Generator) each(ctx context.Context, n int, fn func(i int)) {
	var next atomic.Int64
	w := newPool(ctx, g.slots)
	for range min(n, cap(g.slots.sem)) {
		w.run(func(ctx context.Context) {
			for ctx.Err() == nil {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				fn(i)
			}
		})
	}
	w.wg.Wait()
}
