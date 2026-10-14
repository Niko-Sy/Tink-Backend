package server

import (
	"chatroombackend/api/websocketmsg"
	"chatroombackend/internal/telemetry"
	"chatroombackend/middleware"
	"chatroombackend/service/group"
	"chatroombackend/service/invitation"
	"chatroombackend/service/social"
	"context"
	"sync"
	"time"
)

// StartBackground returns completion so shutdown waits before closing the DB.
func StartBackground(ctx context.Context, m *middleware.DBManager, c social.Capabilities) <-chan struct{} {
	done := make(chan struct{})
	g := group.New(m.GetDB(), m.GetQueries(), websocketmsg.PublishGroupEvent)
	s := invitation.New(m.GetDB(), m.GetQueries(), c.RoomInvitations, g, websocketmsg.PublishSocialEvent)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); s.RunCleanup(ctx) }()
	go func() {
		defer wg.Done()
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				telemetry.Log()
				return
			case <-tick.C:
				telemetry.Log()
			}
		}
	}()
	go func() { wg.Wait(); close(done) }()
	return done
}
