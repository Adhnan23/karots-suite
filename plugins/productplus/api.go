package productplus

import (
	"slices"
	"strings"

	"karots-suite/internal/apperr"
)

// The plugin's own JSON (core's types.gen.ts is core-only); mirrored by hand in
// web/src/plugins/productplus/types.ts.

type FieldDTO struct {
	ID           int64    `json:"id"`
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Type         string   `json:"type"`
	DefaultValue string   `json:"defaultValue"`
	Hint         string   `json:"hint"`
	Required     bool     `json:"required"`
	Searchable   bool     `json:"searchable"`
	ShowAtTill   bool     `json:"showAtTill"`
	PrintOnLabel bool     `json:"printOnLabel"`
	ShowInIntake bool     `json:"showInIntake"`
	Options      []string `json:"options"`
	IsActive     bool     `json:"isActive"`
	Scope        []Cat    `json:"scope"` // empty = all products
	Missing      int      `json:"missing"`
	UsingDefault int      `json:"usingDefault"`
}

// FieldRequest creates or edits a field. DefaultFor ("new" | "all") and
// ConfirmLoss only matter on an edit (see UpdateOpts).
type FieldRequest struct {
	Label        string   `json:"label"`
	Type         string   `json:"type"`
	DefaultValue string   `json:"defaultValue"`
	Hint         string   `json:"hint"`
	Required     bool     `json:"required"`
	Searchable   bool     `json:"searchable"`
	ShowAtTill   bool     `json:"showAtTill"`
	PrintOnLabel bool     `json:"printOnLabel"`
	ShowInIntake bool     `json:"showInIntake"`
	Options      []string `json:"options"`
	Scope        []int64  `json:"scope"`
	DefaultFor   string   `json:"defaultFor"`
	ConfirmLoss  bool     `json:"confirmLoss"`
}

// FormFieldDTO is a field on the product form / intake with the product's value.
type FormFieldDTO struct {
	ID       int64    `json:"id"`
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Hint     string   `json:"hint"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
	Value    string   `json:"value"`
	Removed  bool     `json:"removed"` // a dropdown value whose option was removed
}

type GridDTO struct {
	Rows  []GridRow `json:"rows"`
	Total int       `json:"total"`
}

type SaveResult struct {
	Saved  int         `json:"saved"`
	Errors []CellError `json:"errors"`
}

// extraBody is what the product form sends as ProductRequest.extra.productplus.
type extraBody struct {
	Values map[string]string `json:"values"`
}

func fieldDTO(f Field, st Stats) FieldDTO {
	return FieldDTO{ID: f.ID, Key: f.Key, Label: f.Label, Type: f.Type, DefaultValue: f.DefaultValue, Hint: f.Hint,
		Required: f.Required, Searchable: f.Searchable, ShowAtTill: f.ShowAtTill, PrintOnLabel: f.PrintOnLabel,
		ShowInIntake: f.ShowInIntake, Options: orEmpty(f.Options), IsActive: f.IsActive, Scope: orEmptyCats(f.Scope),
		Missing: st.Missing, UsingDefault: st.UsingDefault}
}

func formDTO(ff FormField) FormFieldDTO {
	return FormFieldDTO{ID: ff.Field.ID, Key: ff.Field.Key, Label: ff.Field.Label, Type: ff.Field.Type, Hint: ff.Field.Hint,
		Required: ff.Field.Required, Options: orEmpty(ff.Field.Options), Value: ff.Value, Removed: ff.Removed}
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func orEmptyCats(s []Cat) []Cat {
	if s == nil {
		return []Cat{}
	}
	return s
}

var fieldTypes = []string{"text", "textarea", "number", "bool", "select", "date"}

// toField checks a field request (the karots-pos rules, minus "a required field
// needs a default": missing values are listed instead) and returns the field.
func (r FieldRequest) toField() (Field, error) {
	f := Field{Label: strings.TrimSpace(r.Label), Type: r.Type, DefaultValue: strings.TrimSpace(r.DefaultValue),
		Hint: strings.TrimSpace(r.Hint), Required: r.Required, Searchable: r.Searchable, ShowAtTill: r.ShowAtTill,
		PrintOnLabel: r.PrintOnLabel, ShowInIntake: r.ShowInIntake}
	if f.Label == "" {
		return f, apperr.Validation("give the field a label")
	}
	if !slices.Contains(fieldTypes, f.Type) {
		return f, apperr.Validation("choose a field type")
	}
	if f.Type == "select" {
		for _, o := range r.Options {
			if o = strings.TrimSpace(o); o != "" && !slices.Contains(f.Options, o) {
				f.Options = append(f.Options, o)
			}
		}
		if len(f.Options) == 0 {
			return f, apperr.Validation("a dropdown needs at least one option")
		}
	}
	if f.DefaultValue != "" || f.Type == "bool" {
		v, err := coerce(f, f.DefaultValue, "")
		if err != nil {
			return f, apperr.Validation("default — " + err.Error())
		}
		f.DefaultValue = v
	}
	return f, nil
}
