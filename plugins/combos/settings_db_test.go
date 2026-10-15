package combos

import (
	"context"
	"testing"
)

// Auto apply is off until the shop turns it on; a missing row (a full restore of
// an older backup) reads as off, and saving puts the row back.
func TestSettingsAutoApply(t *testing.T) {
	st, db, _ := testStore(t)
	ctx := context.Background()
	t.Cleanup(func() { st.SaveSettings(ctx, SettingsDTO{}) }) //nolint:errcheck
	if err := st.SaveSettings(ctx, SettingsDTO{AutoApply: true}); err != nil {
		t.Fatal(err)
	}
	if v, err := st.Settings(ctx); err != nil || !v.AutoApply {
		t.Fatalf("saved on, read %+v %v", v, err)
	}
	db.MustExec(`DELETE FROM combos_settings`)
	if v, err := st.Settings(ctx); err != nil || v.AutoApply {
		t.Fatalf("no row should read off, got %+v %v", v, err)
	}
	if err := st.SaveSettings(ctx, SettingsDTO{AutoApply: true}); err != nil {
		t.Fatal(err)
	}
	if v, _ := st.Settings(ctx); !v.AutoApply {
		t.Fatal("save should restore the row")
	}
}
