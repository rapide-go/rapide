package inertia

import (
	igo "github.com/rapide-go/inertia"
	"github.com/rapide-go/rapide/broadcasting"
	"github.com/rapide-go/rapide/database/query"
)

// ScrollPage wraps Rapide's generic orm/query.Page[T] so it satisfies inertia-go's ProvidesScrollMetadata.
type ScrollPage[T any] struct {
	query.Page[T]
	pageName string
}

// ScrollMetadata returns Inertia infinite scroll metadata derived from query.Page[T].
func (p ScrollPage[T]) ScrollMetadata() igo.ScrollMetadata {
	return igo.PagesScroll(p.pageName, p.CurrentPage, p.LastPage)
}

// ScrollFromPage turns an Rapide orm/query.Page[T] into an Inertia v3 <InfiniteScroll> *Prop in one call.
func ScrollFromPage[T any](page query.Page[T], pageName ...string) *igo.Prop {
	pn := "page"
	if len(pageName) > 0 && pageName[0] != "" {
		pn = pageName[0]
	}
	return igo.Scroll(ScrollPage[T]{Page: page, pageName: pn})
}

// BroadcastTo publishes an event with optional live Inertia prop updates directly onto
// Rapide's in-process SSE broadcast Hub — zero external Pusher/Reverb server required.
func BroadcastTo(channel, event string, payload map[string]any, liveProps map[string]any) error {
	if len(liveProps) > 0 {
		payload = igo.BroadcastProps(payload, liveProps)
	}
	return broadcast.Publish(channel, event, payload)
}
