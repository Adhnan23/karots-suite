package escpos

import (
	"bytes"
	"time"

	"karots-suite/internal/features/sales"
	"karots-suite/internal/money"

	"github.com/shopspring/decimal"
)

// SlipCheque is one cheque printed on a slip.
type SlipCheque struct {
	Number, Bank string
	Date         time.Time
	Amount       decimal.Decimal
}

// ChequeText is how a cheque reads on paper — black and white, so the word
// CHEQUE marks it, not a colour: "CHEQUE 123456 Bank of Ceylon 12/11/26".
func ChequeText(number, bank string, date time.Time) string {
	s := "CHEQUE " + number
	if bank != "" {
		s += " " + bank
	}
	return s + " " + date.Format("02/01/06")
}

// PaymentLabel is a sale payment line's printed label: its cheque for a
// cheque line, else the method's name.
func PaymentLabel(p sales.Payment) string {
	if p.Method == sales.MethodCheque && p.ChequeNumber != nil && p.ChequeDate != nil {
		bank := ""
		if p.ChequeBank != nil {
			bank = *p.ChequeBank
		}
		return ChequeText(*p.ChequeNumber, bank, *p.ChequeDate)
	}
	return capitalize(p.Method)
}

// moneyLine prints label … amount, wrapping a label too long to share the
// line onto its own line (a cheque's bank and number are worth keeping whole).
func moneyLine(b *bytes.Buffer, label, amount string, w int) {
	label = ascii(label)
	if len(label)+len(amount)+1 > w {
		for _, l := range wrap(label, w) {
			line(b, l)
		}
		line(b, leftRight("", amount, w))
		return
	}
	line(b, leftRight(label, amount, w))
}

func chequeLines(b *bytes.Buffer, cs []SlipCheque, sym string, w int) {
	for _, c := range cs {
		moneyLine(b, ChequeText(c.Number, c.Bank, c.Date), money.Format(sym, c.Amount), w)
	}
}
