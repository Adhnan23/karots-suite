package aicatalog

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

// fakeGroups is a grouper with memory, as Alternatives would behave.
type fakeGroups struct {
	groups []plugin.ProductGroupInfo
	next   int64
}

func (f *fakeGroups) hook() plugin.ProductGrouper {
	return plugin.ProductGrouper{
		Groups: func(context.Context) ([]plugin.ProductGroupInfo, error) { return f.groups, nil },
		Create: func(_ context.Context, name string, ids []int64) (int64, error) {
			f.next++
			f.groups = append(f.groups, plugin.ProductGroupInfo{ID: f.next, Name: name, Tiers: []plugin.ProductGroupTier{{ID: f.next * 10}}, Members: ids})
			return f.next, nil
		},
		AddTo: func(_ context.Context, tier, pid int64) error {
			for i := range f.groups {
				for _, t := range f.groups[i].Tiers {
					if t.ID == tier {
						f.groups[i].Members = append(f.groups[i].Members, pid)
						return nil
					}
				}
			}
			return fmt.Errorf("no tier %d", tier)
		},
		Delete: func(_ context.Context, id int64) error {
			for i := range f.groups {
				if f.groups[i].ID == id {
					f.groups = append(f.groups[:i], f.groups[i+1:]...)
					return nil
				}
			}
			return nil
		},
		Remove: func(_ context.Context, pid int64) error {
			for i := range f.groups {
				for j, m := range f.groups[i].Members {
					if m == pid {
						f.groups[i].Members = append(f.groups[i].Members[:j], f.groups[i].Members[j+1:]...)
						return nil
					}
				}
			}
			return nil
		},
	}
}

func groupEnv(t *testing.T, name string) (*Plugin, *fakeGroups, int64) {
	t.Helper()
	db := freshDB(t, name)
	p, _ := testAPI(t, db)
	f := &fakeGroups{}
	p.jobs, p.groupers = []Job{groupsJob()}, func() []plugin.ProductGrouper { return []plugin.ProductGrouper{f.hook()} }
	return p, f, cat(t, db, "Medicine", nil)
}

// Both chatbots' real replies to the trap list: Panadol Extra, the syrup and other strengths left out.
func TestGroupsRealReply(t *testing.T) {
	p, _, _ := groupEnv(t, "aicatalog_groups_real")
	var b Batch
	for i, n := range []string{"Panadol 500mg Tablets", "Cetamol 500mg Tablets", "Panadol Extra Tablets", "Panadol 120mg/5ml Syrup", "Paracetamol 1g Tablets", "Amoxil 250mg Capsules", "Amoxicillin 250mg Capsules", "Amoxicillin 500mg Capsules"} {
		b.Products = append(b.Products, JobProduct{ID: int64(i + 1), Name: n})
	}
	reply := `{"groups":[{"name":"Paracetamol 500mg Tablets","product_ids":[1,2],"reason":"Both contain paracetamol 500mg in tablet form."},{"name":"Amoxicillin 250mg Capsules","product_ids":[6,7],"reason":"Both contain amoxicillin 250mg in capsule form."}]}`
	rows, err := groupsParse(context.Background(), p, b, reply)
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	if r := rows[0]; r.Extra["members"] != "1,2" || r.Ticked || r.Reason == "" || !strings.Contains(r.New, "Cetamol") {
		t.Fatalf("%+v", r)
	}
}

