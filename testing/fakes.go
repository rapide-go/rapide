package testkit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/mail"
	"github.com/rapide-go/rapide/support/manager"
)

// Clock provides controllable time for tests (Rapide Carbon::setTestNow / travel()).
type Clock struct {
	mu     sync.RWMutex
	frozen *time.Time
	offset time.Duration
}

func NewClock() *Clock { return &Clock{} }

func (c *Clock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.frozen != nil {
		return c.frozen.Add(c.offset)
	}
	return time.Now().Add(c.offset)
}

func (c *Clock) Freeze(at time.Time) {
	c.mu.Lock()
	c.frozen = &at
	c.offset = 0
	c.mu.Unlock()
}

func (c *Clock) Travel(d time.Duration) {
	c.mu.Lock()
	c.offset += d
	c.mu.Unlock()
}

func (c *Clock) Reset() {
	c.mu.Lock()
	c.frozen = nil
	c.offset = 0
	c.mu.Unlock()
}

// FakeMailer implements mail.Mailer and captures all sent messages in memory.
type FakeMailer struct {
	mu   sync.RWMutex
	Sent []mail.Message
}

var _ mail.Mailer = (*FakeMailer)(nil)

func (f *FakeMailer) Send(_ context.Context, msg mail.Message) error {
	f.mu.Lock()
	f.Sent = append(f.Sent, msg)
	f.mu.Unlock()
	return nil
}

func (f *FakeMailer) AssertSentCount(t testing.TB, want int) {
	t.Helper()
	f.mu.RLock()
	got := len(f.Sent)
	f.mu.RUnlock()
	if got != want {
		t.Fatalf("FakeMailer: expected %d sent email(s), got %d", want, got)
	}
}

func (f *FakeMailer) AssertNothingSent(t testing.TB) {
	t.Helper()
	f.AssertSentCount(t, 0)
}

// FakeMail swaps the global mail.Mailer and mail.Manager() in container.Default with a *FakeMailer
// (Rapide Mail::fake() equivalent) and restores previous bindings automatically on t.Cleanup.
func FakeMail(t testing.TB) *FakeMailer {
	t.Helper()
	fm := &FakeMailer{}

	prevMailer, hadMailer := container.TryUse[mail.Mailer]()
	prevMgr, hadMgr := container.TryUse[*manager.Manager[mail.Mailer]]()

	mgr := manager.New[mail.Mailer]()
	mgr.Set("fake", fm).SetDefault("fake")

	container.Register[mail.Mailer](fm)
	container.Register[*manager.Manager[mail.Mailer]](mgr)

	t.Cleanup(func() {
		if hadMailer == nil {
			container.Register[mail.Mailer](prevMailer)
		}
		if hadMgr == nil {
			container.Register[*manager.Manager[mail.Mailer]](prevMgr)
		}
	})
	return fm
}
