package opqueue

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnqueueAndExecute(t *testing.T) {
	var completed atomic.Int32

	q := NewQueue(1, nil)
	defer q.Stop()

	q.RegisterExecutor(OpCopy, func(op *Operation) error {
		completed.Add(1)
		op.Progress = 100
		return nil
	})

	id := q.Enqueue(OpCopy, []string{"/a"}, "/b", 1024)
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// Wait for the operation to complete.
	deadline := time.After(2 * time.Second)
	for {
		ops := q.List()
		if len(ops) > 0 && ops[0].GetStatus() == StatusCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("operation did not complete in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if completed.Load() != 1 {
		t.Errorf("expected 1 execution, got %d", completed.Load())
	}
}

func TestCancel(t *testing.T) {
	q := NewQueue(1, nil)
	defer q.Stop()

	started := make(chan struct{})
	q.RegisterExecutor(OpCopy, func(op *Operation) error {
		close(started)
		// Block until cancelled.
		for !op.IsCancelled() {
			time.Sleep(10 * time.Millisecond)
		}
		return fmt.Errorf("cancelled")
	})

	id := q.Enqueue(OpCopy, []string{"/a"}, "/b", 0)

	// Wait until the operation starts.
	<-started

	if err := q.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Wait for status update.
	deadline := time.After(2 * time.Second)
	for {
		ops := q.List()
		if len(ops) > 0 && (ops[0].GetStatus() == StatusCancelled || ops[0].GetStatus() == StatusFailed) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("cancel did not complete in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestProgressCallback(t *testing.T) {
	var progressCalls atomic.Int32

	q := NewQueue(1, func(op *Operation) {
		progressCalls.Add(1)
	})
	defer q.Stop()

	q.RegisterExecutor(OpMove, func(op *Operation) error {
		return nil
	})

	q.Enqueue(OpMove, []string{"/a"}, "/b", 0)

	// Wait for completion.
	deadline := time.After(2 * time.Second)
	for {
		if progressCalls.Load() >= 2 { // At least start + complete.
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected at least 2 progress calls, got %d", progressCalls.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestQueueOrdering(t *testing.T) {
	var order []string
	var mu = make(chan struct{}, 1)
	done := make(chan struct{})

	q := NewQueue(1, nil)
	defer q.Stop()

	q.RegisterExecutor(OpCopy, func(op *Operation) error {
		mu <- struct{}{}
		order = append(order, op.Sources[0])
		<-mu
		if len(order) == 3 {
			close(done)
		}
		return nil
	})

	q.Enqueue(OpCopy, []string{"first"}, "/dst", 0)
	q.Enqueue(OpCopy, []string{"second"}, "/dst", 0)
	q.Enqueue(OpCopy, []string{"third"}, "/dst", 0)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("operations did not complete in time")
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestActiveCount(t *testing.T) {
	q := NewQueue(1, nil)
	defer q.Stop()

	blocker := make(chan struct{})
	q.RegisterExecutor(OpCopy, func(op *Operation) error {
		<-blocker
		return nil
	})

	q.Enqueue(OpCopy, []string{"/a"}, "/b", 0)
	q.Enqueue(OpCopy, []string{"/c"}, "/d", 0)

	time.Sleep(50 * time.Millisecond)

	count := q.ActiveCount()
	if count != 2 {
		t.Errorf("expected 2 active ops, got %d", count)
	}

	close(blocker)
}

func TestClearCompleted(t *testing.T) {
	q := NewQueue(1, nil)
	defer q.Stop()

	q.RegisterExecutor(OpCopy, func(op *Operation) error {
		return nil
	})

	q.Enqueue(OpCopy, []string{"/a"}, "/b", 0)

	// Wait for completion.
	deadline := time.After(2 * time.Second)
	for {
		if q.ActiveCount() == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("op did not complete")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if len(q.List()) != 1 {
		t.Fatalf("expected 1 op in list, got %d", len(q.List()))
	}

	q.ClearCompleted()

	if len(q.List()) != 0 {
		t.Errorf("expected 0 ops after clear, got %d", len(q.List()))
	}
}
