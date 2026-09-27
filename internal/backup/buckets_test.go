package backup

import (
	"reflect"
	"testing"

	"karots-suite/internal/plugin"
)

func TestBuckets(t *testing.T) {
	all := []string{"products", "sales", "combos", "combo_items", "recharge_devices", "goose_db_version"}
	groups := []plugin.DataGroup{
		{Key: "combos", Tables: []string{"combos", "combo_items"}},
		{Key: "recharge", Tables: []string{"recharge_devices"}},
	}

	// Core = everything minus goose minus plugin tables.
	if got, want := CoreTables(all, groups), []string{"products", "sales"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CoreTables = %v, want %v", got, want)
	}

	// Keep recharge only -> exclude the combos group's tables.
	got := ExcludeFor(groups, []string{"recharge"})
	want := []string{"combos", "combo_items"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExcludeFor = %v, want %v", got, want)
	}
}

func TestZeroColumns(t *testing.T) {
	cols := []string{"id", "name", "outstanding_balance", "opening_balance"}
	five, ten, twenty := "5", "10", "20"
	one := "1"
	rows := [][]*string{{&one, nil, &five, &ten}, {&one, nil, &twenty, nil}}
	zeroColumns(cols, rows, []string{"outstanding_balance", "opening_balance"})
	for r := range rows {
		if *rows[r][2] != "0" || *rows[r][3] != "0" {
			t.Fatalf("row %d balances not zeroed: %v", r, rows[r])
		}
	}
	if *rows[0][0] != "1" { // untouched column preserved
		t.Fatalf("id column wrongly modified")
	}
}

func TestSetupOnlyExclude(t *testing.T) {
	core := []string{"products", "categories", "sales", "cash_movements", "settings"}
	got := SetupOnlyExclude(core)
	want := []string{"sales", "cash_movements"} // catalogue/config kept; history dropped
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetupOnlyExclude = %v, want %v", got, want)
	}
}
