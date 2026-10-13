// Package aicatalog is the AI Assistant: ✨ Identify inside the core product form
// and intake (slot productAssists), and a catalogue Clean up page. The AI only
// suggests — a person confirms every write. Core never imports it; it attaches
// through generic hooks and its routes at /api/v1/x/aicatalog. Paste mode (copy
// the prompt to any chatbot, paste the reply back) is the main path.
package aicatalog

import (
	"io/fs"

	"karots-suite/internal/features/categories"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/aicatalog/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
	cats  *categories.Service
}

func (p *Plugin) Name() string                { return "AI Assistant" }
func (p *Plugin) Key() string                 { return "aicatalog" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "aicatalog" }

// dataGroup: the key never leaves the server — every dump and restore blanks it.
var dataGroup = plugin.DataGroup{Key: "aicatalog", Name: "AI Assistant",
	Tables: []string{"aicatalog_items", "aicatalog_optimize_runs", "aicatalog_settings"},
	Secret: []string{"aicatalog_settings.api_key"}}

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	p.cats = categories.NewService(reg.Core.DB)
	reg.AddDataGroup(dataGroup)
	p.registerHooks(reg)
	p.routes(reg)
}
