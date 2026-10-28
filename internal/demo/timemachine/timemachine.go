// Package timemachine holds the time-machine drawing both building screens
// share (the Demo Room's pages import timemachine.js directly; the Shop Builder
// serves this copy).
package timemachine

import _ "embed"

//go:embed timemachine.js
var JS []byte
