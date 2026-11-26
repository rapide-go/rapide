package broadcast

import (
	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
)

// SSEHandler returns an Rapide route Handler that streams Server-Sent Events for channel
// from the Default global Hub using Rapide's c.SSE() writer.
func SSEHandler(channel string) router.Handler {
	return SSEHandlerOn(Default, channel)
}

// SSEHandlerOn returns a Server-Sent Events streaming Handler bound to a specific Hub.
func SSEHandlerOn(hub *Hub, channel string) router.Handler {
	return func(c *ctx.Ctx) error {
		sse := c.SSE()
		ch, unsub := hub.Subscribe(channel)
		defer unsub()

		for {
			select {
			case <-sse.Done():
				return nil
			case msg, ok := <-ch:
				if !ok {
					return nil
				}
				if err := sse.Send(msg.Event, string(msg.Data)); err != nil {
					return nil
				}
			}
		}
	}
}
