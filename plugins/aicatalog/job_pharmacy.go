package aicatalog

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

const marksPrompt = `For each medicine, say whether it needs a prescription in Sri Lanka and whether it is a controlled drug.
Use null when you aren't sure: a wrong answer here can break the law. Give a short reason.
Reply with STRICT JSON ONLY:
{"items":[{"id":0,"rx_only":true,"controlled":false,"reason":""}]}`

const marksWarning = "No suggestion does not mean cleared. The chatbot can miss one. The pharmacist still checks the list."

func marksJob() Job {
	return Job{Key: "marks", Label: "Prescription and controlled marks",
		Desc:    "Suggests which medicines need a prescription or go on the controlled register. Pick the medicines' category, and have the pharmacist check every one.",
		Warning: marksWarning, ListNoAnswer: true,
		Available: func(p *Plugin) bool { return p.attrSet("pharmacy") != nil },
		Prompt: func(_ context.Context, _ *Plugin, b Batch) (string, string, error) {
			var u strings.Builder
			u.WriteString("MEDICINES (id|name|details):\n")
			for _, x := range b.Products {
				u.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "|" + strings.ReplaceAll(x.Hint(), "\n", " ") + "\n")
			}
			return marksPrompt, u.String(), nil
		},
		Parse: marksParse, Apply: attrApply("pharmacy"), Undo: attrUndo("pharmacy")}
}

func marksParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Items []struct {
			ID         int64  `json:"id"`
			RxOnly     *bool  `json:"rx_only"`
			Controlled *bool  `json:"controlled"`
			Reason     string `json:"reason"`
		} `json:"items"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	ids := make([]int64, len(b.Products))
	asked := map[int64]bool{}
	for i, x := range b.Products {
		ids[i], asked[x.ID] = x.ID, true
	}
	cur, err := p.attrSet("pharmacy").Values(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := []JobRow{}
	for _, it := range r.Items {
		if !asked[it.ID] {
			continue
		}
		for _, m := range []struct {
			key, label string
			v          *bool
		}{{"controlled", "Controlled drug", it.Controlled}, {"rx_only", "Needs a prescription", it.RxOnly}} { // controlled first: while it's on, Rx-only can't clear
			if m.v == nil { // not sure = no answer, which stays on the pharmacist's list
				continue
			}
			old := cur[it.ID][m.key]
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "attr", Key: m.key, Label: m.label, Old: old, New: strings.ToLower(yesNo(*m.v)),
				Reason: strings.TrimSpace(it.Reason), Change: old != "" && old != strings.ToLower(yesNo(*m.v))})
		}
	}
	return rows, nil
}

const dosePrompt = `Suggest the usual adult dose for each medicine, for a pharmacist to check.
Use the exact wording of a dose on the shop's list when one fits.
If none fits, set "new" to true and give "dose" as a short form like 1-0-0 or every 6 hours (never the word "new"), its "meaning" for a label, and "per_day".
"per_day" is doses a day; a dose taken only as needed is 0.
If it depends on the patient, leave "dose" empty: never guess.
Reply with STRICT JSON ONLY:
{"items":[{"id":0,"dose":"","new":false,"meaning":"","per_day":0,"reason":""}]}`

const doseWarning = "A chatbot's dose is only a suggestion. The pharmacist approves each one, and it's the usual adult dose, not advice for a patient."

const doseKnownMax = 200

func doseJob() Job {
	return Job{Key: "dose", Label: "Usual dose", OneByOne: true,
		Desc:      "Suggests a usual dose from your own dose list, so the wording stays the same. Never replaces a dose already set.",
		Warning:   doseWarning,
		Available: func(p *Plugin) bool { return p.attrSet("pharmacy") != nil },
		Select:    doseSelect, Prompt: dosePromptFor, Parse: doseParse, Apply: doseApply, Undo: doseUndo}
}

// doseSelect leaves out medicines that already have a dose, unless they're being redone.
func doseSelect(ctx context.Context, p *Plugin, in JobRequest) (Batch, int, error) {
	var b Batch
	var err error
	if len(in.IDs) > 0 {
		b.Products, _, err = selectProducts(ctx, p, "dose", in)
		return b, len(b.Products), err
	}
	size := in.Size
	if size <= 0 {
		size = 1
	}
	in.noCap = true
	all, _, err := selectProducts(ctx, p, "dose", in)
	if err != nil {
		return b, 0, err
	}
	ids := make([]int64, len(all))
	for i, x := range all {
		ids[i] = x.ID
	}
	cur, err := p.attrSet("pharmacy").Values(ctx, ids)
	if err != nil {
		return b, 0, err
	}
	total := 0
	for _, x := range all {
		if cur[x.ID]["dose"] != "" {
			continue
		}
		total++
		if len(b.Products) < min(size, jobBatchMax) {
			b.Products = append(b.Products, x)
		}
	}
	return b, total, nil
}

func dosePromptFor(ctx context.Context, p *Plugin, b Batch) (string, string, error) {
	known, err := p.attrSet("pharmacy").Known(ctx, "dose", doseKnownMax)
	if err != nil {
		return "", "", err
	}
	var u strings.Builder
	u.WriteString("THE SHOP'S DOSE LIST (dose|meaning|doses a day):\n")
	for _, k := range known {
		u.WriteString(k.Value + "|" + k.Meaning + "|" + k.Extra["perDay"] + "\n")
	}
	u.WriteString("\nMEDICINES (id|name|details):\n")
	for _, x := range b.Products {
		u.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "|" + strings.ReplaceAll(x.Hint(), "\n", " ") + "\n")
	}
	return dosePrompt, u.String(), nil
}

var doseSpaces = regexp.MustCompile(`\s+`)

func doseKey(s string) string {
	return strings.ToLower(doseSpaces.ReplaceAllString(strings.TrimSpace(s), ""))
}

func doseParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Items []struct {
			ID      int64   `json:"id"`
			Dose    string  `json:"dose"`
			New     bool    `json:"new"`
			Meaning string  `json:"meaning"`
			PerDay  float64 `json:"per_day"`
			Reason  string  `json:"reason"`
		} `json:"items"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	set := p.attrSet("pharmacy")
	known, err := set.Known(ctx, "dose", doseKnownMax)
	if err != nil {
		return nil, err
	}
	onList := map[string]string{}
	for _, k := range known {
		onList[doseKey(k.Value)] = k.Value
	}
	ids := make([]int64, len(b.Products))
	asked := map[int64]bool{}
	for i, x := range b.Products {
		ids[i], asked[x.ID] = x.ID, true
	}
	cur, err := set.Values(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := []JobRow{}
	for _, it := range r.Items {
		if !asked[it.ID] {
			continue
		}
		dose := strings.TrimSpace(it.Dose)
		if strings.EqualFold(dose, "new") { // ChatGPT's real answer: it's no dose at all
			dose = ""
		}
		meaning := strings.TrimSpace(it.Meaning)
		if dose == "" && (!it.New || meaning == "") {
			continue // it depends on the patient: no answer
		}
		old := cur[it.ID]["dose"]
		if old != "" && !b.Redo {
			continue // never over a dose already set
		}
		row := JobRow{ProductID: it.ID, Kind: "dose", Key: "dose", Label: "Usual dose", Old: old, New: dose, Reason: strings.TrimSpace(it.Reason), Change: old != ""}
		if same, ok := onList[doseKey(dose)]; ok && dose != "" {
			row.New = same
		} else {
			row.IsNew = true
			row.Extra = map[string]string{"meaning": meaning, "perDay": strconv.FormatFloat(max(it.PerDay, 0), 'f', -1, 64)}
			if dose == "" {
				row.Needs = "Type the dose, like 1-0-0"
			}
		}
		if strings.EqualFold(old, row.New) && row.New != "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func doseApply(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error) {
	set := p.attrSet("pharmacy")
	if set == nil || strings.TrimSpace(r.New) == "" || strings.EqualFold(strings.TrimSpace(r.New), "new") {
		return nil, errStale
	}
	r.New = strings.TrimSpace(r.New)
	cur, err := set.Values(ctx, []int64{r.ProductID})
	if err != nil {
		return nil, err
	}
	if cur[r.ProductID]["dose"] != r.Old {
		return nil, errStale
	}
	var ops []UndoOp
	if r.IsNew {
		known, err := set.Known(ctx, "dose", doseKnownMax)
		if err != nil {
			return nil, err
		}
		exists := false
		for _, k := range known {
			exists = exists || doseKey(k.Value) == doseKey(r.New)
		}
		if !exists {
			if err := set.AddKnown(ctx, "dose", knownDose(r)); err != nil {
				return nil, err
			}
			// Undo replays backwards: the medicine's dose goes first, then this.
			ops = append(ops, UndoOp{RowKind: "dose_new", ProductID: r.ProductID, Applied: r.New})
		}
	}
	more, err := attrApply("pharmacy")(ctx, p, tx, r)
	if err != nil {
		if len(ops) > 0 {
			_ = set.RemoveKnown(ctx, "dose", r.New) // nothing uses it now
		}
		return nil, err
	}
	return append(ops, more...), nil
}

func doseUndo(ctx context.Context, p *Plugin, tx *sqlx.Tx, op UndoOp) error {
	if op.RowKind == "dose_new" {
		set := p.attrSet("pharmacy")
		if set == nil {
			return nil
		}
		return set.RemoveKnown(ctx, "dose", op.Applied)
	}
	return attrUndo("pharmacy")(ctx, p, tx, op)
}

func knownDose(r JobRow) plugin.KnownValue {
	return plugin.KnownValue{Value: r.New, Meaning: r.Extra["meaning"], Extra: map[string]string{"perDay": r.Extra["perDay"]}}
}
