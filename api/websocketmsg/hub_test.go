package websocketmsg

import (
	"sync"
	"testing"
)

func TestBoundedQueueAndConcurrentStop(t *testing.T) {
	c := &Client{Send: make(chan []byte, 1), Done: make(chan struct{}), Rooms: map[string]bool{}}
	c.enqueue([]byte("first"))
	c.enqueue([]byte("overflow"))
	select {
	case <-c.Done:
	default:
		t.Fatal("slow connection not stopped")
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.enqueue([]byte("late")); c.stop() }()
	}
	wg.Wait()
}
