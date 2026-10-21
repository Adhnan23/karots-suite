package api

import (
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
)

// slipImage is Image mode's print job: when the request carries the slip as a
// picture (Content-Type image/png, drawn by the SPA at the printer's dots), it
// returns that picture framed as a slip, to send in place of the Classic bytes.
// cfg is the slipSettings copy, so ?size=58|80 sets the width it must match.
// The caller keeps its own kick rule and printer lookup.
func slipImage(c echo.Context, cfg *settings.Settings) ([]byte, bool, error) {
	if !strings.HasPrefix(c.Request().Header.Get("Content-Type"), "image/png") {
		return nil, false, nil
	}
	dots := receiptimg.PrinterDots(cfg.ReceiptWidth)
	img, err := escpos.ReadImage(c.Request().Body, dots)
	if err == nil {
		var job []byte
		if job, err = escpos.ImageJob(img, dots); err == nil {
			return job, true, nil
		}
	}
	if errors.Is(err, escpos.ErrBadImage) {
		return nil, true, &apperr.AppError{Code: "BAD_IMAGE", Message: "That receipt picture can't be printed", Status: 422}
	}
	return nil, true, err
}
