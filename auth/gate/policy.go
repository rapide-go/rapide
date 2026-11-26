package gate

// Policy is a generic fluent resource authorization policy for user U and resource R.
// Implement whichever optional action interfaces your policy needs, or pass a map of abilities
// via RegisterPolicy.
type ViewPolicy[U any, R any] interface {
	View(user U, resource R) bool
}

type UpdatePolicy[U any, R any] interface {
	Update(user U, resource R) bool
}

type DeletePolicy[U any, R any] interface {
	Delete(user U, resource R) bool
}

type CreatePolicy[U any] interface {
	Create(user U) bool
}

// RegisterPolicy inspects policy for ViewPolicy/UpdatePolicy/DeletePolicy/CreatePolicy
// implementations and binds them as typed gates under `<prefix>.view`, `<prefix>.update`,
// `<prefix>.delete`, and `<prefix>.create`.
func RegisterPolicy[U any, R any](prefix string, policy any) {
	if p, ok := policy.(ViewPolicy[U, R]); ok {
		Define(prefix+".view", p.View)
	}
	if p, ok := policy.(UpdatePolicy[U, R]); ok {
		Define(prefix+".update", p.Update)
	}
	if p, ok := policy.(DeletePolicy[U, R]); ok {
		Define(prefix+".delete", p.Delete)
	}
	if p, ok := policy.(CreatePolicy[U]); ok {
		Define(prefix+".create", func(u U, _ R) bool {
			return p.Create(u)
		})
	}
}
