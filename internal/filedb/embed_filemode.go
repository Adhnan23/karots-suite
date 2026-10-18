//go:build filemode

package filedb

import _ "embed"

// The Shop Builder drops the target's bundle here before a `-tags filemode` build
// and removes it after (bundle/ is gitignored).
//
//go:embed bundle/postgres.txz
var embedded []byte

func init() { bundled = embedded }
