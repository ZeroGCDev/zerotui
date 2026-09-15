package app

import (
	"sync"
	"testing"
	"time"
)

func TestQueueBoundedAndConcurrent(t *testing.T) {
	a := New(nil, nil)
	var mu sync.Mutex
	n := 0
	for i := 0; i < 256; i++ {
		if !a.Queue(func() { mu.Lock(); n++; mu.Unlock() }) {
			t.Fatalf("queue rejected item %d", i)
		}
	}
	if a.Queue(func() {}) {
		t.Fatal("queue accepted item past fixed capacity")
	}
	for i := 0; i < 256; i++ {
		fn := <-a.uiQueue
		fn()
	}
	if n != 256 {
		t.Fatalf("ran %d queued callbacks", n)
	}
}

func TestQueueProducerDoesNotRunInline(t *testing.T) {
	a := New(nil, nil)
	ran := make(chan struct{}, 1)
	if !a.Queue(func() { ran <- struct{}{} }) {
		t.Fatal("queue rejected")
	}
	select {
	case <-ran:
		t.Fatal("callback ran inline")
	case <-time.After(time.Millisecond):
	}
}
