// Package karots carries what a shop agrees to, so the installer can show it
// and record exactly which version was accepted (cmd/setup's "I agree" tick).
package karots

import _ "embed"

// License is the Licence and Service Agreement's terms (the LICENSE file).
//
//go:embed LICENSE
var License []byte

// ShopRulesPDF is the Shop Rules & Service Guide, Annexure "A" (make agreement).
//
//go:embed docs/legal/karots-suite-shop-rules.pdf
var ShopRulesPDF []byte
