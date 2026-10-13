package aicatalog

import (
	"context"
	"fmt"
	"strings"
)

// jsonOnly ends every prompt: the parser tolerates prose and fences around the
// answer, but asking keeps chatbots short.
const jsonOnly = "Reply with JSON only."

const systemPrompt = `You identify retail/spare-part products from a short code or name.
Reply with STRICT JSON ONLY, no prose, matching:
{"confident":bool,"best":{"name":"","category":"","specs":"","explanation":""},
"options":[{"name":"","category":"","specs":"","explanation":""}]}
Set confident=true and fill "best" only when you are sure. Otherwise set
confident=false and give up to 4 "options". "category" uses " > " between levels,
but keep it GENERAL and SHALLOW: usually ONE broad category, at most two levels.
Only add a deeper sub-category when a shop would stock MANY items of that exact
type. Never build long chains like A > B > C > D. Prefer "Bearings" (or at most
"Bearings > Ball Bearings"), NOT "Automotive > Parts > Bearings > Deep Groove".
"explanation" is one short sentence a non-expert understands. "specs" holds key
dimensions/ratings if known.
When web search is available, ALWAYS look the item up to confirm it rather than
relying on memory — codes and part numbers are easily confused, so verify before
answering. If you cannot confirm what it is, set confident=false and offer your
best options instead of guessing a single answer.
If a list of the shop's EXISTING categories is given, and one of them fits the
item, REUSE its exact path verbatim instead of inventing a new name — so similar
items land together. Only propose a new category when none of the existing ones fit.`

// withCustom appends the owner's standing custom instructions (every prompt).
func withCustom(system, custom string) string {
	custom = strings.TrimSpace(custom)
	if custom == "" {
		return system
	}
	return system + "\n\nADDITIONAL INSTRUCTIONS (follow these strictly):\n" + custom
}

// withNaming folds the naming style into identify prompts only.
func withNaming(system, naming string) string {
	naming = strings.TrimSpace(naming)
	if naming == "" {
		return system
	}
	return system + "\n\nPRODUCT NAMING RULES (name the item to match these exactly):\n" + naming
}

// identifyUserMsg describes the item: what was typed, the barcode, the owner's
// hint and the shop's category paths.
func identifyUserMsg(query, barcode, hint string, cats []string) string {
	user := "Identify this item: " + strings.TrimSpace(query)
	if b := strings.TrimSpace(barcode); b != "" && b != strings.TrimSpace(query) {
		user += "\nBarcode: " + b
	}
	if strings.TrimSpace(hint) != "" {
		user += "\nAdditional context from the shop owner: " + strings.TrimSpace(hint)
	}
	if len(cats) > 0 {
		user += "\n\nEXISTING shop categories (reuse a fitting one verbatim, else propose new):\n- " + strings.Join(cats, "\n- ")
	}
	return user
}

// IdentifyPrompt is the whole paste prompt: rules, naming style, custom
// instructions, the item and the categories.
func IdentifyPrompt(query, barcode, hint string, cats []string, s Settings) string {
	return withCustom(withNaming(systemPrompt, s.NamingStyle), s.CustomPrompt) + "\n\n" +
		identifyUserMsg(query, barcode, hint, cats) + "\n\n" + jsonOnly
}

// IdentifyFollowUp is the short prompt for the same chat: it remembers the rules.
func IdentifyFollowUp(query, barcode, hint string) string {
	user := "Next item: " + strings.TrimSpace(query)
	if b := strings.TrimSpace(barcode); b != "" && b != strings.TrimSpace(query) {
		user += " (barcode " + b + ")"
	}
	if strings.TrimSpace(hint) != "" {
		user += "\nMore about it: " + strings.TrimSpace(hint)
	}
	return user + "\nSame rules as before. " + jsonOnly
}

func (c *Client) Identify(ctx context.Context, query, barcode, hint string, cats []string) (IdentifyResult, error) {
	content, err := c.complete(ctx, withNaming(systemPrompt, c.cfg.NamingStyle), identifyUserMsg(query, barcode, hint, cats), c.searchEnabled())
	if err != nil {
		return IdentifyResult{}, err
	}
	return parseIdentify(content)
}

