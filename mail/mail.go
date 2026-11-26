package mail

import (
	"context"
	"strings"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/queue"
	"github.com/rapide-go/rapide/support/manager"
	"github.com/rapide-go/rapide/support/option"
)

// Mailer is the contract implemented by mail drivers (SMTP, Log, Resend, Memory, etc.).
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// Manager returns the multi-mailer *manager.Manager[Mailer] bound in the container.
func Manager() *manager.Manager[Mailer] {
	return container.Use[*manager.Manager[Mailer]]()
}

// Mail is the facade accessor returning the default Mailer.
func Mail() Mailer {
	if m, err := container.TryUse[*manager.Manager[Mailer]](); err == nil && m != nil {
		return m.Default()
	}
	return container.Use[Mailer]()
}

// UseMailer resolves a specific named mailer (e.g. mail.UseMailer("smtp"), mail.UseMailer("resend")).
func UseMailer(name string) Mailer {
	return Manager().Must(name)
}

// Extend registers a custom named mailer factory.
func Extend(name string, fn manager.Factory[Mailer]) {
	Manager().Extend(name, fn)
}

// Send delivers msg immediately through the default Mailer.
func Send(ctx context.Context, msg Message) error {
	return Mail().Send(ctx, msg)
}

// SendMailable builds a typed Mailable and delivers it immediately.
func SendMailable(ctx context.Context, m Mailable) error {
	msg, err := m.Build()
	if err != nil {
		return err
	}
	return Send(ctx, msg)
}

// Queue dispatches msg to Rapide's background goroutine queue for async delivery.
func Queue(msg Message) error {
	return queue.DispatchFunc(func(ctx context.Context) error {
		return Mail().Send(ctx, msg)
	})
}

// QueueMailable builds a typed Mailable and dispatches it asynchronously onto the queue.
func QueueMailable(m Mailable) error {
	msg, err := m.Build()
	if err != nil {
		return err
	}
	return Queue(msg)
}

// LogDriver logs outbound emails instead of sending them (ideal for local dev).
type LogDriver struct{}

func (LogDriver) Send(_ context.Context, msg Message) error {
	applog.Info("mail.log",
		"from", msg.From,
		"to", strings.Join(msg.To, ","),
		"subject", msg.Subject,
		"attachments", len(msg.Attachments),
	)
	return nil
}

// Provider binds a multi-mailer manager.Manager[Mailer] into the application container.
type Provider struct {
	foundation.BaseProvider
	Default string
	Driver  Mailer
	Mailers map[string]manager.Factory[Mailer]
	Options []option.Option[manager.Manager[Mailer]]
}

func (p Provider) Register(app *foundation.App) error {
	m := manager.New[Mailer](
		manager.WithInstance[Mailer]("log", LogDriver{}),
		manager.WithFactory[Mailer]("memory", func() (Mailer, error) {
			return NewMemory(), nil
		}),
	)

	if p.Driver != nil {
		m.Set("default", p.Driver)
		m.SetDefault("default")
	}
	for name, fn := range p.Mailers {
		m.Register(name, fn)
	}
	option.Apply(m, p.Options...)

	if p.Default != "" {
		m.SetDefault(p.Default)
	} else if p.Driver == nil {
		m.SetDefault(config.Env("MAIL_MAILER", "log"))
	}

	container.Bind[*manager.Manager[Mailer]](app.Container, m)
	container.Bind[Mailer](app.Container, m.Default())

	app.OnShutdown(func(ctx context.Context) error {
		return m.CloseAll()
	})
	return nil
}
