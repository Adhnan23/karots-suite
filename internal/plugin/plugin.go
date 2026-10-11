// Package plugin is the compile-time extension framework. Plugins live under
// plugins/<name>/, register themselves with Register (usually from an init()),
// and the bootstrapper decides which are blank-imported into a given build. A
// plugin can add migrations, services, JSON routes (mounted at /api/v1/x) and
// data/behaviour hooks; its UI is a React half in web/src/plugins/<key> — all
// without the core importing any plugin.
//
// Import direction (no cycles): plugins and internal/app both import this
// package; this package imports the core feature services it exposes via Core,
// but never imports internal/api or internal/app. The route Mux lives here so
// the API layer can mount it without api ⇄ plugin importing each other.
package plugin

import "io/fs"

// Plugin is a compile-time extension.
type Plugin interface {
	// Name is a human label for logs and the bootstrapper.
	Name() string
	// Key is the stable id: the plugin.json key, the web/src/plugins/<key> folder,
	// the /api/v1/x/<key> routes and the /admin/x/<key> pages.
	Key() string
	// Migrations returns the plugin's embedded migration filesystem and a unique
	// goose version-table suffix (e.g. "alpha" → goose_db_version_alpha).
	// Return (nil, "") when the plugin has no migrations. The suffix keeps the
	// plugin's schema versioning separate from core, so enabling a plugin later
	// on a live database applies only its migrations — never a wipe.
	Migrations() (fs.FS, string)
	// Setup wires the plugin's routes, services and UI hooks onto the registry.
	// It runs after core routes are registered, so plugin routes override core.
	Setup(*Registry)
}

var registered []Plugin

// Register adds a plugin to the build. Safe to call from an init() in the
// plugin package; the bootstrapper controls which plugin packages are imported.
func Register(p Plugin) { registered = append(registered, p) }

// All returns the registered plugins in registration order.
func All() []Plugin { return registered }

// SetupAll runs every registered plugin's Setup against the registry. Called
// once from internal/app (startPlugins) before the Mux is mounted.
func SetupAll(r *Registry) {
	for _, p := range registered {
		// Tag every contribution p registers with its name, so per-profile
		// visibility (demo) can filter by plugin. Reset after, so any non-plugin
		// registration is unowned (always visible).
		current = p.Name()
		p.Setup(r)
		current = ""
	}
}
