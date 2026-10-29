package api

// ChequeLine is one cheque in a payment form. Received: Bank is the payer's
// bank. Given: LockerID is the shop's bank place it is drawn on.
type ChequeLine struct {
	Bank     string `json:"bank"`
	LockerID int64  `json:"lockerId"`
	Number   string `json:"number"`
	Amount   string `json:"amount"`
	Date     string `json:"date"` // YYYY-MM-DD
	Note     string `json:"note"`
}
