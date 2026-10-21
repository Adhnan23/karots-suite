package backup

import (
	"reflect"
	"testing"
)

func TestTablesToClear(t *testing.T) {
	existing := []string{"products", "sales", "alpha", "goose_db_version"}
	archive := []string{"products", "sales"} // backup had no combos

	// ReplaceAll: clear everything current (incl. combos), never goose.
	got := tablesToClear(existing, archive, RestoreReplaceAll)
	want := []string{"products", "sales", "alpha"}
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

// A backup from before smart reorder levels has no reorder_auto: a product with
// a reorder level typed in restores Fixed; one left at 0 (never set) becomes Auto.
func TestFillLegacyColumns(t *testing.T) {
	one, two, typed, unset := "1", "2", "20", "0"
	old := Table{Name: "products", Columns: []string{"id", "reorder_level"}, Rows: [][]*string{{&one, &typed}, {&two, &unset}}}
	schema := []string{"id", "reorder_level", "reorder_auto"}
	got := fillLegacyColumns(old, schema)
	if !reflect.DeepEqual(got.Columns, schema) || *got.Rows[0][2] != "false" || *got.Rows[1][2] != "true" {
		t.Fatalf("old products = %v, want typed 20 Fixed and unset 0 Auto", got.Columns)
	}
	if got := fillLegacyColumns(old, []string{"id", "reorder_level"}); len(got.Columns) != 2 {
		t.Fatalf("schema without the column got it: %v", got.Columns)
	}
	yes := "true"
	cur := Table{Name: "products", Columns: []string{"id", "reorder_level", "reorder_auto"}, Rows: [][]*string{{&one, &typed, &yes}}}
	if got := fillLegacyColumns(cur, schema); *got.Rows[0][2] != "true" || len(got.Columns) != 3 {
		t.Fatalf("current products changed: %v", got.Columns)
	}
}
