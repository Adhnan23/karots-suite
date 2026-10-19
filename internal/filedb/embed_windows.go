//go:build filemode && windows

package filedb

import "embed"

// zonky's Windows PostgreSQL needs the Visual C++ runtime, which a fresh Windows
// lacks (initdb then dies with 0xc0000135). Microsoft lets apps ship these three
// DLLs themselves; vcrt/ holds the signed x64 copies (14.44, from vc_redist):
//
//	msvcp140.dll        0f885b509a685d2bbfa652fed26b5fb31d88fbdab0a978c641d1c7b8aa460aa9
//	vcruntime140.dll    d5e4d9a3e835fa679450145d6a7d94e36573a509317111904d9b3712c30d9066
//	vcruntime140_1.dll  1f2d41c4aa5db0bc33ebf7b66d72943a817d7ce6cbe880502a9403823633093f
//
//go:embed vcrt/*.dll
var vcrtFS embed.FS

func init() { vcrt = &vcrtFS }