func TestGroupsDropBad(t *testing.T) {
	p, _, _ := groupEnv(t, "aicatalog_groups_bad")
	b := Batch{Products: []JobProduct{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}, Context: []JobProduct{{ID: 3, Name: "C"}, {ID: 4, Name: "D"}}}
	rows, err := groupsParse(context.Background(), p, b, `{"groups":[
		{"name":"alone","product_ids":[1]},
		{"name":"foreign","product_ids":[1,99]},
		{"name":"","product_ids":[1,2]},
		{"name":"context only","product_ids":[3,4]},
		{"name":"fine","product_ids":[2,3,99]},
		{"name":"reuses a taken product","product_ids":[2,4]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Extra["name"] != "fine" || rows[0].Extra["members"] != "2,3" {
		t.Fatalf("%+v", rows)
	}
}

func TestGroupSelectKeepsTwinsTogether(t *testing.T) {
	p, f, c := groupEnv(t, "aicatalog_groups_select")
	db := p.store.db
	other := cat(t, db, "Cleaning", nil)
	a := prod(t, db, "Panadol 500mg", c)
	b := prod(t, db, "Cetamol 500mg", c)
	grouped := prod(t, db, "Grouped 1g", c)
	vague := prod(t, db, "Mystery item", c)
	h := prod(t, db, "Harpic 500ml", other)
	f.groups = []plugin.ProductGroupInfo{{ID: 1, Name: "Old", Tiers: []plugin.ProductGroupTier{{ID: 10}}, Members: []int64{grouped}}}
	got, total, err := groupsSelect(context.Background(), p, JobRequest{Size: 1, CategoryID: c})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Products) != 1 || got.Products[0].ID != b || total != 2 {
		t.Fatalf("alphabetical, one to place: %+v %d", got.Products, total)
	}
	if len(got.Context) != 1 || got.Context[0].ID != a {
		t.Fatalf("its twin comes as context: %+v", got.Context)
	}
	_, user, err := groupsPromptFor(context.Background(), p, got)
	if err != nil || !strings.Contains(user, "Panadol 500mg") || strings.Contains(user, "Harpic") || strings.Contains(user, "Mystery") || !strings.Contains(user, "Old") {
		t.Fatalf("%q %v", user, err)
	}
	_ = vague
	_ = h
	if s, _, _ := groupsPromptFor(context.Background(), p, got); !strings.Contains(s, "same strength") || !strings.Contains(s, "size, rating, fit") {
		t.Fatalf("prompt rules: %q", s)
	}
}

func TestJoinMultiTierNeedsTier(t *testing.T) {
	p, f, c := groupEnv(t, "aicatalog_groups_tiers")
	ctx := context.Background()
	a := prod(t, p.store.db, "Panadol 500mg", c)
	s := prod(t, p.store.db, "Cetamol 500mg", c)
	f.groups = []plugin.ProductGroupInfo{
		{ID: 5, Name: "Tiered", Tiers: []plugin.ProductGroupTier{{ID: 50, Name: "Brand"}, {ID: 51, Name: "Generic"}}},
		{ID: 6, Name: "Plain", Tiers: []plugin.ProductGroupTier{{ID: 60}}},
	}
	b := Batch{Products: []JobProduct{{ID: a, Name: "Panadol 500mg"}, {ID: s, Name: "Cetamol 500mg"}}}
	rows, err := groupsParse(ctx, p, b, fmt.Sprintf(`{"joins":[{"product_id":%d,"group_id":5,"reason":"x"},{"product_id":%d,"group_id":6,"reason":"y"},{"product_id":999,"group_id":6},{"product_id":%d,"group_id":77}]}`, a, s, a))
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	if rows[0].Needs == "" || len(rows[0].Choices) != 2 || rows[0].Ticked || rows[0].Extra["tier"] != "" {
		t.Fatalf("the person picks the tier: %+v", rows[0])
	}
	if rows[1].Needs != "" || rows[1].Extra["tier"] != "60" {
		t.Fatalf("one tier fills itself: %+v", rows[1])
	}
	res, err := p.applyJob(ctx, p.job("groups"), JobApply{Rows: rows[:1], Asked: []int64{a}}, 0)
	if err != nil || res.Applied != 0 || len(res.Skipped) != 1 {
		t.Fatalf("no tier, no join: %+v %v", res, err)
	}
	rows[0].Extra["tier"] = "51"
	if res, err := p.applyJob(ctx, p.job("groups"), JobApply{Rows: rows, Asked: []int64{a, s}}, 0); err != nil || res.Applied != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(f.groups[0].Members) != 1 || len(f.groups[1].Members) != 1 {
		t.Fatalf("%+v", f.groups)
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.groups[0].Members) != 0 || len(f.groups[1].Members) != 0 {
		t.Fatalf("undo: %+v", f.groups)
	}
}

func TestGroupApplyUndo(t *testing.T) {
	p, f, c := groupEnv(t, "aicatalog_groups_undo")
	ctx := context.Background()
	a, b := prod(t, p.store.db, "A 5", c), prod(t, p.store.db, "B 5", c)
	row := JobRow{ProductID: a, Kind: "group", Label: "New group", Extra: map[string]string{"name": "Twins", "members": fmt.Sprintf("%d,%d", a, b)}}
	if res, err := p.applyJob(ctx, p.job("groups"), JobApply{Rows: []JobRow{row}, Asked: []int64{a}, Suggested: []int64{a}}, 0); err != nil || res.Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(f.groups) != 1 {
		t.Fatalf("%+v", f.groups)
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.groups) != 0 {
		t.Fatalf("undo: %+v", f.groups)
	}
	// Someone added to it since: it stays.
	if _, err := p.applyJob(ctx, p.job("groups"), JobApply{Rows: []JobRow{row}, Asked: []int64{a}}, 0); err != nil {
		t.Fatal(err)
	}
	f.groups[0].Members = append(f.groups[0].Members, 999)
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.groups) != 1 {
		t.Fatal("undo deleted a group someone had added to")
	}
}

func TestUnavailableWithoutGrouper(t *testing.T) {
	db := freshDB(t, "aicatalog_groups_none")
	p, do := testAPI(t, db)
	p.jobs, p.groupers = []Job{groupsJob()}, func() []plugin.ProductGrouper { return nil }
	if list := decode[[]JobInfo](t, do("GET", "/jobs", "")); len(list) != 0 {
		t.Fatalf("%+v", list)
	}
}
