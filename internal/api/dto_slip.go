package api

// ---- Slip data (?data=1 on every slip print path) ----
//
// A slip's print data for its React template (Image mode). Each value is the one
// the Classic builder for that path prints: dates and labels come as the printed
// text, money as decimal strings (the template adds the symbol), names as stored
// (the template draws Sinhala / Tamil; Classic can only print ASCII).

type SlipData struct {
	Kind     string        `json:"kind"`  // sale | refund | money | debt | warranty | test
	Width    string        `json:"width"` // "58" | "80"
	Dots     int           `json:"dots"`
	Columns  int           `json:"columns"`  // Font A characters per line
	Currency string        `json:"currency"` // "Rs." when the shop has none
	Shop     SlipShop      `json:"shop"`
	Sale     *SlipSale     `json:"sale,omitempty"`
	Refund   *SlipRefund   `json:"refund,omitempty"`
	Money    *SlipMoney    `json:"money,omitempty"`
	Debt     *SlipDebt     `json:"debt,omitempty"`
	Warranty *SlipWarranty `json:"warranty,omitempty"`
	Test     *SlipTest     `json:"test,omitempty"`
}

// SlipShop is the shared header and footer every slip carries.
type SlipShop struct {
	Name      string `json:"name"`
	NameLocal string `json:"nameLocal"` // Sinhala / Tamil name, "" = none
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	VatNo     string `json:"vatNo"` // "" unless the shop is tax registered
	Footer    string `json:"footer"`
	Logo      string `json:"logo"` // PNG data URI of the printed logo raster (full dots wide), "" = none
}

type SlipSale struct {
	ReceiptNo       string        `json:"receiptNo"`
	Date            string        `json:"date"`
	Cashier         string        `json:"cashier"`
	Customer        string        `json:"customer"` // "" = walk-in
	Items           []SlipItem    `json:"items"`    // in print order; a line group is one item
	Subtotal        string        `json:"subtotal"` // before payouts
	ItemSavings     string        `json:"itemSavings"`
	BillSaving      string        `json:"billSaving"`
	BillSavingLabel string        `json:"billSavingLabel"` // " (10%)" for a percent discount, else ""
	Tax             string        `json:"tax"`
	Given           string        `json:"given"`    // payouts handed to the customer
	Total           string        `json:"total"`    // negative when payouts take the sale below zero
	YouSaved        string        `json:"youSaved"` // all discounts
	Payments        []SlipPayment `json:"payments"` // a zero row prints nothing (a payout sale's empty cash)
	Credit          bool          `json:"credit"`   // the credit block replaces Change
	FromAdvance     string        `json:"fromAdvance"`
	Due             string        `json:"due"`     // this sale's due after the advance
	Balance         string        `json:"balance"` // the customer's account after: < 0 = advance left
	Change          string        `json:"change"`
	OnAccount       string        `json:"onAccount"`  // rounding booked to the customer
	ChangeKept      string        `json:"changeKept"` // rounding kept in the drawer
}

type SlipPayment struct {
	Method string `json:"method"` // as printed ("Cash")
	Amount string `json:"amount"`
}

// SlipItem is a plain line, a payout (Payout), or a line group (Group).
type SlipItem struct {
	Name          string            `json:"name"` // the line's own label wins over the product name
	Payout        bool              `json:"payout"`
	Quantity      string            `json:"quantity"`
	UnitAbbr      string            `json:"unitAbbr"`
	UnitPrice     string            `json:"unitPrice"`
	NetUnitPrice  string            `json:"netUnitPrice"`
	PackLabel     string            `json:"packLabel"` // "" = no pack
	Gross         string            `json:"gross"`
	Discount      string            `json:"discount"`
	DiscountLabel string            `json:"discountLabel"` // " (10%)" or ""
	Subtotal      string            `json:"subtotal"`
	Warranty      string            `json:"warranty"` // "12-month warranty until 2027-06-13", "" = none
	Serials       []string          `json:"serials"`
	Group         bool              `json:"group"`
	Sets          string            `json:"sets"`     // group: how many sets
	SetGross      string            `json:"setGross"` // group: one set's full price
	SetNet        string            `json:"setNet"`   // group: one set's price after discounts
	Itemised      bool              `json:"itemised"` // group: printed item by item, each priced (subtotal = its total)
	Members       []SlipGroupMember `json:"members"`
}

type SlipGroupMember struct {
	Name     string   `json:"name"`
	Quantity string   `json:"quantity"`
	Subtotal string   `json:"subtotal"` // its price after its discount (an itemised group prints it)
	Discount string   `json:"discount"`
	Warranty string   `json:"warranty"`
	Serials  []string `json:"serials"`
}

type SlipRefund struct {
	ReceiptNo        string           `json:"receiptNo"` // the original sale's
	Date             string           `json:"date"`
	Customer         string           `json:"customer"`
	Reason           string           `json:"reason"`
	Items            []SlipRefundItem `json:"items"`
	Refund           string           `json:"refund"`
	CreditReduction  string           `json:"creditReduction"`
	RemainingBalance string           `json:"remainingBalance"` // "" = no customer
}

type SlipRefundItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	UnitAbbr string `json:"unitAbbr"`
	Refund   string `json:"refund"`
}

type SlipMoney struct {
	Title     string `json:"title"` // the kind's label ("Withdrawal")
	ReceiptNo string `json:"receiptNo"`
	Date      string `json:"date"`
	From      string `json:"from"`
	To        string `json:"to"`
	Party     string `json:"party"`
	By        string `json:"by"`
	Amount    string `json:"amount"`
	Note      string `json:"note"`
}

type SlipDebt struct {
	ReceiptNo     string `json:"receiptNo"`
	Date          string `json:"date"`
	Customer      string `json:"customer"`
	Phone         string `json:"phone"`
	Amount        string `json:"amount"`
	Method        string `json:"method"`        // as printed ("Cash")
	BalanceBefore string `json:"balanceBefore"` // "" (with BalanceAfter) = not recorded
	BalanceAfter  string `json:"balanceAfter"`
	Cashier       string `json:"cashier"`
	// Cheques a cheque payment was made with (Method = the cheque text).
	Cheques []SlipPayment `json:"cheques"`
}

type SlipWarranty struct {
	Date          string `json:"date"`
	Customer      string `json:"customer"`
	Product       string `json:"product"`
	OldSerial     string `json:"oldSerial"`
	NewSerial     string `json:"newSerial"`
	WarrantyUntil string `json:"warrantyUntil"`
	WarrantyLeft  string `json:"warrantyLeft"`
}

type SlipTest struct {
	Printed string `json:"printed"`
}
