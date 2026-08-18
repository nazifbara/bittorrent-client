package main

import (
	"context"
	"testing"
	"time"
)

// newTestClient builds a minimal Client sufficient for queue.go's methods.
// t.Cleanup cancels its context so any background goroutine spawned by
// requeue's non-blocking fallback path unblocks once the test ends.
func newTestClient(t *testing.T, queueCap int) *Client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Client{
		queue:   make(chan *Job, queueCap),
		pending: make(map[string]*pendingRequest),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func TestPendingKey(t *testing.T) {
	got := pendingKey(3, 16384)
	want := "3:16384"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnqueueDeliversJob(t *testing.T) {
	c := newTestClient(t, 1)
	job := &Job{index: 1, begin: 0}
	c.enqueue(job)

	select {
	case got := <-c.queue:
		if got != job {
			t.Fatal("expected the same job pointer back")
		}
	default:
		t.Fatal("expected job to be enqueued")
	}
}

func TestEnqueueUnblocksOnShutdown(t *testing.T) {
	c := newTestClient(t, 1)
	c.enqueue(&Job{index: 0, begin: 0}) // fill the buffer
	c.cancel()

	done := make(chan struct{})
	go func() {
		c.enqueue(&Job{index: 1, begin: 0}) // buffer full, must not block forever
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("enqueue did not return after context cancellation")
	}
}

func TestRequeueNonBlockingWhenQueueFull(t *testing.T) {
	c := newTestClient(t, 1)
	c.enqueue(&Job{index: 0, begin: 0}) // fill the buffer

	done := make(chan struct{})
	go func() {
		c.requeue(&Job{index: 1, begin: 0})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("requeue blocked instead of falling back to a background retry")
	}
}

func TestOnTimeoutRequeuesStillPendingJob(t *testing.T) {
	c := newTestClient(t, 1)
	job := &Job{index: 2, begin: 100}
	key := pendingKey(job.index, job.begin)
	c.pending[key] = &pendingRequest{job: job}

	c.onTimeout(key, job)

	if _, stillPending := c.pending[key]; stillPending {
		t.Fatal("expected pending entry to be removed")
	}
	select {
	case got := <-c.queue:
		if got != job {
			t.Fatal("expected the timed-out job to be requeued")
		}
	default:
		t.Fatal("expected job to be requeued after timeout")
	}
}

func TestOnTimeoutIgnoresAlreadyResolvedJob(t *testing.T) {
	c := newTestClient(t, 1)
	job := &Job{index: 2, begin: 100}
	key := pendingKey(job.index, job.begin)
	// no entry in c.pending -- simulates the block having already arrived
	// and been cleaned up before the timer fired

	c.onTimeout(key, job)

	select {
	case <-c.queue:
		t.Fatal("did not expect job to be requeued when it was already resolved")
	default:
	}
}

func TestAddToQueueSkipsCompletedPiece(t *testing.T) {
	c := newTestClient(t, 4)
	c.piecesGrid = []*PieceState{{Done: true, TotalBlocks: 1}}

	c.AddToQueue(0, 0)

	select {
	case <-c.queue:
		t.Fatal("did not expect a job for an already-completed piece")
	default:
	}
}

func TestAddPieceJobsEnqueuesEveryBlock(t *testing.T) {
	c := newTestClient(t, 8)
	c.piecesGrid = []*PieceState{{TotalBlocks: 3}}

	c.addPieceJobs(0)

	gotOffsets := map[uint32]bool{}
	for i := 0; i < 3; i++ {
		select {
		case job := <-c.queue:
			if job.index != 0 {
				t.Fatalf("expected piece index 0, got %d", job.index)
			}
			gotOffsets[job.begin] = true
		default:
			t.Fatalf("expected 3 jobs total, only got %d", i)
		}
	}
	for _, want := range []uint32{0, blockSize, 2 * blockSize} {
		if !gotOffsets[want] {
			t.Fatalf("expected a job at begin=%d, got offsets %v", want, gotOffsets)
		}
	}
}

func TestShutdownReturnsWhenNoWorkersRunning(t *testing.T) {
	c := newTestClient(t, 1)
	done := make(chan struct{})
	go func() {
		c.shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not return")
	}
}
