package products

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"

	"karots-suite/internal/apperr"
)

// GenerateBarcode mints a valid EAN-13 that no product currently uses. It uses
// the GS1 "restricted distribution" prefix 2 (reserved for in-store codes), so a
// generated value can never collide with a real manufacturer barcode. The DB
// uniqueness check guards against reissuing an existing/deactivated product's
// code; the products_barcode_key constraint remains the source of truth on save.
func (s *Service) GenerateBarcode(ctx context.Context) (string, error) {
	for range 20 {
		code, err := randomEAN13("2")
		if err != nil {
			return "", apperr.Internal("failed to generate barcode", err)
		}
		exists, err := s.repo.BarcodeExists(ctx, code)
		if err != nil {
			return "", apperr.Internal("failed to check barcode", err)
		}
		if !exists {
			return code, nil
		}
	}
	return "", apperr.Internal("could not generate a unique barcode; please try again", nil)
}

// randomEAN13 builds a 13-digit EAN-13 from the given leading prefix, filling the
// remaining data digits randomly and appending the EAN-13 check digit.
func randomEAN13(prefix string) (string, error) {
	digits := []byte(prefix)
	for len(digits) < 12 {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		digits = append(digits, byte('0'+n.Int64()))
	}
	return string(digits) + string(ean13CheckDigit(digits)), nil
}

// ean13CheckDigit computes the EAN-13 modulo-10 check digit for 12 data digits:
// odd positions weight 1, even positions weight 3 (1-indexed from the left).
func ean13CheckDigit(d12 []byte) byte {
	sum := 0
	for i, c := range d12 {
		n := int(c - '0')
		if i%2 == 1 {
			n *= 3
		}
		sum += n
	}
	return byte('0' + (10-(sum%10))%10)
}

// AssignBarcode sets a barcode on a product that currently has none (the small
// "add barcode" action on barcode-less rows). The code must be non-empty and not
// already used by another product; it never overwrites an existing barcode.
func (s *Service) AssignBarcode(ctx context.Context, id int64, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperr.Validation("enter or generate a barcode")
	}
	exists, err := s.repo.BarcodeExists(ctx, code)
	if err != nil {
		return apperr.Internal("failed to check barcode", err)
	}
	if exists {
		return apperr.Validation("that barcode is already used by another product")
	}
	ok, err := s.repo.SetBarcodeIfEmpty(ctx, id, code)
	if err != nil {
		return apperr.Internal("failed to save barcode", err)
	}
	if !ok {
		return apperr.Validation("this item already has a barcode")
	}
	return nil
}
