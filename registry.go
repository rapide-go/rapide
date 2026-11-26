package rapide

import (
	"io/fs"
	"slices"
	"sync"

	"github.com/rapide-go/rapide/config"
	schedule "github.com/rapide-go/rapide/console/scheduling"
	"github.com/rapide-go/rapide/database/schema"
)

var (
	regMu         sync.RWMutex
	regBinders    []config.Binder
	regMigrations []schema.Migration
	regSchedules  []func(*schedule.Scheduler)
	regAssetsFS   fs.FS
	regAssetsDir  string
)

// RegisterConfig registers typed config binders loaded automatically by rapide.New().
// Call from `config/register.go`'s init() so bootstrap/app.go needs no manual wiring.
func RegisterConfig(binders ...config.Binder) {
	regMu.Lock()
	regBinders = append(regBinders, binders...)
	regMu.Unlock()
}

// RegisterMigrations registers database migrations loaded automatically by rapide.New().
// Call from `database/migrations/registry.go`'s init().
func RegisterMigrations(migs ...schema.Migration) {
	regMu.Lock()
	regMigrations = append(regMigrations, migs...)
	regMu.Unlock()
}

// RegisterSchedule registers a scheduled-task callback loaded automatically by rapide.New().
// Call from `routes/console.go`'s init().
func RegisterSchedule(fn func(*schedule.Scheduler)) {
	if fn == nil {
		return
	}
	regMu.Lock()
	regSchedules = append(regSchedules, fn)
	regMu.Unlock()
}

// RegisterAssets registers an embedded build filesystem loaded automatically by rapide.New().
// Call from `public/embed.go`'s init().
func RegisterAssets(fsys fs.FS, dir ...string) {
	regMu.Lock()
	regAssetsFS = fsys
	if len(dir) > 0 && dir[0] != "" {
		regAssetsDir = dir[0]
	} else {
		regAssetsDir = "build"
	}
	regMu.Unlock()
}

func snapshotRegistry() (
	binders []config.Binder,
	migs []schema.Migration,
	schedules []func(*schedule.Scheduler),
	assetsFS fs.FS,
	assetsDir string,
) {
	regMu.RLock()
	defer regMu.RUnlock()
	binders = slices.Clone(regBinders)
	migs = slices.Clone(regMigrations)
	schedules = slices.Clone(regSchedules)
	return binders, migs, schedules, regAssetsFS, regAssetsDir
}
