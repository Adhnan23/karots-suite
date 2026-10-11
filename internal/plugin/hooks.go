package plugin

import (
	"context"
	"encoding/json"
	"time"

	"karots-suite/internal/features/activity"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Additive UI hooks. A plugin's Setup registers these; the template layer reads
// them through the exported getters to weave plugin UI into the core shell. They
// are additive only — overriding a whole page is done via the route Mux instead.

// AdminNavEntry adds an admin destination. If Section names an existing section
// key (e.g. "setup"), the link is nested under it; if Section is empty, a new
// top-level section is created from SectionLabel + Icon holding this one link.
type AdminNavEntry struct {
	Section      string // existing section key to nest under; "" = new top-level section
	SectionLabel string // label for the new top-level section (when Section == "")
	Icon         string // emoji/icon for the new top-level section
	Href         string
	Label        string
	Key          string
	Desc         string
}

// CashierTab adds a link to the cashier shell.
type CashierTab struct {
	Href  string
	Label string
	Key   string
}

// PaletteEntry adds a command-palette destination.
type PaletteEntry struct {
	Href  string
	Label string
	Group string
}

// ReportCard adds a card to the core Reports hub (/admin/reports). Registered by
// a plugin so its report shows alongside the built-in ones when it's enabled.
type ReportCard struct {
	Href  string
	Label string
	Desc  string
}

// TenderMethod adds a payment method to the cashier POS split-tender selector.
// Value must be a payment_method enum value the plugin's migration added; Label
// is shown on the till. Any detail picker is the plugin's React slots.tillTenders entry.
type TenderMethod struct {
	Value string
	Label string
	// SettleTx, when set, books this tender's plugin-side money movement inside the
	// core sale's transaction (so it commits atomically with the sale rather than a
	// best-effort call afterwards). Optional — a tender without one is recorded on
	// the sale only. The core sale dispatches to it via the injected plugin-tender
	// hook (see sales.WithPluginTenderLeg), so core never imports the plugin.
	SettleTx func(ctx context.Context, tx *sqlx.Tx, ev TenderSettle) error
}

// TenderSettle is what a TenderMethod.SettleTx receives for one tender line on a
// sale: the sale id, the amount tendered, the acting cashier, and the method's
// opaque per-line Meta.
type TenderSettle struct {
	SaleID  int64
	Amount  decimal.Decimal
	Meta    map[string]string
	ActorID int64
}

// CashierMenuRoot adds a card at the ROOT of the cashier menu (alongside the
// product-group cards). Tapping it navigates into ChildrenURL, a GET that returns
// the menu-node JSON protocol ({"nodes":[…]}). This replaces the old quick-action
// strip: a plugin's actions live in the same drill-down menu as products.
type CashierMenuRoot struct {
	Key         string
	Emoji       string
	Label       string
	ChildrenURL string
}

// DrawerSection contributes an extra panel to the till OPEN and CLOSE dialogs
// (e.g. a plugin's per-session sub-ledger the cashier counts alongside the
// drawer). Core renders an empty slot; client-side it loads each section's form
// fragment and posts it to the section's save URL around the drawer call. The
// fragment is plain inputs (no hx-*); core never references the plugin's domain.
type DrawerSection struct {
	Key          string // stable id, e.g. the plugin key
	OpenFormURL  string // GET → HTML input rows for the Open-till dialog
	CloseFormURL string // GET → HTML input rows for the Close-register dialog
	SaveOpenURL  string // POST (form-encoded) target, after the till opens
	SaveCloseURL string // POST (form-encoded) target, before the till closes
}

// CashierSupplierAction adds a button to the cashier Suppliers page (the counter
// supplier section). A plugin registers it so an action that belongs with
// "paying a supplier" — e.g. buying reload float from a carrier — lives there
// instead of on the plugin's own page, where a cashier once fired it by mistake.
// The button hx-gets FormURL (a GET returning an HTML fragment, typically a
// modal) into #modal-container; that fragment posts to the plugin's own
// endpoint. Core never learns what the action does. Gated the same as the page:
// only cashiers with supplier access (and admins/managers) ever see it, because
// the page they're on is.
type CashierSupplierAction struct {
	Key     string
	Emoji   string
	Label   string
	FormURL string // GET → HTML fragment (the plugin's form), loaded into #modal-container
}

// LogoutGuard reports whether a user has unfinished plugin work that must be
// resolved before they may log out — e.g. an open plugin float session. When
// Block is true the web layer refuses to log the user out and instead sends them
// to Redirect (a page where they can resolve it) with Reason shown as a banner.
// Guards must be cheap (one indexed query) and fail open (return Block=false on
// error) so a plugin issue can never trap a user in a non-logout-able state.
type LogoutGuard func(ctx context.Context, userID int64) (block bool, redirect, reason string)

var (
	adminNav               hook[AdminNavEntry]
	cashierTabs            hook[CashierTab]
	paletteEntries         hook[PaletteEntry]
	reportCards            hook[ReportCard]
	cashierMenuRoots       hook[CashierMenuRoot]
	drawerSections         hook[DrawerSection]
	tenderMethods          hook[TenderMethod]
	logoutGuards           hook[LogoutGuard]
	receiptTabs            hook[ReceiptTab]
	supplierActions        hook[CashierSupplierAction]
	plIncomes              hook[PLIncome]
	expenseOwners          hook[ExpenseOwner]
	activityContribs       hook[ActivityContributor]
	moneyLocationProviders hook[MoneyLocationProvider]
	readinessProviders     hook[ReadinessProvider]
	dataGroups             hook[DataGroup]
)

// ReceiptTab adds a tab to the unified Receipts page on BOTH the admin and cashier
// shells. The core page renders a tab button (Label) plus a panel that lazy-loads
// the plugin's own fragment endpoint for the current role: CashierHref on the
// cashier shell, AdminHref on the admin shell. Key must be unique across plugins.
// A plugin may register several (e.g. one per receipt kind).
type ReceiptTab struct {
	Key         string
	Label       string
	CashierHref string
	AdminHref   string
}

// ExpenseOwner lets a plugin claim core expense rows that its own records point
// at (an expense_id column). Expenses refuses to void a claimed expense —
// deleting it there would leave the plugin's record pointing at nothing and its
// own state (a float, a payout) un-reversed. Return the label of the owning
// record (e.g. "a reload float top-up"), or "" when not claimed (karots-pos d66ac18).
type ExpenseOwner func(ctx context.Context, expenseID int64) (string, error)

// ExpenseClaim returns the first plugin claim on an expense, or "" if none.
// A failing owner counts as a claim, so an error never lets a void through.
func ExpenseClaim(ctx context.Context, expenseID int64) string {
	for _, o := range expenseOwners.all() {
		label, err := o(ctx, expenseID)
		if err != nil {
			return "a plugin record (couldn't be checked)"
		}
		if label != "" {
			return label
		}
	}
	return ""
}

// PLIncome lets a plugin contribute an income line to the core Profit & Loss.
// Amount is summed for the [from,to) range and folded into the P&L by the web
// layer (which owns both reports and plugins, so core reports stay plugin-free).
// Keep Amount cheap — one query — and defensive: return (0, err) rather than
// failing the whole P&L. E.g. a plugin's service charges or commission, whose
// pass-through face value is excluded from core revenue via the pass_through
// product flag.
type PLIncome struct {
	Label  string
	Amount func(ctx context.Context, from, to time.Time) (decimal.Decimal, error)
}

// ActivityContributor lets a plugin feed its own trail into the central Activity
// view (which can't SQL-union plugin tables — core stays plugin-free). The web
// layer calls List with the active filter and merges the returned rows with the
// core sources, then sorts and paginates the whole set. Source is the plugin's
// source id (also its Source-filter value); keep it stable and unique. Return
// (nil, err) defensively rather than failing the whole page. Rows for the hidden
// system/developer account must be excluded here too — same as core.
type ActivityContributor struct {
	Source string
	List   func(ctx context.Context, f activity.Filter) ([]activity.Row, error)
}

// MoneyLocationProvider lets a plugin contribute live money-location balances —
// money the shop owns that isn't a core locker or till, e.g. float a plugin
// holds on devices — to the core Cash Flow overview (its own balance
// card + a net-worth line) and to the day-end Z report (a section). Balances are
// current; keep the query cheap and return (nil, err) so a plugin issue never
// breaks either page. Group is the card/section heading.
type MoneyLocationProvider struct {
	Group string
	List  func(ctx context.Context) ([]MoneyLocation, error)
}

// MoneyLocation is one named balance within a provider's group.
type MoneyLocation struct {
	Name    string
	Balance decimal.Decimal
}

// MoneyLocationGroup is a provider's resolved balances with the group total, as
// consumed by the Cash Flow and Z-report templates.
type MoneyLocationGroup struct {
	Group string
	Items []MoneyLocation
	Total decimal.Decimal
}

// ReadinessProvider lets a plugin contribute sections to the core "Top-up &
// Reorder" report — a not-time-based snapshot of "what must I top up, and can I
// afford it". E.g. a plugin's per-device float (current vs a recommended
// level, the shortfall being cash to buy). Return (nil, err) defensively; a
// plugin issue must never break the report.
type ReadinessProvider func(ctx context.Context) ([]ReadinessSection, error)

// ReadinessSection is one titled block of readiness rows plus its total need.
type ReadinessSection struct {
	Title     string
	Rows      []ReadinessRow
	NeedTotal decimal.Decimal // sum of Need across the rows (cash to reach every target)
}

// ReadinessRow is one thing to top up. Have/Target are pre-formatted for display;
// Need is the money to reach the target (0 when at/over). When EditURL is set the
// report renders an inline target editor: an input named EditName seeded with
// EditValue that POSTs to EditURL (the plugin updates the target and responds
// HX-Refresh so the report reloads).
type ReadinessRow struct {
	Name      string
	Have      string
	Target    string
	Need      decimal.Decimal
	EditURL   string
	EditName  string
	EditValue string
}

// Hook registration — plugins call these from Setup.
func (r *Registry) AddAdminNav(e AdminNavEntry)          { adminNav.add(current, e) }
func (r *Registry) AddCashierTab(t CashierTab)           { cashierTabs.add(current, t) }
func (r *Registry) AddPaletteEntry(p PaletteEntry)       { paletteEntries.add(current, p) }
func (r *Registry) AddReportCard(rc ReportCard)          { reportCards.add(current, rc) }
func (r *Registry) AddCashierMenuRoot(m CashierMenuRoot) { cashierMenuRoots.add(current, m) }
func (r *Registry) AddDrawerSection(s DrawerSection)     { drawerSections.add(current, s) }
func (r *Registry) AddTenderMethod(t TenderMethod)       { tenderMethods.add(current, t) }
func (r *Registry) AddLogoutGuard(g LogoutGuard)         { logoutGuards.add(current, g) }
func (r *Registry) AddReceiptTab(t ReceiptTab)           { receiptTabs.add(current, t) }
func (r *Registry) AddCashierSupplierAction(a CashierSupplierAction) {
	supplierActions.add(current, a)
}
func (r *Registry) AddPLIncome(s PLIncome)         { plIncomes.add(current, s) }
func (r *Registry) AddExpenseOwner(o ExpenseOwner) { expenseOwners.add(current, o) }
func (r *Registry) AddActivityContributor(a ActivityContributor) {
	activityContribs.add(current, a)
}
func (r *Registry) AddMoneyLocationProvider(p MoneyLocationProvider) {
	moneyLocationProviders.add(current, p)
}
func (r *Registry) AddReadinessProvider(p ReadinessProvider) {
	readinessProviders.add(current, p)
}
func (r *Registry) AddDataGroup(g DataGroup) { dataGroups.add(current, g) }

// DataGroup lets a plugin declare the database tables it owns, so the backup
// layer can offer per-plugin include/exclude in a selective export and compute
// "core = everything minus every plugin's tables" — all without core importing
// any plugin. Key is a stable id (the plugin key); Name is the display label;
// Tables are the exact table names from the plugin's migrations.
type DataGroup struct {
	Key    string
	Name   string
	Tables []string
}

// CollectDataGroups returns every plugin-declared data group, in registration
// order, for the backup layer and the Data page.
func CollectDataGroups() []DataGroup { return dataGroups.all() }

// CollectReadiness resolves every registered readiness provider into sections for
// the Top-up & Reorder report. A provider that errors or returns nothing is
// skipped — never breaks the report.
func CollectReadiness(ctx context.Context) []ReadinessSection {
	var out []ReadinessSection
	for _, p := range readinessProviders.all() {
		secs, err := p(ctx)
		if err != nil {
			continue
		}
		out = append(out, secs...)
	}
	return out
}

// CollectMoneyLocations resolves every registered provider's current balances
// into groups with totals, for the Cash Flow overview and the Z report. A
// provider that errors or returns nothing is skipped — never breaks the page.
func CollectMoneyLocations(ctx context.Context) []MoneyLocationGroup {
	var out []MoneyLocationGroup
	for _, p := range moneyLocationProviders.all() {
		items, err := p.List(ctx)
		if err != nil || len(items) == 0 {
			continue
		}
		g := MoneyLocationGroup{Group: p.Group, Items: items}
		for _, it := range items {
			g.Total = g.Total.Add(it.Balance)
		}
		out = append(out, g)
	}
	return out
}

// Getters for the template layer.
func AdminNav() []AdminNavEntry                   { return adminNav.all() }
func CashierTabs() []CashierTab                   { return cashierTabs.all() }
func PaletteEntries() []PaletteEntry              { return paletteEntries.all() }
func ReportCards() []ReportCard                   { return reportCards.all() }
func PLIncomes() []PLIncome                       { return plIncomes.all() }
func ActivityContributors() []ActivityContributor { return activityContribs.all() }
func CashierMenuRoots() []CashierMenuRoot         { return cashierMenuRoots.all() }
func DrawerSections() []DrawerSection             { return drawerSections.all() }

// CashierMenuRootsJSON renders the menu roots as a JSON array for the cashier
// Alpine scope: [{"emoji":"…","label":"…","url":"…"}].
// Field names are explicit json tags (lower-case) since the cashier JS reads
// r.emoji / r.label / r.url.
func CashierMenuRootsJSON() string {
	type r struct {
		Emoji string `json:"emoji"`
		Label string `json:"label"`
		URL   string `json:"url"`
	}
	roots := cashierMenuRoots.all()
	out := make([]r, 0, len(roots))
	for _, m := range roots {
		out = append(out, r{m.Emoji, m.Label, m.ChildrenURL})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// DrawerSectionsJSON renders the drawer sections for the cashier Alpine scope:
// [{"key":"…","openFormUrl":"…","closeFormUrl":"…","saveOpenUrl":"…","saveCloseUrl":"…"}].
func DrawerSectionsJSON() string {
	type s struct {
		Key          string `json:"key"`
		OpenFormURL  string `json:"openFormUrl"`
		CloseFormURL string `json:"closeFormUrl"`
		SaveOpenURL  string `json:"saveOpenUrl"`
		SaveCloseURL string `json:"saveCloseUrl"`
	}
	secs := drawerSections.all()
	out := make([]s, 0, len(secs))
	for _, d := range secs {
		out = append(out, s{d.Key, d.OpenFormURL, d.CloseFormURL, d.SaveOpenURL, d.SaveCloseURL})
	}
	b, _ := json.Marshal(out)
	return string(b)
}
func TenderMethods() []TenderMethod { return tenderMethods.all() }
func LogoutGuards() []LogoutGuard   { return logoutGuards.all() }
func ReceiptTabs() []ReceiptTab     { return receiptTabs.all() }

// CashierSupplierActions returns the plugin-contributed buttons for the cashier
// Suppliers page.
func CashierSupplierActions() []CashierSupplierAction { return supplierActions.all() }
