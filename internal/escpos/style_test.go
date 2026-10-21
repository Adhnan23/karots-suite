package escpos

import (
	"bytes"
	"testing"
)

// Classic printing has one look: receipt_style names an Image style now and
// never changes the ESC/POS output.
func TestClassicIgnoresReceiptStyle(t *testing.T) {
	c := cfg("80")
	c.ReceiptStyle = "classic"
	want := Document(sampleDetail(), c, Options{})
	for _, k := range []string{"bold", "modern", "boxed", "compact"} {
		c.ReceiptStyle = k
		if !bytes.Equal(Document(sampleDetail(), c, Options{}), want) {
			t.Errorf("%q: Classic output must ignore receipt_style", k)
		}
	}
}
