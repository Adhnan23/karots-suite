package plugin

// hook is a registration-ordered list of plugin contributions.
type hook[T any] struct{ items []T }

func (h *hook[T]) add(it T) { h.items = append(h.items, it) }

func (h *hook[T]) all() []T { return h.items }
