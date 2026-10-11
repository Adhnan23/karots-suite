package customers

import (
	"context"
	"testing"
)

// TestPaymentNeedsADestination: a card/online repayment with no account, or an
// unknown method, is refused before anything is written (so a nil tx is fine).
func TestPaymentNeedsADestination(t *testing.T) {
	svc := &Service{}
	for _, in := range []PaymentInput{
		{Amount: "100", Method: "card"},
		{Amount: "100", Method: "online"},
		{Amount: "100", Method: "mystery", AccountID: 1},
	} {
		if _, err := svc.RecordPaymentTx(context.Background(), nil, 1, in, 0); err == nil {
			t.Errorf("%s (account %d): want refusal, got nil", in.Method, in.AccountID)
		}
	}
}
