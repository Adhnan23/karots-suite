package plugin

import "crypto/tls"

// Listener is an extra HTTPS address a plugin serves the whole app on, with a
// certificate it supplies (e.g. for phones on the shop's wifi). The normal
// port is unchanged.
type Listener struct {
	Key  string // the plugin's key, for logs
	Addr string // ":8443"
	TLS  *tls.Config
}

var listeners hook[Listener]

func (r *Registry) AddListener(l Listener) { listeners.add(l) }

func Listeners() []Listener { return listeners.all() }
