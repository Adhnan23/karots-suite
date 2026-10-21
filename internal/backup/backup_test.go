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

// A backup from before smart reorder levels has no reorder_auto: its products
// restore Fixed (the column's default is Auto, for new products only).
func TestFillLegacyColumns(t *testing.T) {
	one := "1"
	old := Table{Name: "products", Columns: []string{"id"}, Rows: [][]*string{{&one}}}
	schema := []string{"id", "reorder_auto"}
	got := fillLegacyColumns(old, schema)
	if !reflect.DeepEqual(got.Columns, []string{"id", "reorder_auto"}) || *got.Rows[0][1] != "false" {
		t.Fatalf("old products = %v %v, want reorder_auto false", got.Columns, got.Rows)
	}
	yes := "true"
	cur := Table{Name: "products", Columns: []string{"id", "reorder_auto"}, Rows: [][]*string{{&one, &yes}}}
	if got := fillLegacyColumns(old, []string{"id"}); len(got.Columns) != 1 {
		t.Fatalf("schema without the column got it: %v", got.Columns)
	}
	if got := fillLegacyColumns(cur, schema); *got.Rows[0][1] != "true" || len(got.Columns) != 2 {
		t.Fatalf("current products changed: %v", got.Columns)
	}
}
