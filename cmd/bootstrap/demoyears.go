package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The shop years take minutes to build, so make demo keeps the last ones in the
// user cache under a stamp of everything that shapes their data, and reuses
// them while that is unchanged (-fresh builds them anyway). Opening a shop moves
// its year up to today, so an older year is as good as a new one.

// yearRoots is what the years are made from: the schema, the services and API
// the generator drives, the generator and shops, and the child that runs them.
// Too much here only costs a rebuild; too little ships a stale year.
var yearRoots = []string{"go.mod", "go.sum", "migrations", "plugins", "internal",
	"cmd/demo/child.go", "cmd/demo/templates.go"}

// yearSkips are parts of internal that never touch a shop's data.
var yearSkips = []string{"internal/demo/printer", "internal/demo/ui", "internal/demo/templates",
	"internal/demo/brand.go", "internal/spa", "internal/update", "internal/shopconf", "internal/filedb"}

// yearsStamp hashes every non-test .go / .sql / go.mod / go.sum file under the roots.
func yearsStamp(root string) (string, error) {
	h := sha256.New()
	for _, r := range yearRoots {
		err := filepath.WalkDir(filepath.Join(root, r), func(p string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			for _, s := range yearSkips {
				if rel == s || strings.HasPrefix(rel, s+"/") {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			name := d.Name()
			data := strings.HasSuffix(name, ".sql") || name == "go.mod" || name == "go.sum" ||
				strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
			if d.IsDir() || !data {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			io.WriteString(h, rel+"\x00")
			h.Write(b)
			h.Write([]byte{0})
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// takeYears copies the cached years for stamp into tpl; false if any is missing.
func takeYears(cache, stamp, tpl string, keys []string) bool {
	for _, k := range keys {
		if _, err := os.Stat(filepath.Join(cache, stamp, k+".ksb")); err != nil {
			return false
		}
	}
	for _, k := range keys {
		if copyFile(filepath.Join(cache, stamp, k+".ksb"), filepath.Join(tpl, k+".ksb")) != nil {
			return false
		}
	}
	return true
}

// keepYears caches tpl's years under stamp, dropping any older stamp's.
func keepYears(cache, stamp, tpl string, keys []string) error {
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Join(cache, stamp), 0o755); err != nil {
		return err
	}
	for _, k := range keys {
		if err := copyFile(filepath.Join(tpl, k+".ksb"), filepath.Join(cache, stamp, k+".ksb")); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}
