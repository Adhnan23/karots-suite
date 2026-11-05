package plugin

import "context"

// OfflineData is what a plugin's till screens read, as one piece of JSON-ready
// data, for a client that keeps a copy to work from when the server can't be
// reached. Build runs often, with no person signed in: keep it to what selling
// needs, and leave out cost and profit.
type OfflineData struct {
	Key   string // the plugin's key
	Build func(ctx context.Context) (any, error)
}

var offlineData hook[OfflineData]

func (r *Registry) AddOfflineData(d OfflineData) { offlineData.add(d) }

// OfflineDataAll lists what the compiled-in plugins declared.
func OfflineDataAll() []OfflineData { return offlineData.all() }

// ResetOfflineDataForTest forgets what tests declared.
func ResetOfflineDataForTest() { offlineData = hook[OfflineData]{} }
