package factory

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/rapide-go/rapide/database/query"
)

// Factory builds and persists typed model instances T with optional state overrides
// .
type Factory[T any] struct {
	def    func() T
	states []func(*T)
	count  int
}

// Define creates a reusable Factory[T] from a base definition function.
func Define[T any](def func() T) *Factory[T] {
	return &Factory[T]{def: def, count: 1}
}

func (f *Factory[T]) clone() *Factory[T] {
	return &Factory[T]{
		def:    f.def,
		states: slices.Clone(f.states),
		count:  f.count,
	}
}

// Count sets how many models MakeMany / CreateMany should produce.
func (f *Factory[T]) Count(n int) *Factory[T] {
	cp := f.clone()
	if n < 0 {
		n = 0
	}
	cp.count = n
	return cp
}

// State appends a mutator applied after the base definition (e.g. Admin(), Unverified()).
func (f *Factory[T]) State(fn func(*T)) *Factory[T] {
	cp := f.clone()
	cp.states = append(cp.states, fn)
	return cp
}

// Make builds a single in-memory instance of T without touching the database.
func (f *Factory[T]) Make() T {
	item := f.def()
	for _, st := range f.states {
		st(&item)
	}
	return item
}

// MakeMany builds Count() in-memory instances of T.
func (f *Factory[T]) MakeMany() []T {
	out := make([]T, f.count)
	for i := range out {
		out[i] = f.Make()
	}
	return out
}

// Create inserts a single T into the default database via query.Query[T]().Create.
func (f *Factory[T]) Create() (T, error) {
	return query.Query[T]().Create(f.Make())
}

// CreateMany inserts Count() instances of T into the default database.
func (f *Factory[T]) CreateMany() ([]T, error) {
	out := make([]T, 0, f.count)
	for range f.count {
		created, err := f.Create()
		if err != nil {
			return out, err
		}
		out = append(out, created)
	}
	return out, nil
}

// --- Zero-dependency fake data helpers ---

var (
	seqCounter atomic.Int64
	firstNames = []string{"Ada", "Grace", "Alan", "Linus", "Ken", "Rob", "Barbara", "Margaret", "Donald", "Edsger"}
	lastNames  = []string{"Lovelace", "Hopper", "Turing", "Torvalds", "Thompson", "Pike", "Liskov", "Hamilton", "Knuth", "Dijkstra"}
	loremWords = []string{"rapide", "swift", "generic", "binary", "stream", "route", "model", "cache", "event", "queue", "worker", "craft"}
)

// Sequence returns a monotonically increasing integer for unique test values.
func Sequence() int64 { return seqCounter.Add(1) }

// Name returns a random full name.
func Name() string {
	return pick(firstNames) + " " + pick(lastNames)
}

// Email returns a unique synthetic email address.
func Email() string {
	n := Sequence()
	return fmt.Sprintf("%s.%d@example.test", strings.ToLower(pick(firstNames)), n)
}

// Slug returns a URL-friendly random slug with words count.
func Slug(words int) string {
	if words <= 0 {
		words = 3
	}
	parts := make([]string, words+1)
	for i := range words {
		parts[i] = pick(loremWords)
	}
	parts[words] = fmt.Sprintf("%d", Sequence())
	return strings.Join(parts, "-")
}

// Sentence returns a capitalized sentence with words count.
func Sentence(words int) string {
	if words <= 0 {
		words = 6
	}
	parts := make([]string, words)
	for i := range words {
		parts[i] = pick(loremWords)
	}
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ") + "."
}

// UUID returns a random RFC 4122 v4 UUID string.
func UUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// IntRange returns a cryptographically random int in [minVal, maxVal].
func IntRange(minVal, maxVal int) int {
	if maxVal <= minVal {
		return minVal
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxVal-minVal+1)))
	if err != nil {
		return minVal
	}
	return minVal + int(n.Int64())
}

func pick(list []string) string {
	return list[IntRange(0, len(list)-1)]
}