const cleanupSystemPrompt = `You tidy a shop's product catalogue. You are given CATEGORIES
(id|name|parentId|productCount|path; parentId "-" means top level) and PRODUCTS
(id|name|categoryId|flag; flag U = uncategorized). Propose ONLY clearly beneficial,
conservative changes. Reply STRICT JSON ONLY:
{"renames":[{"id":0,"to":"","reason":""}],
 "reparents":[{"id":0,"parent_id":0,"reason":""}],
 "merges":[{"from":0,"into":0,"reason":""}],
 "moves":[{"product_id":0,"category_id":0,"reason":""}],
 "uncategorized":[{"product_id":0,"category":"","reason":""}],
 "names":[{"product_id":0,"to":"","reason":""}],
 "duplicates":[{"a":0,"b":0,"reason":""}]}
Rules:
- Keep the tree GENERAL and SHALLOW. FLATTEN over-specific categories that hold few products (say under ~5) into their broader parent or a sibling — via merge, or reparent to a broader category. Only keep a deep/very specific category when it holds MANY products. Avoid long chains like A > B > C > D.
- merges: fold duplicate / near-duplicate (or tiny over-specific) categories together; "into" is the id you keep (broader/better/plural name), "from" is removed.
- reparents: group related loose top-level categories under a broad parent (parent_id = an existing category id; use 0 to make top level).
- renames: fix casing/spelling/singular-plural for consistency.
- moves: move an obviously mis-filed product to a better EXISTING category id.
- uncategorized: for EVERY product flagged U, a "category" path using " > " between levels — an existing category's path verbatim when one fits, else a new GENERAL, SHALLOW one.
- duplicates: pairs of products that are the SAME item listed twice (e.g. "Coke 1L" and "Coca Cola 1 Ltr"). Only when sure — a different size, flavour, colour or variant is NOT a duplicate.
Only reference ids that appear in the lists; parent_id and category_id must be existing category ids. Use empty arrays when nothing needs changing. No prose.`

const namesNone = `- names: always an empty array.`

func namesRule(naming string) string {
	naming = strings.TrimSpace(naming)
	if naming == "" {
		return namesNone
	}
	return "- names: rename products whose name does not follow the PRODUCT NAMING RULES below; leave names that already fit alone.\n\nPRODUCT NAMING RULES:\n" + naming
}

// CleanupProduct is one product row the clean-up reasons over.
type CleanupProduct struct {
	ID         int64  `db:"id"`
	Name       string `db:"name"`
	CategoryID int64  `db:"category_id"`
	Uncat      bool   `db:"uncat"`
}

func cleanupUserMsg(cats []CatRow, paths map[int64]string, prods []CleanupProduct) string {
	var b strings.Builder
	b.WriteString("CATEGORIES (id|name|parentId|productCount|path):\n")
	for _, c := range cats {
		parent := "-"
		if c.ParentID != nil {
			parent = fmt.Sprintf("%d", *c.ParentID)
		}
		fmt.Fprintf(&b, "%d|%s|%s|%d|%s\n", c.ID, c.Name, parent, c.Count, paths[c.ID])
	}
	b.WriteString("\nPRODUCTS (id|name|categoryId|flag):\n")
	for _, p := range prods {
		flag := ""
		if p.Uncat {
			flag = "U"
		}
		fmt.Fprintf(&b, "%d|%s|%d|%s\n", p.ID, p.Name, p.CategoryID, flag)
	}
	return b.String()
}

func cleanupSystem(s Settings) string { return cleanupSystemPrompt + "\n" + namesRule(s.NamingStyle) }

// CleanupPrompt is the whole paste prompt for one chunk.
func CleanupPrompt(cats []CatRow, paths map[int64]string, prods []CleanupProduct, s Settings) string {
	return withCustom(cleanupSystem(s), s.CustomPrompt) + "\n\n" + cleanupUserMsg(cats, paths, prods) + "\n" + jsonOnly
}

func (c *Client) Cleanup(ctx context.Context, cats []CatRow, paths map[int64]string, prods []CleanupProduct) (CleanupPlan, error) {
	content, err := c.complete(ctx, cleanupSystem(c.cfg), cleanupUserMsg(cats, paths, prods), false)
	if err != nil {
		return CleanupPlan{}, err
	}
	return parseCleanup(content)
}
