package plugin

// Per-profile plugin visibility. Plugins are compile-time enabled and register
// their contributions (nav, cards, tenders, product hooks…) once at SetupAll.
// The demo, however, wants each profile to expose only its own plugins — so
// every contribution is tagged with the plugin that registered it (owner), and
// the getters filter by an injectable predicate.
//
// The real server never sets a filter: visible == nil means "show everything",
// and hook.all() then returns the registration-order slice unchanged — so this
// is completely behaviour-neutral outside the demo.

// current is the plugin whose Setup is running, set by SetupAll so each hook.add
// records the right owner. Empty outside SetupAll (e.g. core registrations) —
// an empty owner is always visible.
var current string

// visible reports whether a given plugin's contributions should show. nil = all.
var visible func(pluginName string) bool

// SetVisibilityFilter installs the predicate (demo only). Passing nil restores
// "show everything". Safe to call at runtime (e.g. on a demo profile switch).
func SetVisibilityFilter(fn func(pluginName string) bool) { visible = fn }

// hook is a registration-ordered list of contributions, each tagged with its
// owning plugin so all() can filter by the active visibility predicate.
type hook[T any] struct {
	items  []T
	owners []string
}

func (h *hook[T]) add(owner string, it T) {
	h.items = append(h.items, it)
	h.owners = append(h.owners, owner)
}

// all returns the visible contributions in registration order. With no filter,
// or for an unowned (core) contribution, everything shows.
func (h *hook[T]) all() []T {
	if visible == nil {
		return h.items
	}
	out := make([]T, 0, len(h.items))
	for i, it := range h.items {
		if h.owners[i] == "" || visible(h.owners[i]) {
			out = append(out, it)
		}
	}
	return out
}
