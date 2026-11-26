package queue_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/rapide-go/rapide/queue"
)

func TestChainAndBatch(t *testing.T) {
	var order []int
	var caught error

	chain := queue.NewChain(
		queue.JobFunc(func(context.Context) error { order = append(order, 1); return nil }),
		queue.JobFunc(func(context.Context) error { order = append(order, 2); return errors.New("boom") }),
		queue.JobFunc(func(context.Context) error { order = append(order, 3); return nil }),
	).Catch(func(err error) { caught = err })

	if err := chain.Handle(context.Background()); err == nil || caught == nil {
		t.Fatalf("expected chain error & catch")
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected chain to stop at step 2, got %v", order)
	}

	var ran atomic.Int32
	thenCalled := false
	finallyCalled := false

	batch := queue.NewBatch(
		queue.JobFunc(func(context.Context) error { ran.Add(1); return nil }),
		queue.JobFunc(func(context.Context) error { ran.Add(1); return nil }),
	).Then(func() { thenCalled = true }).Finally(func() { finallyCalled = true })

	if err := batch.Handle(context.Background()); err != nil {
		t.Fatalf("unexpected batch err: %v", err)
	}
	if ran.Load() != 2 || !thenCalled || !finallyCalled {
		t.Fatalf("batch hooks failed: ran=%d then=%v finally=%v", ran.Load(), thenCalled, finallyCalled)
	}
}
