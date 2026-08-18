package main

import (
	"sync"
	"testing"
	"time"
)

func TestRTTTrackerDefaultTimeout(t *testing.T) {
	r := newRTTTracker(10)
	got := r.timeout()
	want := 8 * time.Second
	if got != want {
		t.Fatalf("expected default timeout %v, got %v", want, got)
	}
}

func TestRTTTrackerIgnoresNonPositiveSamples(t *testing.T) {
	r := newRTTTracker(10)
	r.record(0)
	r.record(-5 * time.Millisecond)
	got := r.timeout()
	want := 8 * time.Second
	if got != want {
		t.Fatalf("expected default timeout after ignored samples, got %v", got)
	}
}

func TestRTTTrackerAverageWithinBounds(t *testing.T) {
	r := newRTTTracker(10)
	r.record(1 * time.Second)
	r.record(2 * time.Second)
	// avg = 1.5s, timeout = avg*4 = 6s, within [2s, 20s]
	got := r.timeout()
	want := 6 * time.Second
	if got != want {
		t.Fatalf("expected timeout %v, got %v", want, got)
	}
}

func TestRTTTrackerEvictsOldestSampleBeyondMaxLen(t *testing.T) {
	r := newRTTTracker(2)
	r.record(1 * time.Second)
	r.record(2 * time.Second)
	r.record(3 * time.Second)
	// oldest (1s) should have been evicted, leaving [2s, 3s] -> avg 2.5s -> timeout 10s
	got := r.timeout()
	want := 10 * time.Second
	if got != want {
		t.Fatalf("expected timeout %v after eviction, got %v", want, got)
	}
	if len(r.samples) != 2 {
		t.Fatalf("expected 2 retained samples, got %d", len(r.samples))
	}
}

func TestRTTTrackerClampsLowTimeout(t *testing.T) {
	r := newRTTTracker(10)
	r.record(10 * time.Millisecond)
	got := r.timeout()
	want := 2 * time.Second
	if got != want {
		t.Fatalf("expected clamped minimum timeout %v, got %v", want, got)
	}
}

func TestRTTTrackerClampsHighTimeout(t *testing.T) {
	r := newRTTTracker(10)
	r.record(10 * time.Second)
	got := r.timeout()
	want := 20 * time.Second
	if got != want {
		t.Fatalf("expected clamped maximum timeout %v, got %v", want, got)
	}
}

func TestRTTTrackerConcurrentRecordIsSafe(t *testing.T) {
	r := newRTTTracker(50)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.record(time.Duration(n+1) * time.Millisecond)
		}(i)
	}
	wg.Wait()
	// Just verifying no race/panic and a sane result is produced.
	// Run with `go test -race` to actually exercise the concurrency guarantee.
	if got := r.timeout(); got <= 0 {
		t.Fatalf("expected a positive timeout, got %v", got)
	}
}
