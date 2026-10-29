package api

// ---- Warranty ----

type WarrantyClaimRow struct {
	ID                int64  `json:"id"`
	At                string `json:"at"`
	Reason            string `json:"reason"`
	Resolution        string `json:"resolution"` // replaced | …
	ProductName       string `json:"productName"`
	OldSerial         string `json:"oldSerial"`
	ReplacementSerial string `json:"replacementSerial"` // "" = no new unit (receipt claim)
	CustomerName      string `json:"customerName"`
	HandledBy         string `json:"handledBy"`
}

type WarrantyUnitRow struct {
	ID            int64  `json:"id"`
	Serial        string `json:"serial"`
	ProductName   string `json:"productName"`
	CustomerName  string `json:"customerName"`
	ReceiptNo     string `json:"receiptNo"`
	SoldAt        string `json:"soldAt"`
	Months        int    `json:"months"`
	Until         string `json:"until"`  // YYYY-MM-DD
	Left          string `json:"left"`   // "11 mo left" | "<1 mo left" | "expired"
	Status        string `json:"status"` // active | replaced
	UnderWarranty bool   `json:"underWarranty"`
}

type WarrantyUnitView struct {
	WarrantyUnitRow `tstype:",extends"`
	Claims          []WarrantyClaimRow `json:"claims"`
}

type WarrantyReceiptLine struct {
	ProductID     int64  `json:"productId"`
	Name          string `json:"name"`
	UnitAbbr      string `json:"unitAbbr"`
	Qty           string `json:"qty"`
	Replaced      int    `json:"replaced"`
	Remaining     string `json:"remaining"` // units still claimable
	Months        int    `json:"months"`
	Until         string `json:"until"`
	UnderWarranty bool   `json:"underWarranty"`
}

type WarrantyReceiptView struct {
	SaleID       int64                 `json:"saleId"`
	ReceiptNo    string                `json:"receiptNo"`
	SoldAt       string                `json:"soldAt"`
	CustomerName string                `json:"customerName"`
	Lines        []WarrantyReceiptLine `json:"lines"`
}

type WarrantyReplaceRequest struct {
	UnitID    int64  `json:"unitId"`
	NewSerial string `json:"newSerial"`
	Reason    string `json:"reason"`
}

type WarrantyReceiptReplaceRequest struct {
	SaleID    int64  `json:"saleId"`
	ProductID int64  `json:"productId"`
	Reason    string `json:"reason"`
}

type WarrantyClaimRef struct {
	ClaimID int64 `json:"claimId"`
}
