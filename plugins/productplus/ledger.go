package productplus

import "karots-suite/internal/ledgercheck"

// Product Plus's own rules the data can show (docs/rules/plugins/productplus.md).
func init() {
	// PPF-1: every stored value fits its field. Yes / no is "1" or ""; a number is a
	// plain decimal; a date a real YYYY-MM-DD. (A dropdown value may be an option
	// removed since — kept on purpose and shown as removed — so it isn't checked.)
	ledgercheck.Add("PPF-1", `
		SELECT format('product %s field %s', v.product_id, f.key), format('%s value %L', f.type, v.value)
		FROM pp_values v JOIN pp_fields f ON f.id = v.field_id
		WHERE v.value <> '' AND (
		      (f.type = 'bool' AND v.value <> '1')
		   OR (f.type = 'number' AND v.value !~ '^[-+]?([0-9]+([.][0-9]*)?|[.][0-9]+)([eE][-+]?[0-9]+)?$')
		   OR (f.type = 'date' AND (v.value !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' OR NOT pg_input_is_valid(v.value, 'date'))))`)
	// PPF-6 / PLG-2 / PLG-3: a value names a product that exists and wasn't merged away.
	ledgercheck.Add("PPF-6", `
		SELECT format('product %s field %s', v.product_id, v.field_id),
		       COALESCE('its product was merged into ' || p.merged_into_id, 'no such product')
		FROM pp_values v LEFT JOIN products p ON p.id = v.product_id
		WHERE p.id IS NULL OR p.merged_into_id IS NOT NULL`)
	// PPF-9: labels unique (keys are, by the database).
	ledgercheck.Add("PPF-9", `
		SELECT 'field ' || min(id), format('%s fields called %L', count(*), lower(btrim(label)))
		FROM pp_fields GROUP BY lower(btrim(label)) HAVING count(*) > 1`)
	// PPF-10: the supplier field is a text or dropdown field.
	ledgercheck.Add("PPF-10", `
		SELECT 'supplier field ' || f.id, format('a %s field gives the supplier', f.type)
		FROM pp_supplier_field sf JOIN pp_fields f ON f.id = sf.field_id
		WHERE f.type NOT IN ('text', 'select')`)
	// PPF-11: a rule's value is kept trimmed and lower-cased, or it never matches.
	ledgercheck.Add("PPF-11", `
		SELECT format('rule %L', value_key), 'value not trimmed and lower-cased'
		FROM pp_supplier_map WHERE value_key <> lower(btrim(value_key))`)
}
