package plugin

import (
	"karots-suite/internal/config"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/features/stock"

	"github.com/jmoiron/sqlx"
)

// Core is the frozen API surface a plugin may use: the database, the loaded
// config, and a curated set of core services. Plugins depend only on this (and
// the Registry) — never on internal/api — so the API layer can evolve freely.
//
// The service instances here are the SAME wired ones the core API uses (built
// once in internal/app), so a plugin sale, expense or audit entry is identical
// to a core one.
type Core struct {
	DB  *sqlx.DB
	Cfg *config.Config

	Audit        *audit.Service
	Settings     *settings.Service
	CashRegister *cashregister.Service
	Sales        *sales.Service
	Expenses     *expenses.Service
	Products     *products.Service
	Stock        *stock.Service
	// Cashflow + Lockers let a plugin move money between real cash locations
	// (lockers/tills) and list them for a source picker — e.g. paying an outside
	// repairer from a chosen locker, exactly like the core expense/bill-pay flow.
	Cashflow *cashflow.Service
	Lockers  *lockers.Service
	// ProductSheetNameTaken says a column name the products sheet already reads
	// as one of its own (so a plugin's sheet column must not use it).
	ProductSheetNameTaken func(name string) bool
}
