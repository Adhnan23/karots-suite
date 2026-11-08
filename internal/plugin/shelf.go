package plugin

import "slices"

// Karots Shelf, the shop staff's product app, reaches only the admin routes it
// calls. A plugin whose admin route the app needs adds it here; the web layer's
// guard reads the list (core never names a plugin's route).
var shelfRoutes []string

// AllowShelf lets Karots Shelf call one of this plugin's admin routes, path as
// registered on reg.Admin() (e.g. "/myplugin/fields").
func (r *Registry) AllowShelf(method, path string) { allowShelf(method, path) }

func allowShelf(method, path string) {
	shelfRoutes = append(shelfRoutes, method+" /api/v1/x"+path)
}

// ShelfRoutes is every plugin route Karots Shelf may call, as "METHOD /api/v1/x/…".
func ShelfRoutes() []string { return shelfRoutes }

// AllowShelfForTest adds one route and returns what puts the list back (tests only).
func AllowShelfForTest(method, path string) func() {
	was := slices.Clone(shelfRoutes)
	allowShelf(method, path)
	return func() { shelfRoutes = was }
}
