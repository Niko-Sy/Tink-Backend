package websocketmsg

import (
	"chatroombackend/middleware"
	"github.com/golang-jwt/jwt/v5"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTicketExpiryAndAtomicConsumption(t *testing.T) {
	s := NewTicketStore()
	claims := &middleware.CustomClaims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s.values["old"] = ticket{claims, time.Now().Add(-time.Second)}
	if _, e := s.Consume("old"); e == nil {
		t.Fatal("expired")
	}
	s.values["concurrent"] = ticket{claims, time.Now().Add(time.Minute)}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Consume("concurrent"); e == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal(successes.Load())
	}
}
