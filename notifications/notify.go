package notify

import (
	"context"
	"errors"

	"github.com/rapide-go/rapide/broadcasting"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/http/client"
	"github.com/rapide-go/rapide/mail"
	"github.com/rapide-go/rapide/queue"
)

// Notifiable is implemented by recipients (such as models.User) to route notifications.
type Notifiable interface {
	NotificationEmail() string
	NotificationChannel() string
}

// Notification declares which channels ("mail", "broadcast", "database", "webhook") a notification uses.
type Notification interface {
	Via() []string
}

// MailNotification is implemented by notifications deliverable over email.
type MailNotification interface {
	ToMail(recipient Notifiable) mail.Message
}

// BroadcastNotification is implemented by notifications deliverable over real-time SSE.
type BroadcastNotification interface {
	ToBroadcast(recipient Notifiable) (event string, payload map[string]any)
}

// Send delivers notification n to recipient synchronously across all channels returned by n.Via().
func Send(ctx context.Context, recipient Notifiable, n Notification) error {
	var errs []error
	for _, ch := range n.Via() {
		switch ch {
		case "mail":
			if mn, ok := n.(MailNotification); ok {
				msg := mn.ToMail(recipient)
				if len(msg.To) == 0 && recipient.NotificationEmail() != "" {
					msg.To = []string{recipient.NotificationEmail()}
				}
				if err := mail.Send(ctx, msg); err != nil {
					errs = append(errs, err)
				}
			}
		case "broadcast":
			if bn, ok := n.(BroadcastNotification); ok {
				if channel := recipient.NotificationChannel(); channel != "" {
					event, payload := bn.ToBroadcast(recipient)
					if err := broadcast.Publish(channel, event, payload); err != nil {
						errs = append(errs, err)
					}
				}
			}
		case "database":
			if dn, ok := n.(DatabaseNotification); ok {
				if store, err := container.TryUse[*Store](); err == nil && store != nil {
					id, typ, data := dn.ToDatabase(recipient)
					if err := store.Record(ctx, id, typ, data); err != nil {
						errs = append(errs, err)
					}
				}
			}
		case "webhook":
			if wn, ok := n.(WebhookNotification); ok {
				url, payload := wn.ToWebhook(recipient)
				if url != "" {
					resp, err := httpclient.New().Do(ctx, "POST", url, payload)
					if err != nil {
						errs = append(errs, err)
					} else {
						_ = resp.Body.Close()
					}
				}
			}
		}
	}
	return errors.Join(errs...)
}

// Queue dispatches notification delivery onto Rapide's background goroutine worker pool.
func Queue(recipient Notifiable, n Notification) error {
	return queue.DispatchFunc(func(ctx context.Context) error {
		return Send(ctx, recipient, n)
	})
}
