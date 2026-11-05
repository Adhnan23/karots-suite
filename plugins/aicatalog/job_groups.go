package aicatalog

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

const groupsPrompt = `Find products of a shop that can replace each other, so a customer can be offered one when the other is out.
Match the same kind of item with the same size, rating, fit or capacity.
For medicines: the same active ingredients, the same strength and the same form. Never group different strengths or forms, or a product with an extra ingredient.
Only products listed as PRODUCTS TO PLACE or OTHER PRODUCTS may be named. Every group must include at least one product to place.
Put a product into an EXISTING GROUP when it belongs there. Leave out anything you aren't sure of: a wrong group means a customer can be offered the wrong item.
Reply with STRICT JSON ONLY:
{"groups":[{"name":"","product_ids":[],"reason":""}],"joins":[{"product_id":0,"group_id":0,"reason":""}]}
"reason" says what they share, for example "both paracetamol 500mg tablets". Use empty arrays when nothing fits.`

func (p *Plugin) grouper() *plugin.ProductGrouper {
	if p.groupers == nil {
		return nil
	}
	if gs := p.groupers(); len(gs) > 0 {
		return &gs[0]
	}
	return nil
}

func groupsJob() Job {
	return Job{Key: "groups", Label: "Alternative group maker",
		Desc:      "Finds products that can replace each other, with a reason for each group. Check each one: the till offers these as swaps.",
		Available: func(p *Plugin) bool { return p.grouper() != nil },
		Select:    groupsSelect, Prompt: groupsPromptFor, Parse: groupsParse, Apply: groupsApply, Undo: groupsUndo}
}

// groupsSelect takes one category at a time, sending the rest of it as context:
// twins must sit side by side, and a batch of 20 from thousands rarely holds two.
func groupsSelect(ctx context.Context, p *Plugin, in JobRequest) (Batch, int, error) {
	var b Batch
	groups, err := p.grouper().Groups(ctx)
	if err != nil {
		return b, 0, err
	}
	in.exclude, in.needDetails, in.byName = nil, true, true
	for _, g := range groups {
		in.exclude = append(in.exclude, g.Members...)
	}
	if in.CategoryID == 0 && len(in.IDs) == 0 {
		first, _, err := selectProducts(ctx, p, "groups", JobRequest{Size: 1, Filter: in.Filter, exclude: in.exclude, needDetails: true, byName: true})
		if err != nil || len(first) == 0 {
			return b, 0, err
		}
		in.CategoryID = first[0].CategoryID
	}
	var total int
	if b.Products, total, err = selectProducts(ctx, p, "groups", in); err != nil || len(b.Products) == 0 {
		return b, total, err
	}
	asked := map[int64]bool{}
	for _, x := range b.Products {
		asked[x.ID] = true
	}
	// Everything ungrouped in this category, answered or not, may be a twin.
	cat := in.CategoryID
	if cat == 0 {
		cat = b.Products[0].CategoryID
	}
	others, err := p.groupContext(ctx, cat, in.exclude)
	if err != nil {
		return b, total, err
	}
	for _, x := range others {
		if !asked[x.ID] {
			b.Context = append(b.Context, x)
		}
	}
	return b, total, nil
}

const groupContextMax = 300

func (p *Plugin) groupContext(ctx context.Context, cat int64, exclude []int64) ([]JobProduct, error) {
	all, _, err := selectProducts(ctx, p, "groups", JobRequest{Size: groupContextMax, CategoryID: cat, exclude: exclude, needDetails: true, byName: true, wide: true})
	return all, err
}

func groupsPromptFor(ctx context.Context, p *Plugin, b Batch) (string, string, error) {
	groups, err := p.grouper().Groups(ctx)
	if err != nil {
		return "", "", err
	}
	line := func(x JobProduct) string {
		s := strconv.FormatInt(x.ID, 10) + "|" + x.Name
		if h := x.Hint(); h != "" {
			s += "|" + strings.ReplaceAll(h, "\n", " ")
		}
		return s + "\n"
	}
	var u strings.Builder
	u.WriteString("PRODUCTS TO PLACE (id|name|details):\n")
	for _, x := range b.Products {
		u.WriteString(line(x))
	}
	if len(b.Context) > 0 {
		u.WriteString("\nOTHER PRODUCTS in the same category (id|name|details):\n")
		for _, x := range b.Context {
			u.WriteString(line(x))
		}
	}
	if len(groups) > 0 {
		u.WriteString("\nEXISTING GROUPS (id|name):\n")
		for _, g := range groups {
			u.WriteString(strconv.FormatInt(g.ID, 10) + "|" + g.Name + "\n")
		}
	}
	return groupsPrompt, u.String(), nil
}

func groupsParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Groups []struct {
			Name       string  `json:"name"`
			ProductIDs []int64 `json:"product_ids"`
			Reason     string  `json:"reason"`
		} `json:"groups"`
		Joins []struct {
			ProductID int64  `json:"product_id"`
			GroupID   int64  `json:"group_id"`
			Reason    string `json:"reason"`
		} `json:"joins"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	names, place := map[int64]string{}, map[int64]bool{}
	for _, x := range b.Products {
		names[x.ID], place[x.ID] = x.Name, true
	}
	for _, x := range b.Context {
		names[x.ID] = x.Name
	}
	rows := []JobRow{}
	used := map[int64]bool{}
	for _, g := range r.Groups {
		name := strings.TrimSpace(g.Name)
		var ids []string
		var labels []string
		first := int64(0)
		for _, id := range g.ProductIDs {
			if names[id] == "" || used[id] {
				continue
			}
			ids, labels = append(ids, strconv.FormatInt(id, 10)), append(labels, names[id])
			if first == 0 && place[id] {
				first = id
			}
		}
		if name == "" || len(ids) < 2 || first == 0 {
			continue
		}
		for _, g := range g.ProductIDs {
			used[g] = true
		}
		rows = append(rows, JobRow{ProductID: first, Kind: "group", Key: "group", Label: "New group", New: name + "\n" + strings.Join(labels, " · "), Reason: strings.TrimSpace(g.Reason),
			Extra: map[string]string{"name": name, "members": strings.Join(ids, ",")}})
	}
	if len(r.Joins) == 0 {
		return rows, nil
	}
	groups, err := p.grouper().Groups(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]plugin.ProductGroupInfo{}
	for _, g := range groups {
		byID[g.ID] = g
	}
	for _, j := range r.Joins {
		g, ok := byID[j.GroupID]
		if !ok || !place[j.ProductID] || used[j.ProductID] || len(g.Tiers) == 0 {
			continue
		}
		used[j.ProductID] = true
		row := JobRow{ProductID: j.ProductID, Kind: "join", Key: "tier", Label: "Join group", New: g.Name, Reason: strings.TrimSpace(j.Reason), Extra: map[string]string{}}
		if len(g.Tiers) == 1 {
			row.Extra["tier"] = strconv.FormatInt(g.Tiers[0].ID, 10)
		} else {
			row.Needs = "Which tier?" // the person's choice, never the chatbot's
			for _, t := range g.Tiers {
				row.Choices = append(row.Choices, JobChoice{Value: strconv.FormatInt(t.ID, 10), Label: t.Name})
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func groupsApply(ctx context.Context, p *Plugin, _ *sqlx.Tx, r JobRow) ([]UndoOp, error) {
	g := p.grouper()
	if g == nil {
		return nil, errStale
	}
	switch r.Kind {
	case "group":
		var ids []int64
		for _, s := range strings.Split(r.Extra["members"], ",") {
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		id, err := g.Create(ctx, r.Extra["name"], ids)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, errors.New("those products are in other groups now")
		}
		return []UndoOp{{RowKind: "group", ProductID: r.ProductID, Applied: strconv.FormatInt(id, 10), Extra: map[string]string{"members": r.Extra["members"]}}}, nil
	case "join":
		tier, err := strconv.ParseInt(r.Extra["tier"], 10, 64)
		if err != nil || tier == 0 {
			return nil, errors.New("pick a tier first")
		}
		if err := g.AddTo(ctx, tier, r.ProductID); err != nil {
			return nil, err
		}
		return []UndoOp{{RowKind: "join", ProductID: r.ProductID}}, nil
	}
	return nil, errStale
}

func groupsUndo(ctx context.Context, p *Plugin, _ *sqlx.Tx, op UndoOp) error {
	g := p.grouper()
	if g == nil {
		return nil
	}
	switch op.RowKind {
	case "group":
		id, err := strconv.ParseInt(op.Applied, 10, 64)
		if err != nil {
			return err
		}
		// Only as it was made: if someone has added to it since, it's theirs now.
		groups, err := g.Groups(ctx)
		if err != nil {
			return err
		}
		for _, x := range groups {
			if x.ID == id {
				if len(x.Tiers) != 1 || len(x.Members) != len(strings.Split(op.Extra["members"], ",")) {
					return errStale
				}
				return g.Delete(ctx, id)
			}
		}
		return errStale
	case "join":
		return g.Remove(ctx, op.ProductID)
	}
	return nil
}
