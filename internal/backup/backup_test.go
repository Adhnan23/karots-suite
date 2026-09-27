package backup

import (
	"reflect"
	"testing"
)

func TestTablesToClear(t *testing.T) {
	existing := []string{"products", "sales", "combos", "goose_db_version"}
	archive := []string{"products", "sales"} // backup had no combos

	// ReplaceAll: clear everything current (incl. combos), never goose.
	got := tablesToClear(existing, archive, RestoreReplaceAll)
	want := []string{"products", "sales", "combos"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReplaceAll = %v, want %v", got, want)
	}

	// Merge: only tables the archive carries (combos left untouched).
	got = tablesToClear(existing, archive, RestoreMerge)
	want = []string{"products", "sales"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge = %v, want %v", got, want)
	}
}
