// Package templates holds each built-in shop's past — a year of trade built once
// by `make demo` and kept as a suite backup — embedded in the demo binary.
package templates

import "embed"

//go:embed all:files
var files embed.FS

// Get is the backup for shop key, if this build carries one.
func Get(key string) ([]byte, bool) {
	b, err := files.ReadFile("files/" + key + ".ksb")
	return b, err == nil && len(b) > 0
}
