// Package opqueue implements a concurrent operation queue with pause, resume, and cancel support.
package opqueue

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// OpType describes the type of file operation.
type OpType string

const (
	OpCopy   OpType = "copy"
	OpMove   OpType = "move"
	OpDelete OpType = "delete"
)

// OpStatus describes the current state of an operation.
type OpStatus string

const (
	StatusPending    OpStatus = "pending"
	StatusRunning    OpStatus = "running"
	StatusPaused     OpStatus = "paused"
	StatusCompleted  OpStatus = "completed"
	StatusCancelled  OpStatus = "cancelled"
	StatusFailed     OpStatus = "failed"
)

// Operation represents a single queued file operation.
type Operation struct {
	ID          string   `json:"id"`
	Type        OpType   `json:"type"`
	Sources     []string `json:"sources"`
	Destination string   `json:"destination"`
	Progress    float64  `json:"progress"`
	BytesDone   int64    `json:"bytesDone"`
	BytesTotal  int64    `json:"bytesTotal"`
	CurrentFile string   `json:"currentFile"`
	Error       string   `json:"error,omitempty"`
	StartedAt   int64    `json:"startedAt,omitempty"`
	CompletedAt int64    `json:"completedAt,omitempty"`

	mu     sync.RWMutex
	status OpStatus
	cancel chan struct{}
	paused atomic.Bool
}

// GetStatus returns the current status of the operation (thread-safe).
func (op *Operation) GetStatus() OpStatus {
	op.mu.RLock()
	defer op.mu.RUnlock()
	return op.status
}

// setStatus sets the status (thread-safe).
func (op *Operation) setStatus(s OpStatus) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.status = s
}

// ProgressCallback is called to report operation progress.
type ProgressCallback func(op *Operation)

// ExecuteFunc is the function that performs the actual file operation.
// It receives the operation and should update progress as it works.
type ExecuteFunc func(op *Operation) error

// Queue manages a queue of file operations with worker goroutines.
type Queue struct {
	mu         sync.RWMutex
	ops        []*Operation
	pending    chan *Operation
	onProgress ProgressCallback
	nextID     int64
	workers    int
	stopCh     chan struct{}
	wg         sync.WaitGroup
	executors  map[OpType]ExecuteFunc
}

// NewQueue creates an operation queue with the given number of worker goroutines.
func NewQueue(workers int, onProgress ProgressCallback) *Queue {
	if workers <= 0 {
		workers = 2
	}

	q := &Queue{
		ops:        make([]*Operation, 0),
		pending:    make(chan *Operation, 100),
		onProgress: onProgress,
		workers:    workers,
		stopCh:     make(chan struct{}),
		executors:  make(map[OpType]ExecuteFunc),
	}

	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}

	return q
}

// RegisterExecutor sets the function that performs a given operation type.
func (q *Queue) RegisterExecutor(opType OpType, fn ExecuteFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.executors[opType] = fn
}

// Enqueue adds a new operation to the queue and returns its ID.
func (q *Queue) Enqueue(opType OpType, sources []string, destination string, bytesTotal int64) string {
	q.mu.Lock()
	q.nextID++
	id := fmt.Sprintf("op-%d", q.nextID)
	q.mu.Unlock()

	op := &Operation{
		ID:          id,
		Type:        opType,
		Sources:     sources,
		Destination: destination,
		status:      StatusPending,
		BytesTotal:  bytesTotal,
		cancel:      make(chan struct{}),
	}

	q.mu.Lock()
	q.ops = append(q.ops, op)
	q.mu.Unlock()

	q.pending <- op
	return id
}

// Cancel cancels the operation with the given ID.
func (q *Queue) Cancel(id string) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, op := range q.ops {
		if op.ID == id {
			select {
			case <-op.cancel:
				// Already cancelled.
			default:
				close(op.cancel)
			}
			op.setStatus(StatusCancelled)
			return nil
		}
	}
	return fmt.Errorf("operation not found: %s", id)
}

// Pause pauses the operation with the given ID.
func (q *Queue) Pause(id string) error {
	op := q.findOp(id)
	if op == nil {
		return fmt.Errorf("operation not found: %s", id)
	}
	op.paused.Store(true)
	op.setStatus(StatusPaused)
	q.notifyProgress(op)
	return nil
}

// Resume resumes a paused operation.
func (q *Queue) Resume(id string) error {
	op := q.findOp(id)
	if op == nil {
		return fmt.Errorf("operation not found: %s", id)
	}
	op.paused.Store(false)
	op.setStatus(StatusRunning)
	q.notifyProgress(op)
	return nil
}

// List returns all operations in the queue.
func (q *Queue) List() []*Operation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Operation, len(q.ops))
	copy(result, q.ops)
	return result
}

// ActiveCount returns the number of running or pending operations.
func (q *Queue) ActiveCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	count := 0
	for _, op := range q.ops {
		s := op.GetStatus()
		if s == StatusPending || s == StatusRunning || s == StatusPaused {
			count++
		}
	}
	return count
}

// ClearCompleted removes completed/cancelled/failed operations from the list.
func (q *Queue) ClearCompleted() {
	q.mu.Lock()
	defer q.mu.Unlock()

	active := make([]*Operation, 0, len(q.ops))
	for _, op := range q.ops {
		s := op.GetStatus()
		if s == StatusPending || s == StatusRunning || s == StatusPaused {
			active = append(active, op)
		}
	}
	q.ops = active
}

// Stop shuts down the queue and waits for workers to finish.
func (q *Queue) Stop() {
	close(q.stopCh)
	q.wg.Wait()
}

// IsCancelled checks if the operation has been cancelled.
func (op *Operation) IsCancelled() bool {
	select {
	case <-op.cancel:
		return true
	default:
		return false
	}
}

// IsPaused checks if the operation is paused.
func (op *Operation) IsPaused() bool {
	return op.paused.Load()
}

// WaitIfPaused blocks until the operation is unpaused or cancelled.
// Returns true if cancelled.
func (op *Operation) WaitIfPaused() bool {
	for op.IsPaused() {
		select {
		case <-op.cancel:
			return true
		case <-time.After(100 * time.Millisecond):
		}
	}
	return op.IsCancelled()
}

func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stopCh:
			return
		case op := <-q.pending:
			q.execute(op)
		}
	}
}

func (q *Queue) execute(op *Operation) {
	op.setStatus(StatusRunning)
	op.StartedAt = time.Now().Unix()
	q.notifyProgress(op)

	q.mu.RLock()
	executor := q.executors[op.Type]
	q.mu.RUnlock()

	if executor == nil {
		op.setStatus(StatusFailed)
		op.Error = fmt.Sprintf("no executor registered for %s", op.Type)
		q.notifyProgress(op)
		return
	}

	err := executor(op)
	if err != nil {
		if op.IsCancelled() {
			op.setStatus(StatusCancelled)
		} else {
			op.setStatus(StatusFailed)
			op.Error = err.Error()
		}
	} else {
		op.setStatus(StatusCompleted)
		op.Progress = 100
	}

	op.CompletedAt = time.Now().Unix()
	q.notifyProgress(op)
}

func (q *Queue) findOp(id string) *Operation {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, op := range q.ops {
		if op.ID == id {
			return op
		}
	}
	return nil
}

func (q *Queue) notifyProgress(op *Operation) {
	if q.onProgress != nil {
		q.onProgress(op)
	}
}
