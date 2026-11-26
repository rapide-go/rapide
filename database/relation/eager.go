package relation

import "github.com/rapide-go/rapide/database/query"

func uniqueKeys[T any, K comparable](items []T, keyFn func(T) K) []any {
	var zeroK K
	seen := make(map[K]bool, len(items))
	keys := make([]any, 0, len(items))
	for _, it := range items {
		k := keyFn(it)
		if k == zeroK || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	return keys
}

// LoadBelongsTo eager-loads parent models for a slice of children in a single WHERE IN query,
// eliminating N+1 queries (Rapide Post::with('author')->get() equivalent).
func LoadBelongsTo[Child, Parent any, K comparable](
	children []Child,
	fk func(Child) K,
	ownerKey string,
	pk func(Parent) K,
	assign func(*Child, Parent),
) error {
	keys := uniqueKeys(children, fk)
	if len(keys) == 0 {
		return nil
	}
	if ownerKey == "" {
		ownerKey = "id"
	}

	parents, err := query.Query[Parent]().WhereIn(ownerKey, keys).Get()
	if err != nil {
		return err
	}

	byKey := make(map[K]Parent, len(parents))
	for _, p := range parents {
		byKey[pk(p)] = p
	}
	for i := range children {
		if p, ok := byKey[fk(children[i])]; ok {
			assign(&children[i], p)
		}
	}
	return nil
}

// LoadHasMany eager-loads child slices for a slice of parents in a single WHERE IN query,
// eliminating N+1 queries (Rapide User::with('posts')->get() equivalent).
func LoadHasMany[Parent, Child any, K comparable](
	parents []Parent,
	pk func(Parent) K,
	foreignKey string,
	fk func(Child) K,
	assign func(*Parent, []Child),
) error {
	keys := uniqueKeys(parents, pk)
	if len(keys) == 0 {
		return nil
	}

	children, err := query.Query[Child]().WhereIn(foreignKey, keys).Get()
	if err != nil {
		return err
	}

	grouped := make(map[K][]Child, len(parents))
	for _, ch := range children {
		k := fk(ch)
		grouped[k] = append(grouped[k], ch)
	}
	for i := range parents {
		assign(&parents[i], grouped[pk(parents[i])])
	}
	return nil
}

// CountHasMany populates child counts for a slice of parents in a single query
// (Rapide User::withCount('posts')->get() equivalent).
func CountHasMany[Parent, Child any, K comparable](
	parents []Parent,
	pk func(Parent) K,
	foreignKey string,
	fk func(Child) K,
	assign func(*Parent, int),
) error {
	return LoadHasMany(parents, pk, foreignKey, fk, func(p *Parent, items []Child) {
		assign(p, len(items))
	})
}
