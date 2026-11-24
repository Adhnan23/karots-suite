package plugin

import "context"

// AddBackground runs f in its own goroutine once the real server is up (cmd/server, with the server's
// background context: f returns when ctx ends). A plugin's nightly or periodic work goes here (e.g. checking
// for a new catalogue version). Tests and the Demo Room's shops never start them: they call the work directly.
func (r *Registry) AddBackground(f func(ctx context.Context)) { backgrounds.add(f) }

var backgrounds hook[func(ctx context.Context)]

// Backgrounds are the registered background jobs, in registration order.
func Backgrounds() []func(ctx context.Context) { return backgrounds.all() }
